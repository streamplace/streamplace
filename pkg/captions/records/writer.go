// Package records connects captions to the place.stream.caption.transcript
// records that outlive a stream: the Writer turns a live session's final cues
// into records, Importer turns an uploaded VTT or SRT file into records, and
// Provider reads records back as the caption tracks of a video.
//
// # Where records go
//
// Whose repo a live track is written to follows how the node came to have the
// track (captions.Track.Origin):
//
//   - canonical (mastered into the streamer's signed stream, from speech
//     recognition or from the streamer's own ingest): the streamer's repo, with
//     the streamer's OAuth session;
//   - sidecar (node captions): the node's own server repo;
//   - local and record: nowhere. Local captions are for this node's viewers
//     only, and record tracks already are records.
//
// A canonical auto track authored by this node is still written to the
// streamer's repo. Other tracks naming a third-party author are pass-through
// tracks and are never written by this node.
//
// # Cadence
//
// A session's final words are batched and written as one record per track
// every FlushInterval (60 s), plus a last one when the session ends. A record
// that cannot be written is kept, with its rkey, and retried with exponential
// backoff (at the rate limit's reset when the PDS says 429), so a retry that
// follows a write that actually landed overwrites it rather than duplicating
// it.
package records

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/captions/transcript"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spid"
)

const (
	// DefaultFlushInterval is how often a session's pending words are written
	// as records. One record a minute is 1,440 writes over a 24 hour stream.
	DefaultFlushInterval = 60 * time.Second
	// DefaultStopTimeout bounds the final flush of a session that is ending.
	DefaultStopTimeout = 30 * time.Second

	backoffMin = 5 * time.Second
	backoffMax = 5 * time.Minute

	// maxPendingWords and maxQueuedRecords bound what an unreachable PDS can
	// make a session hold: about six hours of speech either way.
	maxPendingWords  = 60000
	maxQueuedRecords = 360
)

// Target says whose repo a record is written to.
type Target struct {
	// Repo is the DID of the repo.
	Repo string
	// Node is true for the node's own server repo, false for a streamer's.
	Node bool
}

// Publisher writes one record, with a caller-chosen rkey, to a target repo and
// returns its AT URI. Writing the same rkey again replaces the record, which is
// what makes retries safe.
type Publisher interface {
	Publish(ctx context.Context, target Target, rkey string, rec *placestream.CaptionTranscript) (uri string, err error)
}

// SubjectResolver returns the strongRef of the livestream record that
// captions for the streamer's current session belong to.
type SubjectResolver func(ctx context.Context, streamer string) (comatproto.RepoStrongRef, error)

// Config configures a Writer.
type Config struct {
	Hub *captions.Hub
	// NodeDID is this node's server DID: the repo of sidecar captions and the
	// generator.node of machine-made ones.
	NodeDID   string
	Subject   SubjectResolver
	Publisher Publisher
	// Index, when set, is called with every record that was written so it is
	// searchable at once, without waiting for the firehose to bring it back.
	Index func(ctx context.Context, rec *placestream.CaptionTranscript, uri string) error

	// FlushInterval defaults to DefaultFlushInterval and StopTimeout to
	// DefaultStopTimeout.
	FlushInterval time.Duration
	StopTimeout   time.Duration
	// Chunking bounds the records a flush makes; the zero value is ionosphere's
	// five-minute chunks within the lexicon limits.
	Chunking transcript.ChunkOptions

	// Tick, Now, and RetryDelay exist for tests. Tick replaces the flush
	// timer, Now the clock for backoff and createdAt, and RetryDelay is the
	// pause between the attempts of the final flush (default 2 s).
	Tick       <-chan time.Time
	Now        func() time.Time
	RetryDelay time.Duration
}

// Writer turns live caption cues into transcript records. Start a session per
// streamer when this node starts producing captions for them and stop it when
// the stream ends; sessions are independent and each runs in its own
// goroutines, so a slow PDS never delays the hub or another stream.
type Writer struct {
	cfg Config

	mu       sync.Mutex
	sessions map[string]*session
	active   sync.WaitGroup
	closed   bool
}

// NewWriter returns a Writer, or an error when cfg lacks the hub, publisher,
// or subject resolver it cannot work without.
func NewWriter(cfg Config) (*Writer, error) {
	switch {
	case cfg.Hub == nil:
		return nil, errors.New("captions/records: no hub configured")
	case cfg.Publisher == nil:
		return nil, errors.New("captions/records: no publisher configured")
	case cfg.Subject == nil:
		return nil, errors.New("captions/records: no subject resolver configured")
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = DefaultFlushInterval
	}
	if cfg.StopTimeout <= 0 {
		cfg.StopTimeout = DefaultStopTimeout
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = 2 * time.Second
	}
	return &Writer{cfg: cfg, sessions: map[string]*session{}}, nil
}

func targetFor(nodeDID, streamer string, track captions.Track) (Target, bool) {
	switch track.Origin {
	case captions.OriginCanonical:
		authoredHere := track.Source == captions.SourceAuto && nodeDID != "" && track.Author == nodeDID
		if track.Author != "" && track.Author != streamer && !authoredHere {
			return Target{}, false
		}
		return Target{Repo: streamer}, true
	case captions.OriginSidecar:
		if track.Author != "" && track.Author != nodeDID {
			return Target{}, false
		}
		return Target{Repo: nodeDID, Node: true}, nodeDID != ""
	}
	return Target{}, false
}

// StartSession begins writing the streamer's live captions. mediaStart is the
// start time of the session's first segment, the instant every record's
// startMs counts from. Starting a streamer that already has a session is a
// no-op. The session ends with StopSession, Stop, or ctx.
func (w *Writer) StartSession(ctx context.Context, streamer string, mediaStart time.Time) {
	w.StartSessionWithOrigin(ctx, streamer, mediaStart, true)
}

// StartSessionWithOrigin also records whether this node ingests the stream.
// A relay never writes another node's canonical captions to the streamer's repo.
func (w *Writer) StartSessionWithOrigin(ctx context.Context, streamer string, mediaStart time.Time, origin bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	// A session that is already ending (its stream restarted) does not count:
	// the new one starts alongside it, and the old one finishes its flush.
	if old, ok := w.sessions[streamer]; ok && old.ctx.Err() == nil {
		return
	}
	s := &session{
		w:          w,
		streamer:   streamer,
		mediaStart: mediaStart.UTC().Truncate(time.Millisecond),
		tracks:     map[string]*trackBuf{},
		done:       make(chan struct{}),
		origin:     origin,
	}
	ctx = log.WithLogValues(ctx, "system", "caption-records", "streamer", streamer)
	ctx, s.cancel = context.WithCancel(ctx)
	s.ctx = ctx
	w.sessions[streamer] = s
	events := w.cfg.Hub.Subscribe(ctx, streamer)
	s.backfill()
	w.active.Add(1)
	go func() {
		defer w.active.Done()
		defer close(s.done)
		defer func() {
			w.mu.Lock()
			if w.sessions[streamer] == s {
				delete(w.sessions, streamer)
			}
			w.mu.Unlock()
		}()
		s.run(ctx, events)
	}()
}

// StopSession ends the streamer's session and waits for its last records to be
// written (or for StopTimeout to give up). It is a no-op for a streamer with no
// session.
func (w *Writer) StopSession(streamer string) {
	if done := w.FinishSession(streamer); done != nil {
		<-done
	}
}

// FinishSession cancels precisely the current session and returns its flush
// completion signal, allowing media teardown to continue without PDS I/O.
func (w *Writer) FinishSession(streamer string) <-chan struct{} {
	w.mu.Lock()
	defer w.mu.Unlock()
	s := w.sessions[streamer]
	if s == nil {
		return nil
	}
	s.backfill()
	s.cancel()
	return s.done
}

// Stop ends every session, waiting for each to flush.
func (w *Writer) Stop() {
	w.mu.Lock()
	w.closed = true
	all := make([]*session, 0, len(w.sessions))
	for _, s := range w.sessions {
		all = append(all, s)
	}
	w.mu.Unlock()
	for _, s := range all {
		s.cancel()
	}
	w.active.Wait()
}

type session struct {
	w          *Writer
	streamer   string
	mediaStart time.Time
	ctx        context.Context // ends the session
	cancel     context.CancelFunc
	done       chan struct{}
	origin     bool

	mu     sync.Mutex // guards tracks, which the collector and the flusher share
	tracks map[string]*trackBuf

	retryAt     time.Time // flusher only
	backoff     time.Duration
	rateLimited bool
}

// trackBuf is what one live track has produced and not yet written.
type trackBuf struct {
	track             captions.Track
	seen              map[string]time.Time // recent IDs; bounded to the reconciliation window
	latestEnd, cutoff time.Time
	pending           map[string]captions.Cue
	words             []transcript.Word // taken in, not yet part of a record
	queue             []*queued         // encoded records, oldest first, waiting to be written
}

type queued struct {
	rkey string
	rec  *placestream.CaptionTranscript
}

// backfill takes in the final cues the hub still holds, for a session started
// after the cues began.
func (s *session) backfill() {
	hub := s.w.cfg.Hub
	for _, track := range hub.Tracks(s.streamer) {
		for _, cue := range hub.Cues(s.streamer, track.ID, time.Time{}, time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)) {
			s.take(captions.Event{Streamer: s.streamer, Track: track, Cue: cue})
		}
	}
}

// take adds one final cue's words to its track's buffer.
func (s *session) take(ev captions.Event) {
	// Only final cues, and only ones from this session: the hub can still hold
	// the cues of an earlier session of a streamer whose stream restarted, and
	// those end before this session's first segment began.
	if !ev.Cue.Final || ev.Cue.End.Before(s.mediaStart) {
		return
	}
	if ev.Track.Origin == captions.OriginCanonical && !s.origin {
		return
	}
	if _, ok := targetFor(s.w.cfg.NodeDID, s.streamer, ev.Track); !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tb, ok := s.tracks[ev.Track.ID]
	if !ok {
		tb = &trackBuf{seen: map[string]time.Time{}, pending: map[string]captions.Cue{}}
		s.tracks[ev.Track.ID] = tb
	}
	tb.track = ev.Track
	if ev.Cue.End.Before(tb.cutoff) {
		return
	}
	if ev.Cue.End.After(tb.latestEnd) {
		tb.latestEnd = ev.Cue.End
	}
	if len(tb.seen) >= maxPendingWords {
		s.pruneSeen(tb)
	}
	if old, ok := tb.pending[ev.Cue.ID]; ok {
		if ev.Track.Origin == captions.OriginCanonical && old.Text == ev.Cue.Text && ev.Cue.End.After(old.End) {
			tb.pending[ev.Cue.ID] = ev.Cue
			tb.seen[ev.Cue.ID] = ev.Cue.End
		}
		return
	}
	if _, dup := tb.seen[ev.Cue.ID]; dup {
		return
	}
	if len(tb.pending) >= maxPendingWords {
		return
	}
	if len(tb.seen) >= maxPendingWords {
		return
	}
	tb.seen[ev.Cue.ID] = ev.Cue.End
	tb.pending[ev.Cue.ID] = ev.Cue
	if over := len(tb.words) - maxPendingWords; over > 0 {
		tb.words = tb.words[over:]
	}
}

func (s *session) run(ctx context.Context, events <-chan captions.Event) {
	collected := make(chan struct{})
	go func() {
		defer close(collected)
		for ev := range events {
			s.take(ev)
		}
	}()

	tick := s.w.cfg.Tick
	if tick == nil {
		t := time.NewTicker(s.w.cfg.FlushInterval)
		defer t.Stop()
		tick = t.C
	}
	for {
		select {
		case <-tick:
			s.flush(ctx, false)
			continue
		case <-ctx.Done():
		}
		break
	}
	// The hub closes the subscription once ctx is done; the collector then
	// finishes the events still queued on it.
	<-collected
	fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.w.cfg.StopTimeout)
	defer cancel()
	s.flush(fctx, true)
	// Final reconciliation also covers viewer-subscription overflow. The media
	// teardown takes a synchronous snapshot before clearing the live hub.
}

// flush encodes the buffered words of every track into records and writes the
// queued records. Unless final, a backoff in progress skips the writes; a final
// flush retries a few times before giving up.
func (s *session) flush(ctx context.Context, final bool) {
	s.backfill()
	s.mu.Lock()
	ids := make([]string, 0, len(s.tracks))
	for id := range s.tracks {
		ids = append(ids, id)
	}
	s.mu.Unlock()
	sort.Strings(ids)

	for _, id := range ids {
		s.encode(ctx, id, final)
	}
	attempts := 1
	if final {
		attempts = 4
	}
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(s.w.cfg.RetryDelay):
			case <-ctx.Done():
				return
			}
		}
		if s.w.cfg.Now().Before(s.retryAt) {
			if !final {
				return
			}
			if s.rateLimited {
				select {
				case <-time.After(s.retryAt.Sub(s.w.cfg.Now())):
				case <-ctx.Done():
					return
				}
			}
		}
		if s.publishQueued(ctx) {
			s.backoff, s.retryAt = 0, time.Time{}
			return
		}
	}
	if final {
		log.Error(ctx, "giving up on caption records at session end", "pending", s.pending())
	}
}

func (s *session) pending() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, tb := range s.tracks {
		n += len(tb.queue)
	}
	return n
}

// encode turns a track's buffered words into queued records. Without a subject
// to attach them to, the words stay buffered.
func (s *session) encode(ctx context.Context, trackID string, final bool) {
	s.mu.Lock()
	tb := s.tracks[trackID]
	s.pruneSeen(tb)
	// MUXL finals are clipped to GoPs. Keep the newest canonical cue mutable
	// until another cue follows, it settles for one flush interval, or the
	// session ends; otherwise a flush can permanently truncate its next piece.
	var latest time.Time
	if !final && tb.track.Origin == captions.OriginCanonical {
		for _, cue := range tb.pending {
			if cue.End.After(latest) {
				latest = cue.End
			}
		}
	}
	settled := s.w.cfg.Now().Add(-s.w.cfg.FlushInterval)
	for id, cue := range tb.pending {
		if cue.End.Equal(latest) && cue.End.After(settled) {
			continue
		}
		tb.words = append(tb.words, transcript.WordsFromCue(s.mediaStart, cue)...)
		delete(tb.pending, id)
	}
	if len(tb.words) == 0 {
		s.mu.Unlock()
		return
	}
	track := tb.track
	// Take the words over so the collector can keep appending to a fresh
	// buffer while the records are built.
	words := tb.words
	tb.words = nil
	s.mu.Unlock()

	subject, err := s.w.cfg.Subject(ctx, s.streamer)
	if err != nil {
		log.Warn(ctx, "caption records waiting for the livestream record", "track", trackID, "error", err)
		s.mu.Lock()
		tb.words = append(words, tb.words...)
		s.mu.Unlock()
		return
	}
	sort.SliceStable(words, func(i, j int) bool { return words[i].StartMs < words[j].StartMs })
	chunks := transcript.Chunk(words, s.w.cfg.Chunking)
	now := s.w.cfg.Now().UTC()
	built := make([]*queued, 0, len(chunks))
	for _, c := range chunks {
		built = append(built, &queued{
			rkey: spid.TIDClock.Next().String(),
			rec:  s.record(track, subject, c, now),
		})
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	tb.queue = append(tb.queue, built...)
	if over := len(tb.queue) - maxQueuedRecords; over > 0 {
		log.Warn(ctx, "dropping the oldest unwritten caption records", "track", trackID, "dropped", over)
		tb.queue = tb.queue[over:]
	}
}

func (s *session) record(track captions.Track, subject comatproto.RepoStrongRef, c transcript.Compact, now time.Time) *placestream.CaptionTranscript {
	language := track.Language
	if language == "" {
		language = "und"
	}
	kind := string(track.Kind)
	if kind == "" {
		kind = string(captions.KindCaptions)
	}
	mediaStart := s.mediaStart.Format("2006-01-02T15:04:05.000Z")
	rec := &placestream.CaptionTranscript{
		LexiconTypeID: "place.stream.caption.transcript",
		Subject:       subject,
		MediaStart:    &mediaStart,
		StartMs:       c.StartMs,
		Text:          c.Text,
		Timings:       c.Timings,
		Language:      language,
		Kind:          &kind,
		Source:        string(track.Source),
		CreatedAt:     now.Format("2006-01-02T15:04:05.000Z"),
	}
	if track.Source == captions.SourceAuto {
		gen := &placestream.CaptionTranscript_Generator{}
		if track.Model != "" {
			model := track.Model
			gen.Model = &model
		}
		if s.w.cfg.NodeDID != "" {
			node := s.w.cfg.NodeDID
			gen.Node = &node
		}
		rec.Generator = gen
	}
	return rec
}

// publishQueued writes every queued record, oldest first per track, and
// reports whether all of them were written. After a failure it schedules the
// next attempt and leaves the unwritten records queued.
func (s *session) publishQueued(ctx context.Context) bool {
	s.mu.Lock()
	ids := make([]string, 0, len(s.tracks))
	for id, tb := range s.tracks {
		if len(tb.queue) > 0 {
			ids = append(ids, id)
		}
	}
	s.mu.Unlock()
	sort.Strings(ids)

	ok := true
	for _, id := range ids {
		for {
			s.mu.Lock()
			tb := s.tracks[id]
			if len(tb.queue) == 0 {
				s.mu.Unlock()
				break
			}
			head, track := tb.queue[0], tb.track
			s.mu.Unlock()

			target, writes := targetFor(s.w.cfg.NodeDID, s.streamer, track)
			if !writes {
				s.drop(id, head)
				continue
			}
			uri, err := s.w.cfg.Publisher.Publish(ctx, target, head.rkey, head.rec)
			if err != nil {
				s.fail(ctx, id, err)
				ok = false
				if s.rateLimited {
					return false
				}
				break
			}
			log.Log(ctx, "wrote caption transcript", "uri", uri, "track", id, "words", len(transcript.Tokens(head.rec.Text)))
			if s.w.cfg.Index != nil {
				if err := s.w.cfg.Index(ctx, head.rec, uri); err != nil {
					log.Warn(ctx, "failed to index caption transcript", "uri", uri, "error", err)
				}
			}
			s.drop(id, head)
		}
	}
	return ok
}

func (s *session) drop(trackID string, q *queued) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tb := s.tracks[trackID]
	if len(tb.queue) > 0 && tb.queue[0] == q {
		tb.queue = tb.queue[1:]
	}
}

// fail schedules the next write attempt after err: at the PDS's rate limit
// reset when it gave one, otherwise after a doubling wait.
func (s *session) fail(ctx context.Context, trackID string, err error) {
	now := s.w.cfg.Now()
	wait, limited := retryAfter(err, now)
	if !limited {
		s.backoff = min(max(s.backoff*2, backoffMin), backoffMax)
		wait = s.backoff
	}
	s.retryAt = now.Add(wait)
	s.rateLimited = limited
	log.Warn(ctx, "failed to write caption transcript, will retry", "track", trackID, "retryIn", wait.String(), "rateLimited", limited, "error", err)
}

func (s *session) pruneSeen(tb *trackBuf) {
	cutoff := tb.latestEnd.Add(-s.w.cfg.Hub.Retention())
	if cutoff.After(tb.cutoff) {
		tb.cutoff = cutoff
	}
	for id, end := range tb.seen {
		if end.Before(tb.cutoff) {
			if _, mutable := tb.pending[id]; !mutable {
				delete(tb.seen, id)
			}
		}
	}
}
