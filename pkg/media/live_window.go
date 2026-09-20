package media

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"stream.place/streamplace/pkg/livehls"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

// liveWindowSize is how many recent segments per track the in-memory live-HLS
// window keeps — the sliding window served to players. Bounds per-stream
// memory; older segments fall out of the playlist (DVR depth is a later,
// storage-backed feature).
const liveWindowSize = 12

// liveWindowMinDuration is the least media the count window may shrink to.
// Segments are cut at keyframes, so a scene-cut-heavy passage arrives as a
// run of one- or two-frame segments; counting those against the 12 would
// drop whole seconds at once and leave a player a few seconds behind live
// with nothing to fetch (404s — the stall at the same spot every replay).
const liveWindowMinDuration = 10 * time.Second

// liveWindowMinFragment joins signed segments shorter than this into one
// playlist fragment (they concatenate blindly and each starts at a
// keyframe). Serving a scene-cut passage one keyframe per fragment costs a
// player a round trip per 40ms of video; half a second is enough to keep a
// buffer fed and delays the live edge by at most that during the passage.
const liveWindowMinFragment = 500 * time.Millisecond

// liveWindowRetention ages segments out by wall-clock arrival time, independent
// of liveWindowSize. The count window only evicts as new segments push old ones
// out, so when a stream stalls or ends its last segments would otherwise sit in
// the window forever and a retrying player replays them endlessly. With time
// eviction the window empties this long after the last segment, and the window
// is then dropped from the map (so the stream reads as offline). Generous
// enough not to cut a briefly-lagging player.
const liveWindowRetention = 30 * time.Second

// newLiveWindow makes a live-HLS window with the node's window settings.
func newLiveWindow() *livehls.Writer {
	return livehls.NewWriter(livehls.WithWindow(liveWindowSize), livehls.WithMinDuration(liveWindowMinDuration), livehls.WithMinFragment(liveWindowMinFragment), livehls.WithRetention(liveWindowRetention))
}

// GetLiveWindow returns the streamer's live-HLS window, or nil if it has no
// live segments — either none observed yet, or all aged out (a stalled/ended
// stream). In the latter case the window is dropped from the map so it's freed
// and the stream reads as offline; it's recreated if the stream resumes.
func (mm *MediaManager) GetLiveWindow(did string) *livehls.Writer {
	mm.liveWindowsMut.Lock()
	defer mm.liveWindowsMut.Unlock()
	w := mm.liveWindows[did]
	if w != nil && w.Empty() {
		delete(mm.liveWindows, did)
		delete(mm.liveWindowPublished, did)
		delete(mm.liveWindowLatest, did)
		return nil
	}
	return w
}

// LiveWindowPublished reports whether the streamer's latest windowed segment
// was published (the stream is live to the public) rather than pre-live.
func (mm *MediaManager) LiveWindowPublished(did string) bool {
	mm.liveWindowsMut.Lock()
	defer mm.liveWindowsMut.Unlock()
	return mm.liveWindowPublished[did]
}

// feedLiveWindow re-derives the per-track event stream from a stored canonical
// segment and folds it into the streamer's live-HLS window. Called for every
// validated segment — local or replicated — so a node serves live HLS for any
// stream whose segments flow through its ValidateMP4. Best-effort: window
// errors are logged, never fatal to ingest.
//
// Unpublished (pre-live) segments are folded in too, and the window remembers
// whether its latest segment was published. The getLive* handlers keep an
// unpublished window to the streamer's own playback session (see spxrpc's
// getPlaybackSession) — the HLS counterpart of WebRTC's viewer == streamer
// gate — and answer StreamNotLive to everyone else. When the stream goes
// public the window is started over, so no preview segment is ever served
// as part of the public stream.
func (mm *MediaManager) feedLiveWindow(ctx context.Context, did string, segment []byte, start time.Time, published bool) {
	// Segments are fed from concurrent goroutines, so the window's state
	// change and the choice of writer happen under one lock: a preview
	// segment that picked its writer after the stream went public would
	// otherwise land in the public window. Nor is arrival order segment
	// order: a pre-live segment that finishes validating after the stream
	// went public is dropped rather than flipping the window back to a
	// preview under its viewers. A pre-live segment newer than everything
	// in the window is the stream going back to preview, and does flip it.
	mm.liveWindowsMut.Lock()
	if mm.liveWindowPublished == nil {
		mm.liveWindowPublished = map[string]bool{}
	}
	if mm.liveWindowLatest == nil {
		mm.liveWindowLatest = map[string]time.Time{}
	}
	if !published && mm.liveWindowPublished[did] && start.Before(mm.liveWindowLatest[did]) {
		mm.liveWindowsMut.Unlock()
		log.Debug(ctx, "dropping pre-live segment that arrived after the stream went public", "did", did, "start", start)
		return
	}
	w := mm.liveWindows[did]
	if published && !mm.liveWindowPublished[did] && w != nil {
		// The stream just went public. The window's flag is per stream, not
		// per segment, so everything in it is about to be served to anyone;
		// the pre-live preview segments still sitting in it must not be. The
		// public window starts at this segment.
		w = nil
	}
	if w == nil {
		w = newLiveWindow()
		mm.liveWindows[did] = w
	}
	mm.liveWindowPublished[did] = published
	if start.After(mm.liveWindowLatest[did]) {
		mm.liveWindowLatest[did] = start
	}
	mm.liveWindowsMut.Unlock()
	eventCh := make(chan *muxl.MuxlEvent, 8)
	errCh := make(chan error, 1)
	go func() {
		err := muxl.RunMuxlUnwrapEvents(ctx, bytes.NewReader(segment), eventCh)
		close(eventCh)
		errCh <- err
	}()
	for ev := range eventCh {
		if err := w.Observe(ev); err != nil {
			log.Error(ctx, "live-hls: window observe failed", "streamer", did, "error", err)
		}
	}
	if err := <-errCh; err != nil {
		log.Error(ctx, "live-hls: window feed failed", "streamer", did, "error", err)
	}
}

// FeedLiveRenditions folds an addendum of signed rendition tracks (see
// MintVideoRenditions) into the streamer's live-HLS window. Each rendition
// is its own track there — a variant in the master playlist — with its
// own media sequence, so an addendum arriving a beat after its source
// segment (the transcoder's round trip) is fine. start is the source
// segment's start time (zero when unknown): a pre-live addendum that lands
// after the stream went public is dropped like its source would be.
func (mm *MediaManager) FeedLiveRenditions(ctx context.Context, did string, addendum []byte, start time.Time, published bool) {
	if len(addendum) == 0 {
		return
	}
	mm.feedLiveWindow(ctx, did, addendum, start, published)
}

// LiveRenditionNames lists the transcoded renditions the streamer's live
// window currently carries, highest first, named the way the player and
// the transcode profiles name them ("720p"): what a viewer of this node can
// actually pick, whether the node transcoded them or received them from
// the origin. Empty when the stream has no window or no renditions yet.
func (mm *MediaManager) LiveRenditionNames(did string) []string {
	w := mm.GetLiveWindow(did)
	if w == nil {
		return nil
	}
	return renditionNames(w.VideoTracks())
}

// RenditionName is the name a rendition of these dimensions goes by
// ("720p"): the ladder's name, which is the short edge, so a portrait
// source's 360×640 rendition is "360p" wherever it is generated,
// advertised or requested.
func RenditionName(width, height uint32) string {
	return fmt.Sprintf("%dp", renditionEdge(width, height))
}

func renditionEdge(width, height uint32) uint32 {
	if width > 0 && width < height {
		return width
	}
	return height
}

func renditionNames(tracks []livehls.VideoTrack) []string {
	var rs []livehls.VideoTrack
	for _, t := range tracks {
		id, err := strconv.ParseUint(t.ID, 10, 32)
		if err != nil || uint32(id) < renditionTrackBase || t.Height == 0 {
			continue
		}
		rs = append(rs, t)
	}
	sort.Slice(rs, func(i, j int) bool {
		return renditionEdge(rs[i].Width, rs[i].Height) > renditionEdge(rs[j].Width, rs[j].Height)
	})
	names := make([]string, 0, len(rs))
	for _, t := range rs {
		names = append(names, RenditionName(t.Width, t.Height))
	}
	return names
}
