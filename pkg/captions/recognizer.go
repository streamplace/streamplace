package captions

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/stt"
)

// RecognizerOptions configures a live speech recognizer for one stream.
type RecognizerOptions struct {
	Streamer  string
	Origin    Origin
	Author    string   // DID the track is attributed to (the node)
	Languages []string // policy hints, most prominent first; empty detects
	Hub       *Hub
	Engine    stt.Engine
	// OnCoverage runs after finalized captions are published, with the absolute
	// audio watermark before any still-provisional words. It is monotonic.
	OnCoverage func(time.Time)

	// Step is how much new audio arrives between passes over the window.
	Step time.Duration
	// MinWindow is the least uncommitted audio worth a pass.
	MinWindow time.Duration
	// SilenceFlush is the trailing silence that commits everything
	// recognized so far.
	SilenceFlush time.Duration
}

func (o *RecognizerOptions) defaults() {
	if o.Step <= 0 {
		o.Step = time.Second
	}
	if o.MinWindow <= 0 {
		o.MinWindow = 2 * time.Second
	}
	if o.SilenceFlush <= 0 {
		o.SilenceFlush = 1200 * time.Millisecond
	}
}

type pcmChunk struct {
	start time.Time
	pcm   []float32
}

// Recognizer turns a stream's 16 kHz mono audio into caption cues on the
// hub. Audio is pushed with the wall-clock time of its first sample (the
// segment startTime clock); a sliding window is transcribed every Step of
// new audio, words that two consecutive passes agree on are committed
// (LocalAgreement-2) and grouped into final cues, the rest are published as
// the interim tail of the open cue.
type Recognizer struct {
	opts  RecognizerOptions
	ctx   context.Context
	lease stt.Lease
	in    chan pcmChunk
	sync  chan chan struct{}
	stop  chan struct{}
	done  chan struct{}
	once  sync.Once

	// Worker-owned state.
	buf       []float32 // uncommitted audio, starting at bufStart
	bufStart  time.Time
	sincePass int    // samples appended since the last pass
	prev      []Word // the previous pass's uncommitted words
	prompt    []string
	language  string
	track     Track
	grouper   *Grouper
	coverage  time.Time
}

// NewRecognizer leases recognition capacity and starts the worker. It
// returns stt.ErrOverBudget (wrapped) when the engine has no room.
func NewRecognizer(ctx context.Context, opts RecognizerOptions) (*Recognizer, error) {
	opts.defaults()
	if opts.Engine == nil {
		return nil, errors.New("no speech engine")
	}
	if opts.Hub == nil {
		return nil, errors.New("no caption hub")
	}
	lease, err := opts.Engine.Lease(ctx)
	if err != nil {
		return nil, fmt.Errorf("lease speech model: %w", err)
	}
	r := &Recognizer{
		opts:  opts,
		ctx:   ctx,
		lease: lease,
		in:    make(chan pcmChunk, 1024),
		sync:  make(chan chan struct{}),
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
	if len(opts.Languages) > 0 {
		r.language = opts.Languages[0]
	}
	// Re-admission on the same track/hub must not reuse already finalized IDs.
	r.grouper = NewGrouper(DefaultCueLayout(), uuid.NewString()+"-")
	go r.run()
	return r, nil
}

// Push hands over decoded audio: pcm starts at the wall-clock time start.
// Chunks must arrive in time order; a gap or overlap against the previous
// chunk ends the current window (its words are committed) and starts a
// fresh one at start. Push never blocks the caller: when the worker is too
// far behind the chunk is dropped.
func (r *Recognizer) Push(start time.Time, pcm []float32) {
	if len(pcm) == 0 {
		return
	}
	select {
	case r.in <- pcmChunk{start: start, pcm: pcm}:
	default:
		log.Warn(r.ctx, "speech recognition is behind, dropping audio", "streamer", r.opts.Streamer, "dropped", samplesDuration(len(pcm)))
	}
}

// promptWords is how many committed words are carried over as the next
// pass's prompt.
const promptWords = 48

// Close flushes the remaining audio and words into final cues and releases
// the lease.
func (r *Recognizer) Close() {
	r.once.Do(func() { close(r.stop) })
	<-r.done
}

const rate = stt.SampleRate

const (
	recognizerMaxWindow         = 10 * time.Second
	recognizerMaxBuffer         = 30 * time.Second
	recognizerNoSpeechThreshold = 0.6
	recognizerSilenceRMS        = 0.004
)

func (r *Recognizer) run() {
	defer close(r.done)
	defer r.lease.Release()
	for {
		select {
		case c := <-r.in:
			r.append(c)
			r.drain()
			r.maybePass(false)
		case ack := <-r.sync:
			r.drain()
			r.maybePass(false)
			close(ack)
		case <-r.stop:
			// Audio still queued belongs to the session; take it, then
			// finish everything with a forced final pass.
			r.drain()
			r.maybePass(true)
			// A refused/failed final pass must still flush previously known words.
			r.finish()
			return
		case <-r.ctx.Done():
			r.finish()
			return
		}
	}
}

// drain takes every queued chunk without blocking.
func (r *Recognizer) drain() {
	for {
		select {
		case c := <-r.in:
			r.append(c)
		default:
			return
		}
	}
}

// settle blocks until the worker has taken every chunk pushed so far and
// run any pass they were due. For tests.
func (r *Recognizer) settle() {
	ack := make(chan struct{})
	select {
	case r.sync <- ack:
		<-ack
	case <-r.done:
	}
}

// discontinuityTolerance is how far a chunk's start may sit from the end of
// the previous one before the audio is treated as a new timeline.
const discontinuityTolerance = 80 * time.Millisecond

func (r *Recognizer) append(c pcmChunk) {
	if len(r.buf) > 0 || !r.bufStart.IsZero() {
		expected := r.bufStart.Add(samplesDuration(len(r.buf)))
		if d := c.start.Sub(expected); d > discontinuityTolerance || d < -discontinuityTolerance {
			// New timeline: commit what the old one said, then restart.
			r.maybePass(true)
			r.commitAll()
			r.buf = r.buf[:0]
			r.prev = nil
			r.bufStart = c.start
		}
	} else {
		r.bufStart = c.start
	}
	r.buf = append(r.buf, c.pcm...)
	r.sincePass += len(c.pcm)
	if maxSamples := int(recognizerMaxBuffer.Seconds() * rate); len(r.buf) > maxSamples {
		drop := len(r.buf) - maxSamples
		log.Warn(r.ctx, "speech recognition is behind, dropping audio", "streamer", r.opts.Streamer, "dropped", samplesDuration(drop))
		r.commitAll()
		r.advance(drop)
	}
}

func samplesDuration(n int) time.Duration {
	return time.Duration(float64(n) / rate * float64(time.Second))
}

// advance drops the first n samples of the window.
func (r *Recognizer) advance(n int) {
	if n <= 0 {
		return
	}
	if n >= len(r.buf) {
		r.bufStart = r.bufStart.Add(samplesDuration(len(r.buf)))
		r.buf = r.buf[:0]
		return
	}
	r.bufStart = r.bufStart.Add(samplesDuration(n))
	r.buf = append(r.buf[:0], r.buf[n:]...)
}

// maybePass runs a transcription pass when enough new audio has arrived (or
// when forced), then applies the agreement, silence and bound rules.
func (r *Recognizer) maybePass(force bool) {
	have := samplesDuration(len(r.buf))
	if !force {
		if have < r.opts.MinWindow || samplesDuration(r.sincePass) < r.opts.Step {
			return
		}
	} else if have < 300*time.Millisecond {
		return
	}
	r.sincePass = 0

	if rms(r.buf) < recognizerSilenceRMS {
		// Nothing but silence: whatever was pending is done, and the
		// window moves past the silence.
		r.commitAll()
		r.advance(len(r.buf))
		r.reportCoverage(r.bufStart)
		return
	}

	model := r.lease.Model()
	if model == nil {
		// Over budget right now: keep the window bounded and try again
		// on the next step.
		if have > recognizerMaxWindow {
			r.advance(len(r.buf) - int(r.opts.MinWindow.Seconds()*rate))
		}
		return
	}
	info := model.Info()
	winStart := r.bufStart
	pcm := r.buf
	res, err := model.Transcribe(r.ctx, pcm, stt.Options{Language: r.language, Prompt: strings.Join(r.prompt, " ")})
	if err != nil {
		if r.ctx.Err() == nil {
			log.Warn(r.ctx, "speech recognition pass failed", "streamer", r.opts.Streamer, "error", err)
		}
		return
	}
	if r.language == "" && res.Language != "" {
		r.language = res.Language
	}
	r.track = Track{
		ID:       TrackID(r.opts.Origin, SourceAuto, r.trackLanguage()),
		Language: r.trackLanguage(),
		Kind:     KindCaptions,
		Source:   SourceAuto,
		Origin:   r.opts.Origin,
		Label:    "Auto captions",
		Author:   r.opts.Author,
		Model:    info.Name,
	}

	var words []Word
	if res.NoSpeechProb < recognizerNoSpeechThreshold {
		words = r.guard(winStart, pcm, suppressRepeats(res.Words))
	}
	winEnd := winStart.Add(samplesDuration(len(pcm)))

	// LocalAgreement-2: the prefix this pass shares with the previous one
	// over the same uncommitted audio is final.
	n := agreedPrefix(r.prev, words)
	// Bounded latency: past MaxWindow, words well clear of the window's
	// end are final even without agreement.
	if have > recognizerMaxWindow {
		cutoff := winEnd.Add(-r.opts.Step)
		for n < len(words) && words[n].End.Before(cutoff) {
			n++
		}
	}
	// Trailing silence: the speaker paused, so everything is final and the
	// open cue closes.
	tail := int(r.opts.SilenceFlush.Seconds() * rate)
	paused := len(pcm) > tail && rms(pcm[len(pcm)-tail:]) < recognizerSilenceRMS
	if paused {
		n = len(words)
	}

	for _, w := range words[:n] {
		r.commit(w)
	}
	r.prev = words[n:]

	switch {
	case paused:
		// The silence itself need not be heard again.
		r.commitAll()
		r.advance(len(r.buf))
	case n > 0:
		// The window now starts where the last final word ended. The
		// buffer is still the pass's audio: appends happen on this
		// goroutine, between passes.
		r.advance(int(words[n-1].End.Sub(winStart).Seconds() * rate))
	case len(words) == 0:
		// No speech found: keep only the last stretch, in case a word was
		// cut at the edge.
		keep := int(r.opts.MinWindow.Seconds() * rate)
		if len(pcm) > keep {
			r.advance(len(pcm) - keep)
		}
	}
	if force {
		r.finish()
	} else {
		r.publishInterim()
	}
	// Examining audio is not enough: the signer may consume only immutable
	// finals, so never release it past a still-open cue or uncommitted word.
	finalized := winEnd
	if cue, ok := r.grouper.Current(r.prev); ok && cue.Start.Before(finalized) {
		finalized = cue.Start
	}
	r.reportCoverage(finalized)
}

func (r *Recognizer) reportCoverage(end time.Time) {
	if end.After(r.coverage) {
		r.coverage = end
		if r.opts.OnCoverage != nil {
			r.opts.OnCoverage(end)
		}
	}
}

func (r *Recognizer) trackLanguage() string {
	if r.language == "" {
		return "und"
	}
	return r.language
}

// guard applies the hallucination filters that need the audio: words
// claimed over silence are dropped, and times are made absolute.
func (r *Recognizer) guard(winStart time.Time, pcm []float32, in []stt.Word) []Word {
	out := make([]Word, 0, len(in))
	for _, w := range in {
		text := strings.TrimSpace(w.Text)
		if text == "" {
			continue
		}
		s := int(w.Start.Seconds() * rate)
		e := int(w.End.Seconds() * rate)
		if s < 0 {
			s = 0
		}
		if e > len(pcm) {
			e = len(pcm)
		}
		if e <= s {
			e = min(s+rate/10, len(pcm))
		}
		if e > s && rms(pcm[s:e]) < recognizerSilenceRMS {
			continue
		}
		out = append(out, Word{Text: text, Start: winStart.Add(w.Start), End: winStart.Add(w.End)})
	}
	return out
}

func (r *Recognizer) commit(w Word) {
	for _, c := range r.grouper.Add(w) {
		r.publishFinal(c)
	}
	// The prompt is the recent transcript, bounded to what the model's
	// context comfortably takes.
	r.prompt = append(r.prompt, w.Text)
	if len(r.prompt) > promptWords {
		r.prompt = r.prompt[len(r.prompt)-promptWords:]
	}
}

// commitAll makes the previous pass's uncommitted words final.
func (r *Recognizer) commitAll() {
	for _, w := range r.prev {
		r.commit(w)
	}
	r.prev = nil
	for _, c := range r.grouper.Flush() {
		r.publishFinal(c)
	}
}

func (r *Recognizer) finish() {
	r.commitAll()
}

func (r *Recognizer) publishFinal(c Cue) {
	if r.track.ID == "" {
		return
	}
	r.opts.Hub.Publish(r.opts.Streamer, r.track, c)
}

func (r *Recognizer) publishInterim() {
	if r.track.ID == "" {
		return
	}
	if c, ok := r.grouper.Current(r.prev); ok {
		r.opts.Hub.Publish(r.opts.Streamer, r.track, c)
	}
}

// agreedPrefix is the length of the longest common prefix of two word
// sequences, compared by normalized text.
func agreedPrefix(prev, cur []Word) int {
	n := 0
	for n < len(prev) && n < len(cur) {
		if NormalizeWord(prev[n].Text) != NormalizeWord(cur[n].Text) {
			break
		}
		n++
	}
	return n
}

// suppressRepeats cuts a transcript at the start of a looping n-gram (1-3
// words repeated several times in a row), the signature of a whisper
// hallucination; one copy is kept.
func suppressRepeats(words []stt.Word) []stt.Word {
	norm := make([]string, len(words))
	for i, w := range words {
		norm[i] = NormalizeWord(w.Text)
	}
	for n := 1; n <= 3; n++ {
		limit := 3
		if n == 1 {
			limit = 4
		}
		for i := 0; i+n <= len(norm); i++ {
			reps := 1
			for j := i + n; j+n <= len(norm); j += n {
				same := true
				for k := range n {
					if norm[i+k] == "" || norm[i+k] != norm[j+k] {
						same = false
						break
					}
				}
				if !same {
					break
				}
				reps++
			}
			if reps >= limit {
				return words[:i+n]
			}
		}
	}
	return words
}

func rms(pcm []float32) float32 {
	if len(pcm) == 0 {
		return 0
	}
	var sum float64
	for _, s := range pcm {
		sum += float64(s) * float64(s)
	}
	return float32(math.Sqrt(sum / float64(len(pcm))))
}
