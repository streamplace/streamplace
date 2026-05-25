package media

import (
	"bytes"
	"context"

	"stream.place/streamplace/pkg/livehls"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

// liveWindowSize is how many recent segments per track the in-memory live-HLS
// window keeps — the sliding window served to players. Bounds per-stream
// memory; older segments fall out of the playlist (DVR depth is a later,
// storage-backed feature).
const liveWindowSize = 12

// liveWindow returns the streamer's in-memory live-HLS window, creating it on
// first use.
func (mm *MediaManager) liveWindow(did string) *livehls.Writer {
	mm.liveWindowsMut.Lock()
	defer mm.liveWindowsMut.Unlock()
	w := mm.liveWindows[did]
	if w == nil {
		w = livehls.NewWriter(livehls.WithWindow(liveWindowSize))
		mm.liveWindows[did] = w
	}
	return w
}

// GetLiveWindow returns the streamer's live-HLS window, or nil if no segments
// have been observed for them yet. The serving layer (XRPC live playlists)
// reads playlists + segment bytes out of it.
func (mm *MediaManager) GetLiveWindow(did string) *livehls.Writer {
	mm.liveWindowsMut.Lock()
	defer mm.liveWindowsMut.Unlock()
	return mm.liveWindows[did]
}

// feedLiveWindow re-derives the per-track event stream from a stored canonical
// segment and folds it into the streamer's live-HLS window. Called for every
// validated segment — local or replicated — so a node serves live HLS for any
// stream whose segments flow through its ValidateMP4. Best-effort: window
// errors are logged, never fatal to ingest.
func (mm *MediaManager) feedLiveWindow(ctx context.Context, did string, segment []byte) {
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
