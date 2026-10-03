package media

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

// archiveText is one GoP's archival text runs, keyed by the start of the GoP's
// span on the media timeline (the TextRequest muxl made for it). Runs is nil
// when the live GoP's text already matches.
type archiveText struct {
	StartMs uint64            `json:"startMs"`
	Runs    map[uint32][]byte `json:"runs,omitempty"`
}

// captionArchiveSlack covers signing and delivery after the archive pass's
// hold, so a recording waits for a slow GoP rather than giving up early.
const captionArchiveSlack = 5 * time.Second

// captionArchiveLimit bounds GoPs nothing claims, e.g. when the stream is not
// being recorded.
const captionArchiveLimit = 64

// captionArchive collects an ingest session's archival text runs until the
// recording claims them. Live segments go out as soon as they are signed; the
// recorded copy of each one waits here for captions laid out in the GoP where
// they were spoken. It rides the ingest context; see withCaptionArchive.
type captionArchive struct {
	wait     time.Duration
	mu       sync.Mutex
	changed  chan struct{}
	texts    map[uint64]archiveText
	finished bool
}

func newCaptionArchive(hold time.Duration) *captionArchive {
	return &captionArchive{wait: hold + captionArchiveSlack, changed: make(chan struct{}), texts: make(map[uint64]archiveText)}
}

func (a *captionArchive) put(t archiveText) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.texts[t.StartMs] = t
	for len(a.texts) > captionArchiveLimit {
		oldest := t.StartMs
		for start := range a.texts {
			oldest = min(oldest, start)
		}
		delete(a.texts, oldest)
	}
	a.signal()
	return nil
}

// finish records that the archive pass has handed over every GoP, so claims
// for anything else stop waiting.
func (a *captionArchive) finish() {
	a.mu.Lock()
	a.finished = true
	a.signal()
	a.mu.Unlock()
}

func (a *captionArchive) signal() { close(a.changed); a.changed = make(chan struct{}) }

// claim waits for the archive pass to report the GoP starting at startMs. It
// gives up once the pass has finished without it, or after the hold.
func (a *captionArchive) claim(ctx context.Context, startMs uint64) (archiveText, bool) {
	timer := time.NewTimer(a.wait)
	defer timer.Stop()
	a.mu.Lock()
	defer a.mu.Unlock()
	for {
		if t, ok := a.texts[startMs]; ok {
			delete(a.texts, startMs)
			return t, true
		}
		if a.finished {
			return archiveText{}, false
		}
		changed := a.changed
		a.mu.Unlock()
		select {
		case <-ctx.Done():
			a.mu.Lock()
			return archiveText{}, false
		case <-timer.C:
			log.Warn(ctx, "recording the live captions: no archive pass for this GoP", "startMs", startMs)
			a.mu.Lock()
			return archiveText{}, false
		case <-changed:
		}
		a.mu.Lock()
	}
}

// copy returns the bytes to record for the live GoP seg: seg with its text
// runs replaced by the archival ones. It falls back to seg if the archive pass
// never reports that GoP.
func (a *captionArchive) copy(ctx context.Context, seg []byte) []byte {
	events, err := unwrapMuxlEvents(ctx, seg)
	catalog, segment := catalogAndSegment(events)
	start, ok := gopStartMs(catalog, segment)
	if err != nil || !ok {
		log.Warn(ctx, "recording the live captions: cannot place the segment's GoP", "error", err)
		return seg
	}
	text, ok := a.claim(ctx, start)
	if !ok || text.Runs == nil {
		return seg
	}
	out, err := replaceTextRuns(catalog, segment.Tracks, text.Runs)
	if err != nil {
		log.Warn(ctx, "recording the live captions: could not replace them", "error", err)
		return seg
	}
	return out
}

// gopStartMs is the start of a GoP's span as muxl's streaming signer reports it
// to TextFn: the first decode time of the lowest video track, else of the
// lowest audio track, in whole milliseconds.
func gopStartMs(catalog *muxl.MuxlCatalog, segment *muxl.MuxlEvent) (uint64, bool) {
	if catalog == nil || segment == nil {
		return 0, false
	}
	var id, scale uint32
	pick := func(trackID, timescale uint32) {
		if id == 0 || trackID < id {
			id, scale = trackID, timescale
		}
	}
	if catalog.Video != nil {
		for _, video := range catalog.Video.Renditions {
			pick(video.TrackID(), video.Timescale())
		}
	}
	if id == 0 && catalog.Audio != nil {
		for _, audio := range catalog.Audio.Renditions {
			pick(audio.TrackID(), audio.Timescale())
		}
	}
	ticks, ok := segment.FirstDecodeTimes[strconv.FormatUint(uint64(id), 10)]
	if id == 0 || !ok {
		return 0, false
	}
	if scale == 0 {
		return ticks, true
	}
	ts := uint64(scale)
	return ticks/ts*1000 + ticks%ts*1000/ts, true
}

// replaceTextRuns swaps a GoP's text runs for archival ones. Every other run is
// kept byte for byte, and the result is in ascending track-ID order.
func replaceTextRuns(catalog *muxl.MuxlCatalog, tracks map[string][]byte, runs map[uint32][]byte) ([]byte, error) {
	if catalog.Text != nil {
		for _, text := range catalog.Text.Renditions {
			delete(tracks, strconv.FormatUint(uint64(text.TrackID()), 10))
		}
	}
	for id, run := range runs {
		key := strconv.FormatUint(uint64(id), 10)
		if _, ok := tracks[key]; ok {
			return nil, fmt.Errorf("archive text track %d collides with a media track", id)
		}
		tracks[key] = run
	}
	return concatTracksByID(tracks), nil
}

type captionArchiveKey struct{}

// withCaptionArchive marks ctx's ingest session as recording archival captions
// into archive. The context reaches both the session's signer and ValidateMP4
// for its segments, so a segment carries its own session's archive even when
// it is validated after the signer has returned.
func withCaptionArchive(ctx context.Context, archive *captionArchive) context.Context {
	return context.WithValue(ctx, captionArchiveKey{}, archive)
}

func captionArchiveFrom(ctx context.Context) *captionArchive {
	archive, _ := ctx.Value(captionArchiveKey{}).(*captionArchive)
	return archive
}

// ArchiveCopy returns the bytes to record for this segment. For an ingest
// session that masters captions that is Muxl with its text runs laid out again
// by the archive pass, which can take up to --captions-master-delay; otherwise
// it is Muxl itself.
func (n *NewSegmentNotification) ArchiveCopy(ctx context.Context) []byte {
	if n.archive == nil {
		return n.Muxl
	}
	return n.archive.copy(ctx, n.Muxl)
}
