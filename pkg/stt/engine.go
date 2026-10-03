package stt

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
)

type scheduledModel struct {
	model Model
	cost  float64
	scale int64
}
type scheduler struct {
	mu        sync.Mutex
	models    []scheduledModel // increasing accuracy
	budget    float64          // logical CPU-seconds per wall-clock second
	leases    map[*engineLease]struct{}
	ready     chan struct{}
	stopped   bool
	cancel    context.CancelFunc
	done      chan struct{}
	closeOnce sync.Once
}
type engineLease struct {
	engine   *scheduler
	selected int
	released bool
}

// NewEngine returns without loading/benchmarking models on the node startup path.
// Lease waits for measurement, respecting its context. Costs include 20% headroom.
func NewEngine(ctx context.Context, cli *config.CLI) (Engine, error) {
	if !supportedCPU() {
		return nil, fmt.Errorf("whisper CPU backend requires AVX2, FMA and F16C; automatic captions disabled on this CPU")
	}
	if math.IsNaN(cli.CaptionsCPUBudget) || cli.CaptionsCPUBudget <= 0 || cli.CaptionsCPUBudget > 1 {
		return nil, fmt.Errorf("captions CPU budget must be in (0,1]")
	}
	budget := float64(runtime.NumCPU()) * cli.CaptionsCPUBudget
	threads := max(1, min(4, int(budget)))
	ctx, cancel := context.WithCancel(ctx)
	e := &scheduler{budget: budget, leases: make(map[*engineLease]struct{}), ready: make(chan struct{}), cancel: cancel, done: make(chan struct{})}
	for _, size := range []string{"tiny", "base", "small"} {
		e.models = append(e.models, scheduledModel{model: &whisperModel{info: ModelInfo{Name: "whisper-" + size + "-q5_1", Size: size, Multilingual: true, Bundled: true}, path: "ggml-" + size + "-q5_1.bin", threads: threads}})
	}
	if cli.CaptionsModelDir != "" {
		entries, err := os.ReadDir(cli.CaptionsModelDir)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("read captions model directory: %w", err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasPrefix(name, "ggml-") || !strings.HasSuffix(name, ".bin") || strings.Contains(name, "silero") {
				continue
			}
			e.models = append(e.models, scheduledModel{model: &whisperModel{info: ModelInfo{Name: strings.TrimSuffix(name, ".bin"), Size: "external", Multilingual: !strings.Contains(name, ".en"), Bundled: false}, path: filepath.Join(cli.CaptionsModelDir, name), threads: threads}})
		}
	}
	go e.benchmark(ctx, threads)
	return e, nil
}
func (e *scheduler) benchmark(ctx context.Context, threads int) {
	defer close(e.done)
	// Synthetic voiced signal intentionally bypasses VAD: silence must not make
	// the benchmark falsely report an almost-free recognition cost.
	pcm := make([]float32, 3*SampleRate)
	for i := range pcm {
		t := float64(i) / SampleRate
		pcm[i] = float32(.15*math.Sin(2*math.Pi*170*t) + .05*math.Sin(2*math.Pi*340*t))
	}
	var measured []scheduledModel
	for _, item := range e.models {
		m := item.model.(*whisperModel)
		// Load separately so startup disk/weight allocation is not counted as RTF.
		logged := whisperOutput(ctx)
		s, err := m.acquire()
		logged()
		if err == nil {
			m.put(s)
		}
		if err == nil {
			start := time.Now()
			_, err = m.transcribe(ctx, pcm, Options{Language: "en"}, false)
			if err == nil {
				factor := time.Since(start).Seconds() / 3
				m.factor(factor)
				item.cost = factor * float64(threads) * 1.2
				item.scale = m.scale()
				measured = append(measured, item)
				log.Log(ctx, "caption model benchmark", "model", m.Info().Name, "threads", threads, "realtimeFactor", factor)
			}
		}
		if err != nil {
			log.Error(ctx, "caption model unavailable", "model", m.Info().Name, "error", err)
			m.close()
		}
		if ctx.Err() != nil {
			break
		}
	}
	// Extra models join the same accuracy order as bundled models. Neither
	// quantized file size nor benchmark noise should reverse model preference.
	sort.SliceStable(measured, func(i, j int) bool { return measured[i].scale < measured[j].scale })
	e.mu.Lock()
	e.models = measured
	close(e.ready)
	e.mu.Unlock()
}
func (l *engineLease) Model() Model {
	e := l.engine
	e.mu.Lock()
	defer e.mu.Unlock()
	if l.released || e.stopped {
		return nil
	}
	return e.models[l.selected].model
}
func (l *engineLease) Release() {
	e := l.engine
	e.mu.Lock()
	defer e.mu.Unlock()
	if l.released {
		return
	}
	l.released = true
	delete(e.leases, l)
	e.rebalance()
}
func (e *scheduler) candidate(share float64, upgrade bool) int {
	limit := share
	if upgrade {
		limit *= .8
	}
	best := -1
	for i, m := range e.models {
		info := m.model.Info()
		if m.cost <= limit && info.RealtimeFactor <= 1 {
			best = i
		}
	}
	return best
}
func (e *scheduler) rebalance() {
	n := len(e.leases)
	if n == 0 {
		return
	}
	share := e.budget / float64(n)
	for l := range e.leases {
		if e.models[l.selected].cost > share {
			if i := e.candidate(share, false); i >= 0 {
				l.selected = i
			}
		} else if i := e.candidate(share, true); i > l.selected {
			l.selected = i
		}
	}
}
func (e *scheduler) Lease(ctx context.Context) (Lease, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-e.ready:
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stopped {
		return nil, fmt.Errorf("speech engine closed")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	share := e.budget / float64(len(e.leases)+1)
	selected := e.candidate(share, false)
	if selected < 0 {
		return nil, ErrOverBudget
	}
	l := &engineLease{engine: e, selected: selected}
	e.leases[l] = struct{}{}
	e.rebalance()
	return l, nil
}
func (e *scheduler) Close() error {
	e.closeOnce.Do(func() {
		e.mu.Lock()
		e.stopped = true
		e.mu.Unlock()
		e.cancel()
		<-e.done
		for _, m := range e.models {
			if w, ok := m.model.(*whisperModel); ok {
				w.close()
			}
		}
	})
	return nil
}
