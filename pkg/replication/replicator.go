package replication

import (
	"context"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/streamplace"
)

type Replicator interface {
	// start the replicator, ending on context cancellation. if your replicator doesn't need to start anything, you can just block on <-ctx.Done()
	Start(context.Context, *config.CLI) error
	// hey, we have a new segment! send it to whoever
	SendSegment(context.Context, *media.NewSegmentNotification) error
	// populate this origin record with whatever fields are pertinent to your replicator
	BuildOriginRecord(*streamplace.BroadcastOrigin) error
}
