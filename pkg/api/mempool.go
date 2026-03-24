package api

import (
	"context"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

// StartMempoolSubscriber subscribes to new segments from the media manager
// and feeds each signed segment through a per-streamer MUXL concatenator.
// The concatenator re-MUXLizes the signed fMP4 into structured events
// (with C2PA signatures baked in) which are delivered to the mempool.
//
// Call this once at startup. It runs until ctx is cancelled.
func (a *StreamplaceAPI) StartMempoolSubscriber(ctx context.Context) {
	segCh := a.MediaManager.NewSegment()

	// Per-streamer MUXL concatenators, keyed by repoDID.
	concats := map[string]*muxl.ConcatenatorEvents{}

	go func() {
		for {
			select {
			case <-ctx.Done():
				// Clean up all concatenators
				for _, c := range concats {
					c.Close()
				}
				return
			case notif := <-segCh:
				if !notif.Segment.Published {
					continue
				}
				streamer := notif.Segment.RepoDID
				mp := a.MempoolManager.GetOrCreate(streamer)

				// Get or create a concatenator for this streamer
				concat, ok := concats[streamer]
				if !ok {
					concat = muxl.NewConcatenatorEvents(ctx, func(ev muxl.MuxlEvent) error {
						return mp.HandleEvent(ev)
					})
					concats[streamer] = concat
				}

				// Feed the signed segment (full fMP4 with init+data) to the concatenator
				if err := concat.Write(notif.Data); err != nil {
					log.Error(ctx, "mempool: error writing to concatenator", "error", err, "streamer", streamer)
				}
			}
		}
	}()
}
