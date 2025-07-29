package media

import (
	"context"
	"fmt"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/model"
)

// Handle shutting down a pipeline when a signing key is revoked or a user gets banned
func (mm *MediaManager) HandleKeyRevocation(ctx context.Context, ms MediaSigner, pipeline *gst.Pipeline) {
	sub := mm.bus.Subscribe(ms.Streamer())
	defer mm.bus.Unsubscribe(ms.Streamer(), sub)
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-sub:
			switch v := msg.(type) {
			case *model.SigningKey:
				if v.RevokedAt == nil {
					continue
				}
				if v.DID == ms.DID() {
					err := fmt.Errorf("signing key revoked, ending stream: %s", v.RKey)
					pipeline.Error(err.Error(), err)
					return
				}
			case *comatproto.LabelDefs_Label:
				if atproto.IsBanned(v) {
					err := fmt.Errorf("user banned, ending stream: %s", v.Uri)
					pipeline.Error(err.Error(), err)
					return
				}
			default:
				continue
			}
		}
	}
}
