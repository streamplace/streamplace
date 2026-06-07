package media

import (
	"context"
	"fmt"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/model"
)

// watchKeyRevocation blocks until the streamer's signing key is revoked or the
// streamer is banned (or ctx ends), then calls onRevoked once with a reason and
// returns. It takes the streamer's bus key + DID as plain strings (not a
// MediaSigner) so it can also be driven from just the resume metadata of a
// worker that outlived a main restart. It is the shared core behind both the
// in-process HandleKeyRevocation (which errors the gst pipeline) and the
// isolated-worker supervisors (which kill the worker subprocess). The latter
// matters because an isolated worker has no bus/model of its own, so it cannot
// notice a ban — main has to watch on its behalf, or a banned user keeps
// streaming.
func (mm *MediaManager) watchKeyRevocation(ctx context.Context, streamer, did string, onRevoked func(reason string)) {
	sub := mm.bus.Subscribe(streamer)
	defer mm.bus.Unsubscribe(streamer, sub)
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-sub:
			switch v := msg.(type) {
			case *model.SigningKey:
				if v.RevokedAt != nil && v.DID == did {
					onRevoked(fmt.Sprintf("signing key revoked: %s", v.RKey))
					return
				}
			case *comatproto.LabelDefs_Label:
				if atproto.IsBanned(v) {
					onRevoked(fmt.Sprintf("user banned: %s", v.Uri))
					return
				}
			}
		}
	}
}

// HandleKeyRevocation shuts down an in-process ingest pipeline when the
// streamer's signing key is revoked or they get banned.
func (mm *MediaManager) HandleKeyRevocation(ctx context.Context, ms MediaSigner, pipeline *gst.Pipeline) {
	mm.watchKeyRevocation(ctx, ms.Streamer(), ms.DID(), func(reason string) {
		err := fmt.Errorf("ending stream: %s", reason)
		pipeline.Error(err.Error(), err)
	})
}
