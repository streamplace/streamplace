package media

import (
	"context"
	"fmt"

	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/model"
)

// Handle shutting down a pipeline when a signing key is revoked
func (mm *MediaManager) HandleKeyRevocation(ctx context.Context, ms MediaSigner, pipeline *gst.Pipeline) {
	sub := mm.bus.Subscribe(ms.Streamer())
	defer mm.bus.Unsubscribe(ms.Streamer(), sub)
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-sub:
			signingKey, ok := msg.(*model.SigningKey)
			if !ok {
				continue
			}
			if signingKey.RevokedAt == nil {
				continue
			}
			if signingKey.DID == ms.DID() {
				err := fmt.Errorf("signing key revoked, ending stream: %s", signingKey.RKey)
				pipeline.Error(err.Error(), err)
				return
			}
		}
	}
}
