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
	mu           sync.Mutex
	models       []scheduledModel // increasing accuracy
	budget       float64          // logical CPU-seconds per wall-clock second
	batchCost    float64
	batchThreads int
	leases       map[*engineLease]struct{}
	changed      chan struct{}
	ready        chan struct{}
	stopped      bool
	cancel       context.CancelFunc
	done         chan struct{}
	closeOnce    sync.Once
}
type engineLease struct {
	engine     *scheduler
	opts       LeaseOptions
	selected   int
	released   bool
	batchModel Model
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
	e := &scheduler{budget: budget, batchCost: min(float64(threads), budget), batchThreads: threads, leases: make(map[*engineLease]struct{}), changed: make(chan struct{}), ready: make(chan struct{}), cancel: cancel, done: make(chan struct{})}
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
		s, err := m.acquire()
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
	e.notify()
	e.mu.Unlock()
}
func (e *scheduler) notify() { close(e.changed); e.changed = make(chan struct{}) }
func (e *scheduler) Models() []ModelInfo {
	e.mu.Lock()
	defer e.mu.Unlock()
	infos := make([]ModelInfo, len(e.models))
	for i, m := range e.models {
		infos[i] = m.model.Info()
	}
	return infos
}
func (l *engineLease) Model() Model {
	e := l.engine
	e.mu.Lock()
	defer e.mu.Unlock()
	if l.released || e.stopped {
		return nil
	}
	if l.batchModel != nil {
		return l.batchModel
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
	e.notify()
}
func (e *scheduler) batchUsed() float64 {
	n := 0
	for l := range e.leases {
		if !l.opts.Realtime {
			n++
		}
	}
	return float64(n) * e.batchCost
}
func (e *scheduler) candidate(l *engineLease, share float64, upgrade bool) int {
	limit := share
	if upgrade {
		limit *= .8
	}
	best := -1
	for i, m := range e.models {
		info := m.model.Info()
		if l.opts.Model != "" && info.Name != l.opts.Model {
			continue
		}
		if m.cost <= limit && info.RealtimeFactor <= 1 {
			best = i
		}
	}
	return best
}
func (e *scheduler) rebalance() {
	n := 0
	for l := range e.leases {
		if l.opts.Realtime {
			n++
		}
	}
	if n == 0 {
		return
	}
	share := (e.budget - e.batchUsed()) / float64(n)
	for l := range e.leases {
		if !l.opts.Realtime {
			continue
		}
		if e.models[l.selected].cost > share {
			if i := e.candidate(l, share, false); i >= 0 {
				l.selected = i
			}
		} else if i := e.candidate(l, share, true); i > l.selected {
			l.selected = i
		}
	}
}
func (e *scheduler) Lease(ctx context.Context, opts LeaseOptions) (Lease, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-e.ready:
	}
	for {
		e.mu.Lock()
		if e.stopped {
			e.mu.Unlock()
			return nil, fmt.Errorf("speech engine closed")
		}
		if err := ctx.Err(); err != nil {
			e.mu.Unlock()
			return nil, err
		}
		if len(e.models) == 0 {
			e.mu.Unlock()
			return nil, ErrOverBudget
		}
		if opts.Model != "" {
			found := false
			for _, m := range e.models {
				if m.model.Info().Name == opts.Model {
					found = true
				}
			}
			if !found {
				e.mu.Unlock()
				return nil, fmt.Errorf("unknown speech model %q", opts.Model)
			}
		}
		l := &engineLease{engine: e, opts: opts}
		if opts.Realtime {
			n := 1
			for other := range e.leases {
				if other.opts.Realtime {
					n++
				}
			}
			share := (e.budget - e.batchUsed()) / float64(n)
			l.selected = e.candidate(l, share, false)
			fits := l.selected >= 0
			for other := range e.leases {
				if other.opts.Realtime && e.candidate(other, share, false) < 0 {
					fits = false
				}
			}
			if !fits {
				e.mu.Unlock()
				return nil, ErrOverBudget
			}
			e.leases[l] = struct{}{}
			e.rebalance()
			e.mu.Unlock()
			return l, nil
		}
		used := e.batchUsed()
		for other := range e.leases {
			if other.opts.Realtime {
				used += e.models[other.selected].cost
			}
		}
		if used+e.batchCost <= e.budget {
			l.selected = len(e.models) - 1
			if opts.Model != "" {
				for i, m := range e.models {
					if m.model.Info().Name == opts.Model {
						l.selected = i
					}
				}
			}
			if e.batchCost < float64(e.batchThreads) {
				l.batchModel = &pacedModel{Model: e.models[l.selected].model, slowdown: float64(e.batchThreads)/e.batchCost - 1}
			}
			e.leases[l] = struct{}{}
			e.mu.Unlock()
			return l, nil
		}
		changed := e.changed
		e.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-changed:
		}
	}
}
func (e *scheduler) Close() error {
	e.closeOnce.Do(func() {
		e.mu.Lock()
		e.stopped = true
		e.notify()
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

// Fractional-CPU nodes still run VOD jobs: pace their single native thread
// rather than waiting forever for a whole logical CPU that can never be spare.
type pacedModel struct {
	Model
	slowdown float64
}

func (m *pacedModel) Transcribe(ctx context.Context, pcm []float32, opts Options) (*Result, error) {
	start := time.Now()
	result, err := m.Model.Transcribe(ctx, pcm, opts)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	delay := time.Duration(float64(time.Since(start)) * m.slowdown)
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return result, err
	}
}
