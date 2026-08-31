package media

import (
	"bytes"
	"context"
	"time"

	"stream.place/streamplace/pkg/livehls"
	"stream.place/streamplace/pkg/llhls"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

// liveWindowSize is how many recent segments per track the in-memory live-HLS
// window keeps — the sliding window served to players. Bounds per-stream
// memory; older segments fall out of the playlist (DVR depth is a later,
// storage-backed feature).
const liveWindowSize = 12

// liveWindowRetention ages segments out by wall-clock arrival time, independent
// of liveWindowSize. The count window only evicts as new segments push old ones
// out, so when a stream stalls or ends its last segments would otherwise sit in
// the window forever and a retrying player replays them endlessly. With time
// eviction the window empties this long after the last segment, and the window
// is then dropped from the map (so the stream reads as offline). Generous
// enough not to cut a briefly-lagging player.
const liveWindowRetention = 30 * time.Second

const llhlsWindowSegments = 30
const llhlsWindowBytes = 64 << 20

func (mm *MediaManager) llWindow(did string) *llhls.Window {
	mm.llWindowsMut.Lock()
	defer mm.llWindowsMut.Unlock()
	w := mm.llWindows[did]
	if w == nil {
		w = llhls.NewWindow(llhls.WithMaxSegments(llhlsWindowSegments), llhls.WithMaxBytes(llhlsWindowBytes))
		mm.llWindows[did] = w
	}
	return w
}

// GetLLWindow returns the low-latency window for a stream. Unlike the legacy
// window, its lifetime is tied to the CMAF presentation and reconnects replace
// the presentation in-place.
func (mm *MediaManager) GetLLWindow(did string) *llhls.Window {
	mm.llWindowsMut.Lock()
	defer mm.llWindowsMut.Unlock()
	return mm.llWindows[did]
}

// liveWindow returns the streamer's in-memory live-HLS window, creating it on
// first use.
func (mm *MediaManager) liveWindow(did string) *livehls.Writer {
	mm.liveWindowsMut.Lock()
	defer mm.liveWindowsMut.Unlock()
	w := mm.liveWindows[did]
	if w == nil {
		w = livehls.NewWriter(livehls.WithWindow(liveWindowSize), livehls.WithRetention(liveWindowRetention))
		mm.liveWindows[did] = w
	}
	return w
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
		return nil
	}
	return w
}

// feedLiveWindow re-derives the per-track event stream from a stored canonical
// segment and folds it into the streamer's live-HLS window. Called for every
// validated segment — local or replicated — so a node serves live HLS for any
// stream whose segments flow through its ValidateMP4. Best-effort: window
// errors are logged, never fatal to ingest.
//
// Only PUBLISHED segments are folded in. Live HLS requests are unauthenticated
// today, so a pre-live (unpublished) segment in the window would be watchable by
// anyone, not just the streamer. Until HLS gains per-viewer auth we withhold
// pre-live HLS entirely: an unfed window stays nil, and the getLive* handlers
// return StreamNotLive. The streamer still monitors their own pre-live stream
// over WebRTC, which gates playback on viewer == streamer.
func (mm *MediaManager) feedLiveWindow(ctx context.Context, did string, segment []byte, published bool) {
	if !published {
		return
	}
	eventCh := make(chan *muxl.MuxlEvent, 8)
	errCh := make(chan error, 1)
	go func() {
		err := muxl.RunMuxlUnwrapEvents(ctx, bytes.NewReader(segment), eventCh)
		close(eventCh)
		errCh <- err
	}()
	w := mm.liveWindow(did)
	for ev := range eventCh {
		if err := w.Observe(ev); err != nil {
			log.Error(ctx, "live-hls: window observe failed", "streamer", did, "error", err)
		}
	}
	if err := <-errCh; err != nil {
		log.Error(ctx, "live-hls: window feed failed", "streamer", did, "error", err)
	}
}
