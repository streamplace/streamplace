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

// liveWindowSize is the number of recent segments per track kept in memory.
// Older segments fall out of the live playlist.
const liveWindowSize = 12

// liveWindowRetention removes segments after a stream stalls or ends. This
// lets GetLiveWindow report the stream as offline without new segments arriving
// to drive count-based eviction.
const liveWindowRetention = 30 * time.Second

const (
	llhlsWindowSegments   = 30
	llhlsWindowBytes      = 64 << 20
	llhlsLivePartHoldBack = 5 * llhlsPartTarget
)

// llhlsCompletionHold keeps a finished parent open briefly so a blocking
// reload for its final part can observe and fetch that part before completion
// moves the parent into the segment-only portion of the playlist.
const llhlsCompletionHold = 300 * time.Millisecond

func newLLWindow() *llhls.Window {
	return llhls.NewWindow(
		llhls.WithMaxSegments(llhlsWindowSegments),
		llhls.WithMaxBytes(llhlsWindowBytes),
		llhls.WithDynamicTargetDuration(time.Second),
		llhls.WithPlaylistDurations(0, llhlsPartTarget),
		llhls.WithPartHoldBack(llhlsLivePartHoldBack),
		llhls.WithSegmentCompletionDelay(llhlsCompletionHold),
	)
}

func (mm *MediaManager) replaceLLWindow(did string) *llhls.Window {
	mm.llWindowsMut.Lock()
	defer mm.llWindowsMut.Unlock()
	if mm.llWindows == nil {
		mm.llWindows = make(map[string]*llhls.Window)
	}
	w := newLLWindow()
	mm.llWindows[did] = w
	return w
}

// GetLLWindow returns the low-latency window for a stream. Unlike the legacy
// window, its lifetime is tied to the CMAF presentation and reconnects replace
// the map entry.
func (mm *MediaManager) GetLLWindow(did string) *llhls.Window {
	mm.llWindowsMut.Lock()
	defer mm.llWindowsMut.Unlock()
	return mm.llWindows[did]
}

func (mm *MediaManager) removeLLWindow(did, presentation string, expected *llhls.Window) {
	mm.llWindowsMut.Lock()
	defer mm.llWindowsMut.Unlock()
	window := mm.llWindows[did]
	if window == expected && window != nil && (window.Presentation() == "" || window.Presentation() == presentation) {
		delete(mm.llWindows, did)
	}
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
