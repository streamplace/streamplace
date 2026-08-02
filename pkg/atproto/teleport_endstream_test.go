package atproto

import (
	"context"
	"testing"

	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"
	comatproto "stream.place/streamplace/pkg/comatproto"
)

// TestEndLivestreamForTeleportWithoutOATProxy: ending a livestream for a
// teleport needs an OAuth proxy to act as the streamer, but not every
// synchronizer has one — `streamplace sync` runs without it, and the serve path
// once forgot to wire it. With a stored session present the code used to march
// straight into OATProxy.RefreshIfNeeded and nil-deref; since this runs on a
// time.AfterFunc goroutine, that panic took down the whole node in production.
// Best-effort means a missing proxy is a logged no-op, never a crash.
func TestEndLivestreamForTeleportWithoutOATProxy(t *testing.T) {
	ctx := context.Background()
	atsync, _, _ := offlineSynchronizer(t)
	require.Nil(t, atsync.OATProxy)

	streamer := "did:plc:cccccccccccccccccccccccc"
	ref := &comatproto.RepoStrongRef{
		Uri: "at://" + streamer + "/place.stream.livestream/3lteleport0001",
		Cid: "bafyreihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenxquvyku",
	}

	// No stored session: returns before ever touching the proxy.
	atsync.endLivestreamForTeleport(ctx, streamer, ref)

	// Stored session: this is the production crash — the session lookup
	// succeeds and the next step is the nil proxy.
	require.NoError(t, atsync.StatefulDB.CreateOAuthSession("test-jkt", &oatproxy.OAuthSession{
		DID:               streamer,
		Handle:            "streamer.test",
		PDSUrl:            "http://127.0.0.1:1",
		DownstreamDPoPJKT: "test-jkt",
	}))
	atsync.endLivestreamForTeleport(ctx, streamer, ref)
}
