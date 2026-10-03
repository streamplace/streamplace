package stt

/*
#cgo pkg-config: whisper
#cgo linux LDFLAGS: -lstdc++ -lm
#cgo windows LDFLAGS: -lstdc++ -lm
#cgo darwin LDFLAGS: -lc++ -lm
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"math"
	"os"
	"runtime"
	"runtime/cgo"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"stream.place/streamplace/pkg/log"
)

//go:embed assets/*.bin assets/licenses.txt
var bundled embed.FS

func init() {
	ModelFiles, _ = fs.Sub(bundled, "assets")
	C.sp_log_init()
}

func supportedCPU() bool {
	return C.sp_whisper_supported_cpu() != 0
}

//export goWhisperAbort
func goWhisperAbort(handle C.uintptr_t) C.int {
	if cgo.Handle(handle).Value().(context.Context).Err() != nil {
		return 1
	}
	return 0
}

// goWhisperLog receives whisper.cpp output printed outside whisperOutput.
//
//export goWhisperLog
func goWhisperLog(text *C.char) {
	logWhisper(context.Background(), C.GoString(text))
}

// whisperOutput collects what whisper.cpp prints on this goroutine's thread
// until the returned func runs, which logs it as one debug line.
func whisperOutput(ctx context.Context) func() {
	runtime.LockOSThread()
	C.sp_log_begin()
	return func() {
		text := C.sp_log_end()
		runtime.UnlockOSThread()
		if text != nil {
			logWhisper(ctx, C.GoString(text))
			C.free(unsafe.Pointer(text))
		}
	}
}

func logWhisper(ctx context.Context, output string) {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) > 0 {
		log.Debug(ctx, "whisper", "output", strings.Join(lines, "; "))
	}
}

type nativeState struct {
	state *C.struct_whisper_state
	vad   *C.struct_whisper_vad_context
}
type whisperModel struct {
	mu      sync.Mutex
	info    ModelInfo
	path    string
	threads int
	ctx     *C.struct_whisper_context
	idle    []*nativeState
	all     []*nativeState
	active  sync.WaitGroup
	closed  bool
}

func (m *whisperModel) Info() ModelInfo  { m.mu.Lock(); defer m.mu.Unlock(); return m.info }
func (m *whisperModel) factor(f float64) { m.mu.Lock(); m.info.RealtimeFactor = f; m.mu.Unlock() }

// Network dimensions rank model capacity independently of quantization and
// noisy benchmark timings. Encoder/decoder transformer matrix counts differ.
func (m *whisperModel) scale() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	audio := int64(C.whisper_model_n_audio_state(m.ctx))
	text := int64(C.whisper_model_n_text_state(m.ctx))
	return 12*audio*audio*int64(C.whisper_model_n_audio_layer(m.ctx)) +
		16*text*text*int64(C.whisper_model_n_text_layer(m.ctx))
}
func (m *whisperModel) acquire() (*nativeState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, fmt.Errorf("speech model closed")
	}
	if m.ctx == nil {
		var data []byte
		var err error
		if m.info.Bundled {
			data, err = fs.ReadFile(ModelFiles, m.path)
		} else {
			data, err = os.ReadFile(m.path)
		}
		if err != nil {
			return nil, fmt.Errorf("load speech model: %w", err)
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("empty speech model")
		}
		// whisper copies/decodes weights once; no model blob traverses cgo during inference.
		m.ctx = C.sp_whisper_load(unsafe.Pointer(&data[0]), C.size_t(len(data)))
		runtime.KeepAlive(data)
		if m.ctx == nil {
			return nil, fmt.Errorf("invalid whisper model %s", m.path)
		}
	}
	if n := len(m.idle); n > 0 {
		s := m.idle[n-1]
		m.idle = m.idle[:n-1]
		m.active.Add(1)
		return s, nil
	}
	s := &nativeState{state: C.whisper_init_state(m.ctx)}
	if s.state == nil {
		return nil, fmt.Errorf("allocate whisper state")
	}
	data, err := fs.ReadFile(ModelFiles, "ggml-silero-v5.1.2.bin")
	if err != nil {
		C.whisper_free_state(s.state)
		return nil, err
	}
	s.vad = C.sp_vad_load(unsafe.Pointer(&data[0]), C.size_t(len(data)), C.int(m.threads))
	runtime.KeepAlive(data)
	if s.vad == nil {
		C.whisper_free_state(s.state)
		return nil, fmt.Errorf("load Silero VAD")
	}
	m.all = append(m.all, s)
	m.active.Add(1)
	return s, nil
}
func (m *whisperModel) put(s *nativeState) {
	m.mu.Lock()
	m.idle = append(m.idle, s)
	m.mu.Unlock()
	m.active.Done()
}
func (m *whisperModel) close() {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	m.active.Wait()
	for _, s := range m.all {
		C.whisper_vad_free(s.vad)
		C.whisper_free_state(s.state)
	}
	if m.ctx != nil {
		C.whisper_free(m.ctx)
	}
}
func (m *whisperModel) Transcribe(ctx context.Context, pcm []float32, opts Options) (*Result, error) {
	return m.transcribe(ctx, pcm, opts, true)
}
func (m *whisperModel) transcribe(ctx context.Context, pcm []float32, opts Options, vad bool) (*Result, error) {
	defer whisperOutput(ctx)()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(pcm) == 0 {
		return &Result{Language: opts.Language, NoSpeechProb: 1}, nil
	}
	for _, v := range pcm {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || v < -1 || v > 1 {
			return nil, fmt.Errorf("PCM must be finite and within [-1,1]")
		}
	}
	language, _, _ := strings.Cut(opts.Language, "-")
	language = strings.ToLower(language)
	lang := C.CString(language)
	defer C.free(unsafe.Pointer(lang))
	if language != "" && C.whisper_lang_id(lang) < 0 {
		return nil, fmt.Errorf("unsupported speech language %q", opts.Language)
	}
	s, err := m.acquire()
	if err != nil {
		return nil, err
	}
	defer m.put(s)
	samples := (*C.float)(unsafe.Pointer(&pcm[0]))
	if vad {
		if !bool(C.whisper_vad_detect_speech(s.vad, samples, C.int(len(pcm)))) {
			return nil, fmt.Errorf("Silero VAD inference failed")
		}
		probs := unsafe.Slice(C.whisper_vad_probs(s.vad), int(C.whisper_vad_n_probs(s.vad)))
		speech := false
		for _, p := range probs {
			if p >= 0.5 {
				speech = true
				break
			}
		}
		if !speech {
			return &Result{Language: opts.Language, NoSpeechProb: 1}, nil
		}
	}
	prompt := C.CString(opts.Prompt)
	defer C.free(unsafe.Pointer(prompt))
	h := cgo.NewHandle(ctx)
	defer h.Delete()
	status := C.sp_whisper_run(m.ctx, s.state, samples, C.int(len(pcm)), C.int(m.threads), lang, prompt, C.uintptr_t(h))
	runtime.KeepAlive(pcm)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if status != 0 {
		return nil, fmt.Errorf("whisper inference failed: %d", status)
	}
	result := &Result{Language: C.GoString(C.whisper_lang_str(C.whisper_full_lang_id_from_state(s.state)))}
	duration := time.Duration(len(pcm)) * time.Second / SampleRate
	segments := int(C.whisper_full_n_segments_from_state(s.state))
	for i := 0; i < segments; i++ {
		seg := C.int(i)
		result.NoSpeechProb += float32(C.whisper_full_get_segment_no_speech_prob_from_state(s.state, seg))
		for j := 0; j < int(C.whisper_full_n_tokens_from_state(s.state, seg)); j++ {
			tok := C.int(j)
			d := C.whisper_full_get_token_data_from_state(s.state, seg, tok)
			if d.id >= C.whisper_token_eot(m.ctx) {
				continue
			}
			text := C.GoString(C.whisper_full_get_token_text_from_state(m.ctx, s.state, seg, tok))
			if text == "" {
				continue
			}
			start := max(time.Duration(d.t0)*10*time.Millisecond, 0)
			end := min(max(time.Duration(d.t1)*10*time.Millisecond, start), duration)
			start = min(start, duration)
			// Whisper uses subword tokens. Join continuations and punctuation to the word,
			// preserving UTF-8 fragments until the complete word has been assembled.
			first, _ := utf8.DecodeRuneInString(text)
			if len(result.Words) > 0 && !unicode.IsSpace(first) {
				w := &result.Words[len(result.Words)-1]
				w.Text += text
				w.End = max(w.End, end)
				w.Prob = min(w.Prob, float32(d.p))
			} else if strings.TrimSpace(text) != "" {
				result.Words = append(result.Words, Word{Text: strings.TrimSpace(text), Start: start, End: end, Prob: float32(d.p)})
			}
		}
	}
	if segments > 0 {
		result.NoSpeechProb /= float32(segments)
	}
	return result, nil
}
