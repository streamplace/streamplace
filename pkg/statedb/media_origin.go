package statedb

import (
	"context"
	"fmt"
)

// IndexOwnMediaOrigin records in the local index that this node holds the given
// blob, so a freshly published VOD is listable immediately.
//
// pkg/vod publishes the place.stream.media.origin record to the server repo and
// would otherwise wait for it to federate back over the firehose before the
// index learns about it — a round-trip that has silently failed for extended
// stretches (it is down entirely on a --secure node whose self-subscription
// can't dial its own listener). getVideoList hides any video without an origin
// row, so a dropped event means a permanently unlistable-but-playable video.
//
// This lives on StatefulDB because pkg/vod deliberately doesn't import
// pkg/model (it can then run as a standalone microservice), but it already
// holds a *StatefulDB — so this is the seam that reaches the index without
// widening that dependency.
func (state *StatefulDB) IndexOwnMediaOrigin(ctx context.Context, blobCID string, size int64, mimeType string) error {
	if state.model == nil {
		// Standalone/microservice deployments run without an index; the
		// firehose path on the indexing node is then the only writer.
		return nil
	}
	if err := state.model.UpsertOwnMediaOrigin(ctx, state.CLI.ServerDID(), blobCID, size, mimeType); err != nil {
		return fmt.Errorf("index own media origin: %w", err)
	}
	return nil
}
