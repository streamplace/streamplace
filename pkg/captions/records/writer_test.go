package records

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/captions/transcript"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/placestream"
)

const (
	streamer = "did:plc:streamer"
	nodeDID  = "did:web:node.example"
)

var t0 = time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

type publishCall struct {
	target Target
	rkey   string
	rec    *placestream.CaptionTranscript
}

// fakeRepos stands in for the PDS and the server repo.
type fakeRepos struct {
	mu    sync.Mutex
	calls []publishCall
	// fail, when set, decides the error of each call by its 0-based index.
	fail func(i int) error
	// block, when set, holds every call until it is closed.
	block chan struct{}
}

func (f *fakeRepos) Publish(ctx context.Context, target Target, rkey string, rec *placestream.CaptionTranscript) (string, error) {
	if f.block != nil {
		<-f.block
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	i := len(f.calls)
	f.calls = append(f.calls, publishCall{target, rkey, rec})
	if f.fail != nil {
		if err := f.fail(i); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("at://%s/place.stream.caption.transcript/%s", target.Repo, rkey), nil
}

func (f *fakeRepos) snapshot() []publishCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]publishCall(nil), f.calls...)
}

// written returns the calls that succeeded: the records that exist.
func (f *fakeRepos) written() []publishCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []publishCall
	for i, c := range f.calls {
		if f.fail == nil || f.fail(i) == nil {
			out = append(out, c)
		}
	}
	return out
}

type harness struct {
	t     *testing.T
	hub   *captions.Hub
	repos *fakeRepos
	w     *Writer
	tick  chan time.Time
	now   time.Time
	nowMu sync.Mutex
}

func (h *harness) clock() time.Time {
	h.nowMu.Lock()
	defer h.nowMu.Unlock()
	return h.now
}

func (h *harness) advance(d time.Duration) {
	h.nowMu.Lock()
	defer h.nowMu.Unlock()
	h.now = h.now.Add(d)
}

func newHarness(t *testing.T, mutate func(*Config)) *harness {
	t.Helper()
	h := &harness{t: t, hub: captions.NewHub(0), repos: &fakeRepos{}, tick: make(chan time.Time), now: t0.Add(time.Hour)}
	cfg := Config{
		Hub:     h.hub,
		NodeDID: nodeDID,
		Subject: func(context.Context, string) (comatproto.RepoStrongRef, error) {
			return comatproto.RepoStrongRef{LexiconTypeID: "com.atproto.repo.strongRef", Uri: "at://" + streamer + "/place.stream.livestream/3abc", Cid: "bafylive"}, nil
		},
		Publisher:   h.repos,
		Tick:        h.tick,
		Now:         h.clock,
		RetryDelay:  time.Millisecond,
		StopTimeout: 5 * time.Second,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	w, err := NewWriter(cfg)
	require.NoError(t, err)
	h.w = w
	t.Cleanup(w.Stop)
	return h
}

// flushed triggers a flush and returns once it has run. The tick channel is
// unbuffered, so the second send returns only after the first flush finished.
func (h *harness) flushed() {
	h.t.Helper()
	h.tick <- time.Time{}
	h.tick <- time.Time{}
}

// buffered waits until the session has taken in n words, so a flush sees them.
func (h *harness) buffered(streamer string, n int) {
	h.t.Helper()
	require.Eventually(h.t, func() bool {
		h.w.mu.Lock()
		s := h.w.sessions[streamer]
		h.w.mu.Unlock()
		if s == nil {
			return false
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		total := 0
		for _, tb := range s.tracks {
			total += len(tb.words)
			for _, cue := range tb.pending {
				total += len(transcript.WordsFromCue(s.mediaStart, cue))
			}
		}
		return total >= n
	}, 5*time.Second, time.Millisecond)
}

var autoTrack = captions.Track{
	ID: "canonical-auto-en", Language: "en", Kind: captions.KindCaptions,
	Source: captions.SourceAuto, Origin: captions.OriginCanonical, Model: "whisper-base-q5_1",
}

// say publishes a final cue of words, 300ms each, starting offsetMs after t0.
func say(h *harness, track captions.Track, id string, offsetMs int64, words ...string) {
	cue := captions.Cue{ID: id, Final: true, Start: t0.Add(ms(offsetMs))}
	at := offsetMs
	for _, w := range words {
		cue.Words = append(cue.Words, captions.Word{Text: w, Start: t0.Add(ms(at)), End: t0.Add(ms(at + 300))})
		at += 300
	}
	cue.End = t0.Add(ms(at))
	cue.Text = fmt.Sprint(words)
	h.hub.Publish(streamer, track, cue)
}

func ms(n int64) time.Duration { return time.Duration(n) * time.Millisecond }

func decode(rec *placestream.CaptionTranscript) []transcript.Word {
	return transcript.Decode(transcript.Compact{Text: rec.Text, StartMs: rec.StartMs, Timings: rec.Timings})
}

func TestWriterBatchesFinalWordsIntoOneRecordPerFlush(t *testing.T) {
	h := newHarness(t, nil)
	h.w.StartSession(context.Background(), streamer, t0)

	say(h, autoTrack, "c1", 1000, "hello", "there")
	// An interim cue is not part of the transcript.
	h.hub.Publish(streamer, autoTrack, captions.Cue{ID: "c2", Start: t0.Add(ms(2000)), End: t0.Add(ms(2500)), Text: "still talk", Words: []captions.Word{{Text: "still", Start: t0.Add(ms(2000)), End: t0.Add(ms(2300))}}})
	say(h, autoTrack, "c3", 5000, "general", "kenobi")
	h.buffered(streamer, 4)

	require.Empty(t, h.repos.snapshot(), "nothing is written between flushes")

	h.flushed()
	calls := h.repos.snapshot()
	require.Len(t, calls, 1, "one record for the whole batch")
	rec := calls[0].rec
	require.Equal(t, "hello there general kenobi", rec.Text)
	require.Equal(t, int64(1000), rec.StartMs)
	require.Equal(t, []int64{300, 300, -3400, 300, 300}, rec.Timings)
	require.Equal(t, "at://did:plc:streamer/place.stream.livestream/3abc", rec.Subject.Uri)
	require.Equal(t, "bafylive", rec.Subject.Cid)
	require.Equal(t, "2026-09-30T12:00:00.000Z", *rec.MediaStart)
	require.Equal(t, "en", rec.Language)
	require.Equal(t, "captions", *rec.Kind)
	require.Equal(t, "auto", rec.Source)
	require.Equal(t, "whisper-base-q5_1", *rec.Generator.Model)
	require.Equal(t, nodeDID, *rec.Generator.Node)
	require.NotEmpty(t, rec.CreatedAt)

	h.flushed()
	require.Len(t, h.repos.snapshot(), 1, "a flush with nothing new writes nothing")

	say(h, autoTrack, "c4", 70_000, "later")
	h.buffered(streamer, 1)
	h.flushed()
	calls = h.repos.snapshot()
	require.Len(t, calls, 2)
	require.Equal(t, "later", calls[1].rec.Text)
	require.Equal(t, int64(70_000), calls[1].rec.StartMs, "startMs always counts from mediaStart, not from the previous record")
	require.NotEqual(t, calls[0].rkey, calls[1].rkey)
}

func TestWriterFlushesTheRestWhenTheSessionEnds(t *testing.T) {
	h := newHarness(t, nil)
	h.w.StartSession(context.Background(), streamer, t0)
	say(h, autoTrack, "c1", 0, "first")
	h.buffered(streamer, 1)
	h.flushed()
	say(h, autoTrack, "c2", 2000, "goodbye")
	h.buffered(streamer, 1)

	h.w.StopSession(streamer) // returns once the last record is written
	calls := h.repos.snapshot()
	require.Len(t, calls, 2)
	require.Equal(t, "goodbye", calls[1].rec.Text)

	h.w.StopSession(streamer) // already over: nothing happens
	require.Len(t, h.repos.snapshot(), 2)
}

func TestWriterEndFlushTakesCuesStillQueuedOnTheHub(t *testing.T) {
	h := newHarness(t, nil)
	h.w.StartSession(context.Background(), streamer, t0)
	for i := range 50 {
		say(h, autoTrack, fmt.Sprintf("c%d", i), int64(i)*1000, "word")
	}
	h.w.StopSession(streamer)
	calls := h.repos.snapshot()
	require.Len(t, calls, 1)
	require.Len(t, decode(calls[0].rec), 50)
}

func TestWriterContextEndFlushesToo(t *testing.T) {
	h := newHarness(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	h.w.StartSession(ctx, streamer, t0)
	say(h, autoTrack, "c1", 0, "bye")
	h.buffered(streamer, 1)
	cancel()
	require.Eventually(t, func() bool { return len(h.repos.snapshot()) == 1 }, 5*time.Second, time.Millisecond)
}

func TestWriterRepoChoice(t *testing.T) {
	tests := []struct {
		name      string
		track     captions.Track
		want      Target
		writes    bool
		recSource string
	}{
		{
			name:   "canonical auto captions go to the streamer's repo",
			track:  captions.Track{ID: "a", Language: "en", Source: captions.SourceAuto, Origin: captions.OriginCanonical},
			want:   Target{Repo: streamer},
			writes: true,
		},
		{
			name:   "canonical ingest captions go to the streamer's repo",
			track:  captions.Track{ID: "b", Language: "en", Source: captions.SourceIngest, Origin: captions.OriginCanonical, Author: streamer},
			want:   Target{Repo: streamer},
			writes: true,
		},
		{
			name:   "sidecar captions go to the node's server repo",
			track:  captions.Track{ID: "c", Language: "en", Source: captions.SourceAuto, Origin: captions.OriginSidecar},
			want:   Target{Repo: nodeDID, Node: true},
			writes: true,
		},
		{
			name:   "sidecar captions this node authored explicitly",
			track:  captions.Track{ID: "d", Language: "en", Source: captions.SourceAuto, Origin: captions.OriginSidecar, Author: nodeDID},
			want:   Target{Repo: nodeDID, Node: true},
			writes: true,
		},
		{name: "local captions are not written", track: captions.Track{ID: "e", Language: "en", Origin: captions.OriginLocal}},
		{name: "tracks read from records are not written back", track: captions.Track{ID: "f", Language: "en", Origin: captions.OriginRecord}},
		{name: "another node's sidecar is not ours to write", track: captions.Track{ID: "g", Language: "en", Source: captions.SourceAuto, Origin: captions.OriginSidecar, Author: "did:web:other.example"}},
		{name: "a canonical auto track by someone else is a pass-through", track: captions.Track{ID: "h", Language: "en", Source: captions.SourceAuto, Origin: captions.OriginCanonical, Author: "did:plc:someoneelse"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, nil)
			h.w.StartSession(context.Background(), streamer, t0)
			say(h, tt.track, "c1", 0, "hi")
			h.w.StopSession(streamer)
			calls := h.repos.snapshot()
			if !tt.writes {
				require.Empty(t, calls)
				return
			}
			require.Len(t, calls, 1)
			require.Equal(t, tt.want, calls[0].target)
		})
	}
}

func TestWriterPersistsCanonicalNodeAuthoredAutoTrackInStreamerRepo(t *testing.T) {
	h := newHarness(t, nil)
	track := autoTrack
	track.Author = nodeDID

	h.w.StartSession(context.Background(), streamer, t0)
	say(h, track, "c1", 0, "generated", "here")
	h.w.StopSession(streamer)

	calls := h.repos.snapshot()
	require.Len(t, calls, 1)
	require.Equal(t, Target{Repo: streamer}, calls[0].target)
	require.NotNil(t, calls[0].rec.Generator)
	require.NotNil(t, calls[0].rec.Generator.Node)
	require.Equal(t, nodeDID, *calls[0].rec.Generator.Node)
}

func TestWriterTracksAreSeparateRecords(t *testing.T) {
	h := newHarness(t, nil)
	h.w.StartSession(context.Background(), streamer, t0)
	spanish := autoTrack
	spanish.ID, spanish.Language = "canonical-auto-es", "es"
	sidecar := captions.Track{ID: "sidecar-auto-en", Language: "en", Source: captions.SourceAuto, Origin: captions.OriginSidecar}
	say(h, autoTrack, "c1", 0, "hello")
	say(h, spanish, "c1", 0, "hola")
	say(h, sidecar, "c1", 0, "hello")
	h.buffered(streamer, 3)
	h.flushed()

	byTarget := map[Target]map[string]string{}
	for _, c := range h.repos.snapshot() {
		if byTarget[c.target] == nil {
			byTarget[c.target] = map[string]string{}
		}
		byTarget[c.target][c.rec.Language] = c.rec.Text
	}
	require.Equal(t, map[Target]map[string]string{
		{Repo: streamer}:            {"en": "hello", "es": "hola"},
		{Repo: nodeDID, Node: true}: {"en": "hello"},
	}, byTarget)
}

func TestWriterRetriesAFailedWriteWithTheSameRecord(t *testing.T) {
	h := newHarness(t, nil)
	h.repos.fail = func(i int) error {
		if i == 0 {
			return errors.New("pds is down")
		}
		return nil
	}
	h.w.StartSession(context.Background(), streamer, t0)
	say(h, autoTrack, "c1", 0, "keep", "me")
	h.buffered(streamer, 2)

	h.flushed()
	require.Len(t, h.repos.snapshot(), 1, "one failed attempt")
	require.Empty(t, h.repos.written())

	// Inside the backoff nothing is attempted, and new words wait with the old.
	say(h, autoTrack, "c2", 1000, "and", "me")
	h.buffered(streamer, 2)
	h.advance(time.Second)
	h.flushed()
	require.Len(t, h.repos.snapshot(), 1)

	h.advance(10 * time.Second)
	h.flushed()
	calls := h.repos.snapshot()
	require.Len(t, calls, 3, "the retry and the records built since")
	require.Equal(t, calls[0].rkey, calls[1].rkey, "the retry replaces, not duplicates")
	require.Equal(t, calls[0].rec, calls[1].rec)
	require.Equal(t, "keep me", calls[1].rec.Text)
	require.Equal(t, "and me", calls[2].rec.Text)
	require.Len(t, h.repos.written(), 2)
}

func TestWriterBackoffGrowsAndResets(t *testing.T) {
	h := newHarness(t, nil)
	h.repos.fail = func(i int) error {
		if i < 3 {
			return errors.New("nope")
		}
		return nil
	}
	h.w.StartSession(context.Background(), streamer, t0)
	say(h, autoTrack, "c1", 0, "x")
	h.buffered(streamer, 1)

	h.flushed()
	h.advance(5 * time.Second) // first wait is 5s
	h.flushed()
	require.Len(t, h.repos.snapshot(), 2)
	h.advance(5 * time.Second) // second wait is 10s, so 5s is not enough
	h.flushed()
	require.Len(t, h.repos.snapshot(), 2)
	h.advance(5 * time.Second)
	h.flushed()
	require.Len(t, h.repos.snapshot(), 3)
	h.advance(20 * time.Second)
	h.flushed()
	require.Len(t, h.repos.snapshot(), 4)
	require.Len(t, h.repos.written(), 1)

	// After a success the next failure starts over at the shortest wait.
	h.repos.fail = func(i int) error {
		if i == 4 {
			return errors.New("again")
		}
		return nil
	}
	say(h, autoTrack, "c2", 5000, "y")
	h.buffered(streamer, 1)
	h.flushed()
	require.Len(t, h.repos.snapshot(), 5)
	h.advance(5 * time.Second)
	h.flushed()
	require.Len(t, h.repos.snapshot(), 6)
}

func TestWriterWaitsOutARateLimit(t *testing.T) {
	h := newHarness(t, nil)
	reset := h.clock().Add(90 * time.Second)
	h.repos.fail = func(i int) error {
		if i == 0 {
			return echo.NewHTTPError(http.StatusTooManyRequests, fmt.Sprintf("http 429 from upstream (will reset at %s)", reset.Format(time.RFC3339)))
		}
		return nil
	}
	h.w.StartSession(context.Background(), streamer, t0)
	say(h, autoTrack, "c1", 0, "x")
	h.buffered(streamer, 1)

	h.flushed()
	h.advance(60 * time.Second) // the generic backoff would have elapsed; the limit has not
	h.flushed()
	require.Len(t, h.repos.snapshot(), 1)
	h.advance(31 * time.Second)
	h.flushed()
	require.Len(t, h.repos.written(), 1)
}

func TestWriterKeepsWordsUntilThereIsALivestreamToPointAt(t *testing.T) {
	var mu sync.Mutex
	ready := false
	h := newHarness(t, func(c *Config) {
		c.Subject = func(context.Context, string) (comatproto.RepoStrongRef, error) {
			mu.Lock()
			defer mu.Unlock()
			if !ready {
				return comatproto.RepoStrongRef{}, errors.New("not indexed yet")
			}
			return comatproto.RepoStrongRef{Uri: "at://did:plc:streamer/place.stream.livestream/late", Cid: "c"}, nil
		}
	})
	h.w.StartSession(context.Background(), streamer, t0)
	say(h, autoTrack, "c1", 0, "early")
	h.buffered(streamer, 1)
	h.flushed()
	require.Empty(t, h.repos.snapshot())

	say(h, autoTrack, "c2", 1000, "bird")
	h.buffered(streamer, 2)
	mu.Lock()
	ready = true
	mu.Unlock()
	h.flushed()
	calls := h.repos.snapshot()
	require.Len(t, calls, 1)
	require.Equal(t, "early bird", calls[0].rec.Text)
	require.Equal(t, "at://did:plc:streamer/place.stream.livestream/late", calls[0].rec.Subject.Uri)
}

func TestWriterBackfillsFromTheHubWithoutDuplicates(t *testing.T) {
	h := newHarness(t, nil)
	say(h, autoTrack, "c1", 0, "before")
	say(h, autoTrack, "c2", 1000, "start")
	h.w.StartSession(context.Background(), streamer, t0)
	// The same cue arriving live after the backfill is not counted twice.
	say(h, autoTrack, "c2", 1000, "start")
	say(h, autoTrack, "c3", 2000, "after")
	h.buffered(streamer, 3)
	h.flushed()
	calls := h.repos.snapshot()
	require.Len(t, calls, 1)
	require.Equal(t, "before start after", calls[0].rec.Text)
}

func TestWriterGivesUpAtSessionEndWithoutHanging(t *testing.T) {
	h := newHarness(t, nil)
	h.repos.fail = func(int) error { return errors.New("down for good") }
	h.w.StartSession(context.Background(), streamer, t0)
	say(h, autoTrack, "c1", 0, "x")
	h.buffered(streamer, 1)
	h.w.StopSession(streamer)
	require.Len(t, h.repos.snapshot(), 4, "the final flush tries a few times, then stops")
	require.Empty(t, h.repos.written())
}

func TestWriterNeverBlocksTheHub(t *testing.T) {
	h := newHarness(t, nil)
	h.repos.block = make(chan struct{})
	h.w.StartSession(context.Background(), streamer, t0)
	say(h, autoTrack, "c0", 0, "x")
	h.buffered(streamer, 1)
	go func() { h.tick <- time.Time{} }() // the flush now hangs inside the publisher

	done := make(chan struct{})
	go func() {
		for i := 1; i < 2000; i++ {
			say(h, autoTrack, fmt.Sprintf("c%d", i), int64(i)*1000, "word")
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the hub was held up by a writer that is stuck on the PDS")
	}
	close(h.repos.block)
}

func TestWriterIndexesWhatItWrites(t *testing.T) {
	var mu sync.Mutex
	var indexed []string
	h := newHarness(t, func(c *Config) {
		c.Index = func(_ context.Context, rec *placestream.CaptionTranscript, uri string) error {
			mu.Lock()
			defer mu.Unlock()
			indexed = append(indexed, uri+" "+rec.Text)
			return errors.New("index trouble is not a write failure")
		}
	})
	h.w.StartSession(context.Background(), streamer, t0)
	say(h, autoTrack, "c1", 0, "hi")
	h.buffered(streamer, 1)
	h.flushed()
	calls := h.repos.snapshot()
	require.Len(t, calls, 1, "a failing index does not make the write retry")
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{"at://did:plc:streamer/place.stream.caption.transcript/" + calls[0].rkey + " hi"}, indexed)
}

func TestNewWriterNeedsItsParts(t *testing.T) {
	_, err := NewWriter(Config{})
	require.Error(t, err)
	_, err = NewWriter(Config{Hub: captions.NewHub(0), Publisher: &fakeRepos{}})
	require.Error(t, err, "no subject resolver")
}

func TestRetryAfter(t *testing.T) {
	now := t0
	d, ok := retryAfter(echo.NewHTTPError(http.StatusTooManyRequests, "http 429 from upstream (will reset at 2026-09-30T12:02:00Z)"), now)
	require.True(t, ok)
	require.Equal(t, 2*time.Minute, d)

	d, ok = retryAfter(fmt.Errorf("wrapped: %w", echo.NewHTTPError(http.StatusTooManyRequests, "rate-limited by upstream, but ratelimit header not found")), now)
	require.True(t, ok)
	require.Equal(t, backoffMin, d)

	_, ok = retryAfter(echo.NewHTTPError(http.StatusInternalServerError, "boom"), now)
	require.False(t, ok)
	_, ok = retryAfter(errors.New("boom"), now)
	require.False(t, ok)
}

func TestWriterRestartedStreamGetsAFreshSessionWhileTheOldOneFlushes(t *testing.T) {
	h := newHarness(t, nil)
	h.repos.block = make(chan struct{}) // the old session's last write is stuck
	ctx1, cancel1 := context.WithCancel(context.Background())
	h.w.StartSession(ctx1, streamer, t0)
	say(h, autoTrack, "c1", 0, "before", "restart")
	h.buffered(streamer, 2)
	cancel1() // the stream timed out

	h.w.StartSession(context.Background(), streamer, t0.Add(time.Hour))
	say(h, autoTrack, "c2", 3_600_000, "after", "restart")
	h.buffered(streamer, 2)
	close(h.repos.block)
	h.w.StopSession(streamer)

	var texts []string
	for _, c := range h.repos.snapshot() {
		texts = append(texts, c.rec.Text)
	}
	require.Contains(t, texts, "before restart", "the old session still wrote its words")
	require.Contains(t, texts, "after restart", "the new session is not swallowed by the old one")
}

func TestWriterReconcilesDroppedFinalEventsAtTeardown(t *testing.T) {
	h := newHarness(t, nil)
	h.w.StartSession(context.Background(), streamer, t0)
	h.w.mu.Lock()
	s := h.w.sessions[streamer]
	h.w.mu.Unlock()
	// Block the collector, not the nonblocking viewer path, to force overflow.
	s.mu.Lock()
	for i := range 1000 {
		h.hub.Publish(streamer, autoTrack, captions.Cue{ID: fmt.Sprint(i), Text: fmt.Sprintf("word%d", i), Start: t0.Add(time.Duration(i) * time.Millisecond), End: t0.Add(time.Duration(i+1) * time.Millisecond), Final: true})
	}
	s.mu.Unlock()
	h.w.StopSession(streamer)
	calls := h.repos.snapshot()
	total := 0
	for _, call := range calls {
		total += len(decode(call.rec))
	}
	require.Equal(t, 1000, total, "archival delivery reconciles the hub after a lossy viewer subscription overflows")
}

func TestWriterShutdownWaitsForReplacedFinishingSession(t *testing.T) {
	h := newHarness(t, nil)
	h.repos.block = make(chan struct{})
	defer close(h.repos.block)
	ctx, cancel := context.WithCancel(context.Background())
	h.w.StartSession(ctx, streamer, t0)
	say(h, autoTrack, "old", 0, "old", "speech")
	h.buffered(streamer, 2)
	h.w.mu.Lock()
	old := h.w.sessions[streamer]
	h.w.mu.Unlock()
	cancel()
	require.Eventually(t, func() bool { return old.pending() == 1 }, time.Second, time.Millisecond)
	h.w.StartSession(context.Background(), streamer, t0.Add(time.Hour))
	done := make(chan struct{})
	go func() { h.w.Stop(); close(done) }()
	select {
	case <-done:
		t.Fatal("shutdown returned while the replaced session's transcript was still blocked")
	case <-time.After(30 * time.Millisecond):
	}
}

func TestWriterBoundsDedupHistoryWithoutRearchivingExpiredReplay(t *testing.T) {
	h := newHarness(t, nil)
	s := &session{w: h.w, streamer: streamer, mediaStart: t0, origin: true, tracks: map[string]*trackBuf{}}
	var first captions.Event
	for i := range 100 {
		ev := captions.Event{Streamer: streamer, Track: autoTrack, Cue: captions.Cue{ID: fmt.Sprint(i), Text: fmt.Sprintf("word%d", i), Start: t0.Add(time.Duration(i) * time.Minute), End: t0.Add(time.Duration(i)*time.Minute + time.Second), Final: true}}
		if i == 0 {
			first = ev
		}
		s.take(ev)
		s.flush(context.Background(), false)
		h.advance(time.Minute)
	}
	require.LessOrEqual(t, len(s.tracks[autoTrack.ID].seen), 16, "deduplication retains a bounded replay window, not a stream-lifetime ledger")
	s.take(first)
	s.flush(context.Background(), true)
	calls := h.repos.snapshot()
	require.Len(t, calls, 100, "an expired replay does not create another transcript")
	require.Equal(t, "word99", calls[99].rec.Text)
}

func TestWriterExtendedCanonicalIDStaysDeduplicated(t *testing.T) {
	h := newHarness(t, func(cfg *Config) { cfg.Hub = captions.NewHub(time.Second) })
	h.hub = h.w.cfg.Hub
	h.w.StartSession(context.Background(), streamer, t0)
	cue := captions.Cue{ID: "long", Text: "A long line", Start: t0.Add(100 * time.Millisecond), End: t0.Add(300 * time.Millisecond), Final: true}
	h.hub.PublishCanonical(streamer, autoTrack, cue)
	h.buffered(streamer, 3)
	cue.Start = cue.End
	cue.End = t0.Add(2 * time.Second)
	h.hub.PublishCanonical(streamer, autoTrack, cue)
	h.flushed()
	h.flushed()
	h.w.StopSession(streamer)
	calls := h.repos.snapshot()
	require.Len(t, calls, 1, "extending a canonical cue updates its deduplication lifetime, not just its pending words")
	require.Equal(t, int64(2000), decode(calls[0].rec)[2].EndMs)
}
