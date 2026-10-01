package captions

import (
	"context"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/stt"
)

// fakeModel answers Transcribe from a script, one result per call, and an
// empty result once the script runs out. It records every call.
type fakeModel struct {
	mu     sync.Mutex
	script []*stt.Result
	calls  int
	lens   []int
	opts   []stt.Options
}

func (m *fakeModel) Info() stt.ModelInfo { return stt.ModelInfo{Name: "whisper-fake"} }

func (m *fakeModel) Transcribe(_ context.Context, pcm []float32, opts stt.Options) (*stt.Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lens = append(m.lens, len(pcm))
	m.opts = append(m.opts, opts)
	i := m.calls
	m.calls++
	if i >= len(m.script) {
		return &stt.Result{Language: "en"}, nil
	}
	return m.script[i], nil
}

type fakeLease struct {
	model stt.Model
	nilAt int // call index from which Model returns nil; 0 = never
	calls int
}

func (l *fakeLease) Model() stt.Model {
	l.calls++
	if l.nilAt > 0 && l.calls >= l.nilAt {
		return nil
	}
	return l.model
}
func (l *fakeLease) Release() {}

type fakeEngine struct {
	lease *fakeLease
	err   error
}

func (e *fakeEngine) Lease(context.Context, stt.LeaseOptions) (stt.Lease, error) {
	if e.err != nil {
		return nil, e.err
	}
	return e.lease, nil
}
func (e *fakeEngine) Models() []stt.ModelInfo { return nil }
func (e *fakeEngine) Close() error            { return nil }

func w(text string, start, end float64) stt.Word {
	return stt.Word{Text: text, Start: time.Duration(start * float64(time.Second)), End: time.Duration(end * float64(time.Second)), Prob: 0.9}
}

func result(words ...stt.Word) *stt.Result {
	return &stt.Result{Language: "en", Words: words}
}

// speech is pseudo-random audio loud enough to count as speech.
func speech(d time.Duration) []float32 {
	n := int(d.Seconds() * rate)
	rng := rand.New(rand.NewPCG(1, 2))
	out := make([]float32, n)
	for i := range out {
		out[i] = (rng.Float32()*2 - 1) * 0.2
	}
	return out
}

func silence(d time.Duration) []float32 {
	return make([]float32, int(d.Seconds()*rate))
}

// recorded collects a streamer's hub events; wait drains the subscription
// so every published event is in events.
type recorded struct {
	events []Event
	cancel context.CancelFunc
	done   chan struct{}
}

func record(h *Hub, streamer string) *recorded {
	ctx, cancel := context.WithCancel(context.Background())
	r := &recorded{cancel: cancel, done: make(chan struct{})}
	ch := h.Subscribe(ctx, streamer)
	go func() {
		defer close(r.done)
		for ev := range ch {
			r.events = append(r.events, ev)
		}
	}()
	return r
}

func (r *recorded) wait() {
	r.cancel()
	<-r.done
}

func (r *recorded) finals() []Cue {
	var out []Cue
	for _, e := range r.events {
		if e.Cue.Final {
			out = append(out, e.Cue)
		}
	}
	return out
}

func (r *recorded) interims() []Cue {
	var out []Cue
	for _, e := range r.events {
		if !e.Cue.Final {
			out = append(out, e.Cue)
		}
	}
	return out
}

func newTestRecognizer(t *testing.T, model *fakeModel, lease *fakeLease) (*Recognizer, *recorded) {
	t.Helper()
	hub := NewHub(time.Minute)
	if lease == nil {
		lease = &fakeLease{model: model}
	}
	rec := record(hub, "did:plc:s")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	r, err := NewRecognizer(ctx, RecognizerOptions{
		Streamer: "did:plc:s", Origin: OriginCanonical, Author: "did:web:node", Hub: hub,
		Engine: &fakeEngine{lease: lease}, Languages: []string{"en"},
		// A pass per second of audio, at least two seconds in the window.
		Step: time.Second, MinWindow: 2 * time.Second, MaxWindow: 10 * time.Second,
	})
	require.NoError(t, err)
	return r, rec
}

func TestRecognizerCommitsAgreedPrefixAndFlushesOnClose(t *testing.T) {
	model := &fakeModel{script: []*stt.Result{
		result(w("hello", 0.1, 0.5), w("world", 0.6, 1.0), w("foo", 1.2, 1.6)),
		result(w("hello", 0.1, 0.5), w("world", 0.6, 1.0), w("bar", 1.2, 1.6), w("baz", 1.8, 2.2)),
		// The window now starts at the end of "world" (1.0s); offsets restart.
		result(w("bar", 0.2, 0.6), w("baz", 0.8, 1.2), w("qux", 1.4, 1.8)),
		// Close: the window starts at the end of "baz" (2.2s).
		result(w("qux", 0.2, 0.6)),
	}}
	r, rec := newTestRecognizer(t, model, nil)

	r.Push(t0, speech(2*time.Second))
	r.settle()
	r.Push(t0.Add(2*time.Second), speech(time.Second))
	r.settle()
	r.Push(t0.Add(3*time.Second), speech(time.Second))
	r.settle()
	r.Close()
	rec.wait()

	finals := rec.finals()
	require.Len(t, finals, 1, "everything commits into one short cue on close")
	require.Equal(t, "hello world bar baz qux", finals[0].Text)
	require.Equal(t, t0.Add(100*time.Millisecond), finals[0].Start, "word times are absolute, from the chunk's wall clock")
	require.Equal(t, t0.Add(2800*time.Millisecond), finals[0].End, "the last pass's offsets are relative to the moved window")
	require.Equal(t, "a0", finals[0].ID)

	interims := rec.interims()
	require.Len(t, interims, 3)
	require.Equal(t, "hello world foo", interims[0].Text, "the first pass is all interim")
	require.Equal(t, "hello world bar baz", interims[1].Text, "the agreed prefix is committed, the rest shown as interim")
	require.Equal(t, "hello world bar baz qux", interims[2].Text)
	for _, c := range interims {
		require.Equal(t, "a0", c.ID, "interim versions carry the id of the cue they will become")
	}

	require.Equal(t, Track{ID: "canonical-auto-en", Language: "en", Kind: KindCaptions, Source: SourceAuto, Origin: OriginCanonical, Label: "Auto captions", Author: "did:web:node", Model: "whisper-fake"}, r.Track())

	require.Equal(t, []int{2 * rate, 3 * rate, 3 * rate, int(1.8 * rate)}, model.lens, "the window starts where the last committed word ended")
	require.Equal(t, "hello world", model.opts[2].Prompt, "committed text is the next prompt")
	require.Equal(t, "en", model.opts[0].Language)
}

func TestRecognizerTimeMappingAcrossDiscontinuity(t *testing.T) {
	model := &fakeModel{script: []*stt.Result{
		result(w("one", 0.5, 1.0)),
		result(w("one", 0.5, 1.0), w("two", 1.5, 2.0)),
		// The jump forces a pass over the old tail (window from 1.0s).
		result(w("two", 0.5, 1.0)),
		// A fresh window starts at the new chunk's time.
		result(w("three", 0.5, 1.0)),
		result(w("three", 0.5, 1.0), w("four", 1.5, 2.0)),
		result(w("four", 0.5, 1.0)),
	}}
	r, rec := newTestRecognizer(t, model, nil)

	r.Push(t0, speech(2*time.Second))
	r.settle()
	r.Push(t0.Add(2*time.Second), speech(time.Second))
	r.settle()
	jump := t0.Add(time.Minute)
	r.Push(jump, speech(2*time.Second)) // a reconnect: new timeline
	r.settle()
	r.Push(jump.Add(2*time.Second), speech(time.Second))
	r.settle()
	r.Close()
	rec.wait()

	finals := rec.finals()
	require.Len(t, finals, 2, "the gap closes the first cue")
	require.Equal(t, "one two", finals[0].Text)
	require.Equal(t, t0.Add(500*time.Millisecond), finals[0].Start)
	require.Equal(t, t0.Add(2*time.Second), finals[0].End)
	require.Equal(t, "three four", finals[1].Text)
	require.Equal(t, jump.Add(500*time.Millisecond), finals[1].Start)
	require.Equal(t, jump.Add(2*time.Second), finals[1].End)
}

func TestRecognizerSilenceFlushesAndSkipsTranscription(t *testing.T) {
	model := &fakeModel{script: []*stt.Result{
		result(w("pause", 0.2, 0.8)),
	}}
	r, rec := newTestRecognizer(t, model, nil)
	// One second of speech then two of silence: the trailing silence makes
	// the single pass final without a second agreeing pass.
	r.Push(t0, append(speech(time.Second), silence(2*time.Second)...))
	r.settle()
	// Pure silence afterwards is never sent to the model.
	r.Push(t0.Add(3*time.Second), silence(3*time.Second))
	r.settle()
	r.Close()
	rec.wait()

	finals := rec.finals()
	require.Len(t, finals, 1)
	require.Equal(t, "pause", finals[0].Text)
	require.Equal(t, t0.Add(200*time.Millisecond), finals[0].Start)
	require.Equal(t, t0.Add(1200*time.Millisecond), finals[0].End, "a short cue is held for the minimum duration")
	require.Equal(t, 1, model.calls, "silence is not transcribed")
}

func TestRecognizerHallucinationGuards(t *testing.T) {
	loop := result(w("thank", 0.1, 0.3), w("you", 0.3, 0.5), w("thank", 0.5, 0.7), w("you", 0.7, 0.9), w("thank", 0.9, 1.1), w("you", 1.1, 1.3), w("thank", 1.3, 1.5), w("you", 1.5, 1.7))
	model := &fakeModel{script: []*stt.Result{
		{Language: "en", NoSpeechProb: 0.9, Words: []stt.Word{w("ghost", 0.1, 0.5)}},
		loop,
		loop,
	}}
	r, rec := newTestRecognizer(t, model, nil)
	r.Push(t0, speech(2*time.Second))
	r.settle()
	r.Push(t0.Add(2*time.Second), speech(time.Second))
	r.settle()
	r.Push(t0.Add(3*time.Second), speech(time.Second))
	r.settle()
	r.Close()
	rec.wait()

	var all []string
	for _, c := range rec.finals() {
		all = append(all, c.Text)
	}
	text := strings.Join(all, " ")
	require.NotContains(t, text, "ghost", "a no-speech pass is dropped")
	require.Equal(t, "thank you", text, "a looping bigram keeps one copy")

	// A word claimed over silence is dropped.
	model = &fakeModel{script: []*stt.Result{result(w("real", 0.1, 0.5), w("quiet", 2.0, 2.5))}}
	r, rec = newTestRecognizer(t, model, nil)
	r.Push(t0, append(speech(1500*time.Millisecond), silence(1500*time.Millisecond)...))
	r.settle()
	r.Close()
	rec.wait()
	finals := rec.finals()
	require.Len(t, finals, 1)
	require.Equal(t, "real", finals[0].Text)
}

func TestRecognizerNoModelMeansNoCues(t *testing.T) {
	model := &fakeModel{script: []*stt.Result{result(w("never", 0.1, 0.5))}}
	r, rec := newTestRecognizer(t, model, &fakeLease{model: model, nilAt: 1})
	r.Push(t0, speech(3*time.Second))
	r.settle()
	r.Close()
	rec.wait()
	require.Empty(t, rec.events)
	require.Equal(t, 0, model.calls)
	require.Equal(t, "", r.Track().ID)
}

func TestNewRecognizerOverBudget(t *testing.T) {
	_, err := NewRecognizer(context.Background(), RecognizerOptions{Hub: NewHub(0), Engine: &fakeEngine{err: stt.ErrOverBudget}})
	require.ErrorIs(t, err, stt.ErrOverBudget)
}

func TestSuppressRepeats(t *testing.T) {
	words := func(s string) []stt.Word {
		var out []stt.Word
		for _, t := range strings.Fields(s) {
			out = append(out, stt.Word{Text: t})
		}
		return out
	}
	texts := func(ws []stt.Word) string {
		var out []string
		for _, w := range ws {
			out = append(out, w.Text)
		}
		return strings.Join(out, " ")
	}
	require.Equal(t, "I said no", texts(suppressRepeats(words("I said no no no no no"))), "a word repeated four times loops")
	require.Equal(t, "no no no", texts(suppressRepeats(words("no no no"))), "three in a row is still speech")
	require.Equal(t, "so, so, so good", texts(suppressRepeats(words("so, so, so good"))))
	require.Equal(t, "and then the end", texts(suppressRepeats(words("and then the end the end the end"))), "a looping bigram keeps one copy")
	require.Equal(t, "a b c a b c", texts(suppressRepeats(words("a b c a b c"))), "two copies of a trigram are not a loop")
	require.Equal(t, "a b c", texts(suppressRepeats(words("a b c a b c a b c"))))
}

func TestAgreedPrefix(t *testing.T) {
	ws := func(s string) []Word {
		var out []Word
		for _, t := range strings.Fields(s) {
			out = append(out, Word{Text: t})
		}
		return out
	}
	require.Equal(t, 2, agreedPrefix(ws("Hello, world foo"), ws("hello world! bar")), "punctuation and case do not break agreement")
	require.Equal(t, 0, agreedPrefix(nil, ws("x")))
	require.Equal(t, 1, agreedPrefix(ws("a"), ws("a b c")))
}

func TestGrouperLayout(t *testing.T) {
	g := NewGrouper(DefaultCueLayout(), "c")
	at := func(s, e float64) (time.Time, time.Time) {
		return t0.Add(time.Duration(s * float64(time.Second))), t0.Add(time.Duration(e * float64(time.Second)))
	}
	add := func(text string, s, e float64) []Cue {
		st, en := at(s, e)
		return g.Add(Word{Text: text, Start: st, End: en})
	}
	// 74 characters fit in two lines; the 75th character starts a new cue.
	var closed []Cue
	words := strings.Fields("the quick brown fox jumps over the lazy dog while the cat watches from the") // 73 chars
	tm := 0.0
	for _, wd := range words {
		closed = append(closed, add(wd, tm, tm+0.2)...)
		tm += 0.25
	}
	require.Empty(t, closed)
	closed = add("window", tm, tm+0.3)
	require.Len(t, closed, 1, "overflow closes the open cue before the new word")
	require.Equal(t, "the quick brown fox jumps over the\nlazy dog while the cat watches from the", closed[0].Text)
	require.Equal(t, "c0", closed[0].ID)
	require.True(t, closed[0].Final)
	require.Len(t, closed[0].Words, len(words))

	// A pause longer than MaxGap starts a new cue.
	closed = add("later", tm+5, tm+5.5)
	require.Len(t, closed, 1)
	require.Equal(t, "window", closed[0].Text)
	require.Equal(t, "c1", closed[0].ID)
	require.Equal(t, time.Second, closed[0].End.Sub(closed[0].Start), "short cues are held for MinDuration")

	// A sentence end at a full line closes the cue right away.
	g2 := NewGrouper(DefaultCueLayout(), "s")
	t2 := 0.0
	var sentenceClosed []Cue
	for _, wd := range strings.Fields("this sentence is exactly long enough to end here.") { // 49 chars
		st, en := at(t2, t2+0.2)
		sentenceClosed = append(sentenceClosed, g2.Add(Word{Text: wd, Start: st, End: en})...)
		t2 += 0.25
	}
	require.Len(t, sentenceClosed, 1)
	require.Equal(t, "this sentence is exactly\nlong enough to end here.", sentenceClosed[0].Text, "two lines are balanced")
	require.False(t, g2.Open())

	// MaxDuration splits long unbroken speech.
	g3 := NewGrouper(DefaultCueLayout(), "d")
	for i := range 7 {
		require.Empty(t, g3.Add(Word{Text: "w", Start: t0.Add(time.Duration(i) * time.Second), End: t0.Add(time.Duration(i+1) * time.Second)}))
	}
	closed = g3.Add(Word{Text: "late", Start: t0.Add(7 * time.Second), End: t0.Add(8 * time.Second)})
	require.Len(t, closed, 1)
	require.Equal(t, "w w w w w w w", closed[0].Text)
	require.Equal(t, 7*time.Second, closed[0].End.Sub(closed[0].Start))

	// Current shows the open cue plus interim words under the open id.
	cur, ok := g3.Current([]Word{{Text: "maybe", Start: t0.Add(8 * time.Second), End: t0.Add(9 * time.Second)}})
	require.True(t, ok)
	require.Equal(t, "late maybe", cur.Text)
	require.Equal(t, "d1", cur.ID)
	require.False(t, cur.Final)
	flushed := g3.Flush()
	require.Len(t, flushed, 1)
	require.Equal(t, "late", flushed[0].Text)
	require.Equal(t, "d1", flushed[0].ID)
	_, ok = g3.Current(nil)
	require.False(t, ok)
	require.Empty(t, g3.Flush())
}

func TestWrapLines(t *testing.T) {
	require.Equal(t, "short", WrapLines("short", 37, 2))
	require.Equal(t, "one two three four five six seven\neight nine ten eleven twelve", WrapLines("one two three four five six seven eight nine ten eleven twelve", 37, 2))
	require.Equal(t, "a\nsupercalifragilisticexpialidocious", WrapLines("a supercalifragilisticexpialidocious", 10, 2), "a word longer than the line stands alone")
	require.Equal(t, "a b c d e f g h", WrapLines("a b c d e f g h", 5, 1), "a single line never wraps")
	require.Equal(t, "aa bb\ncc dd\nee ff", WrapLines("aa bb cc dd ee ff", 5, 3))
	require.Equal(t, "", WrapLines("   ", 37, 2))
}

func TestRecognizerCoverageFollowsDecisionsAtMonotonicWindowEnd(t *testing.T) {
	model := &fakeModel{}
	var covered []time.Time
	start := time.UnixMilli(1000)
	r, err := NewRecognizer(context.Background(), RecognizerOptions{
		Streamer: "coverage", Origin: OriginCanonical, Hub: NewHub(time.Minute),
		Engine: &fakeEngine{lease: &fakeLease{model: model}},
		Step:   time.Second, MinWindow: 2 * time.Second,
		OnCoverage: func(end time.Time) { covered = append(covered, end) },
	})
	require.NoError(t, err)
	r.Push(start, speech(time.Second))
	r.settle()
	require.Empty(t, covered, "PCM receipt alone must not release a held GoP")
	r.Push(start.Add(time.Second), speech(time.Second))
	r.settle()
	require.Equal(t, []time.Time{start.Add(2 * time.Second)}, covered)
	// A pure-silence decision also covers its complete window.
	r.Push(start.Add(3*time.Second), silence(2*time.Second))
	r.settle()
	require.Equal(t, start.Add(5*time.Second), covered[len(covered)-1])
	// A backwards discontinuity cannot move the reported coverage backwards.
	before := len(covered)
	r.Push(start, silence(2*time.Second))
	r.settle()
	require.Len(t, covered, before)
	r.Close()
}

func TestRecognizerEOFCoverageIncludesFinalVoicedWords(t *testing.T) {
	hub := NewHub(time.Minute)
	start := time.UnixMilli(1000)
	var atCoverage []Cue
	var covered time.Time
	model := &fakeModel{script: []*stt.Result{result(w("last words", 0.1, 0.7))}}
	r, err := NewRecognizer(context.Background(), RecognizerOptions{
		Streamer: "eof", Origin: OriginCanonical, Hub: hub,
		Engine: &fakeEngine{lease: &fakeLease{model: model}}, MinWindow: 2 * time.Second,
		OnCoverage: func(end time.Time) {
			covered = end
			atCoverage = hub.Cues("eof", TrackID(OriginCanonical, SourceAuto, "en"), start, end)
		},
	})
	require.NoError(t, err)
	r.Push(start, speech(time.Second))
	r.Close()
	require.Equal(t, start.Add(time.Second), covered)
	require.Len(t, atCoverage, 1, "the final voiced pass must publish before releasing the last GoP")
	require.Equal(t, "last words", atCoverage[0].Text)
	require.True(t, atCoverage[0].Final)
}
