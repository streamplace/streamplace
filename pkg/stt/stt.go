// Package stt runs speech recognition with the whisper models bundled into
// the binary, sharing the node's CPU budget across every stream that needs
// captions.
package stt

import (
	"context"
	"errors"
	"io/fs"
	"time"
)

// ModelFiles holds the bundled model files for serving to browser captioners:
// ggml-tiny-q5_1.bin, ggml-base-q5_1.bin, ggml-small-q5_1.bin, and the Silero
// VAD model, with their license texts. The whisper build sets it at init; it
// is nil in builds without bundled models.
var ModelFiles fs.FS

// SampleRate is the PCM rate Transcribe expects: 16 kHz mono float32 in
// [-1, 1].
const SampleRate = 16000

// ErrOverBudget means no model fits the remaining CPU budget for another
// realtime stream.
var ErrOverBudget = errors.New("speech recognition is over its CPU budget")

// Word is one recognized word. Times are offsets from the first sample passed
// to Transcribe.
type Word struct {
	Text  string
	Start time.Duration
	End   time.Duration
	Prob  float32
}

type Result struct {
	Language     string // detected or forced language, BCP 47
	Words        []Word
	NoSpeechProb float32
}

type Options struct {
	Language string // BCP 47; empty detects the language
	Prompt   string // preceding transcript text, for continuity across windows
}

type ModelInfo struct {
	Name         string // e.g. "whisper-base-q5_1"
	Size         string // "tiny", "base", "small", or "external"
	Multilingual bool
	Bundled      bool
	// RealtimeFactor is processing time per second of audio at the engine's
	// per-stream thread count, as measured by the startup benchmark. Zero
	// before measurement.
	RealtimeFactor float64
}

// Model is a loaded speech model.
type Model interface {
	Info() ModelInfo
	Transcribe(ctx context.Context, pcm []float32, opts Options) (*Result, error)
}

type LeaseOptions struct {
	// Realtime leases must keep up with live audio; the engine reserves
	// capacity for them and may move them to a smaller or larger model as
	// load changes. Non-realtime leases (VOD) wait for spare capacity and get
	// the most accurate model available.
	Realtime bool
	// Model forces a specific model by name.
	Model string
}

// Lease is one stream's claim on speech recognition capacity. Call Model
// before each Transcribe: realtime leases can change models between calls.
// A Lease's Transcribe calls must not overlap.
type Lease interface {
	Model() Model
	Release()
}

type Engine interface {
	Lease(ctx context.Context, opts LeaseOptions) (Lease, error)
	Models() []ModelInfo
	Close() error
}
