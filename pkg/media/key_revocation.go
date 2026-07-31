package media

import (
	"context"
	"fmt"

	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/model"
)

// PlaceStreamError is the $type of the websocket frame the dashboard renders as
// a stream "problem" (see js/.../websocket-consumer.tsx).
const PlaceStreamError = "place.stream.error"

// StreamKick is published to a streamer's bus to forcibly end their live ingest
// for a server-side reason (today: exceeding --maximum-live-bitrate). It rides
// the same per-streamer bus the ban/key-revocation watcher listens on, so
// watchKeyRevocation tears the stream down on it across every ingest path —
// in-process (errors the gst pipeline) and isolated (kills the worker
// subprocess). It also marshals to a place.stream.error frame, so the websocket
// fan-out delivers it to the streamer's dashboard as a problem explaining why
// they were disconnected. One publish does both jobs.
type StreamKick struct {
	LexiconTypeID string `json:"$type"`
	Code          string `json:"code"`
	Message       string `json:"message"`
}

// NewStreamKick builds a StreamKick carrying the correct $type.
func NewStreamKick(code, message string) *StreamKick {
	return &StreamKick{LexiconTypeID: PlaceStreamError, Code: code, Message: message}
}

// watchKeyRevocation blocks until the streamer's signing key is revoked, the
// streamer is banned, or a StreamKick is published for them (or ctx ends), then
// calls onRevoked once with a reason and returns. It takes the streamer's bus
// key + DID as plain strings (not a MediaSigner) so it can also be driven from
// just the resume metadata of a worker that outlived a main restart. It is the
// shared core behind both the in-process HandleKeyRevocation (which errors the
// gst pipeline) and the isolated-worker supervisors (which kill the worker
// subprocess). The latter matters because an isolated worker has no bus/model of
// its own, so it cannot notice a ban — main has to watch on its behalf, or a
// banned (or over-bitrate) user keeps streaming.
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
			case *StreamKick:
				onRevoked(v.Message)
				return
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
