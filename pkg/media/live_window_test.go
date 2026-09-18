package media

import (
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/livehls"
)

func TestLiveWindowFeedLockReclaimsIdleEntries(t *testing.T) {
	mm := &MediaManager{
		liveWindows:     map[string]*livehls.Writer{},
		liveWindowFeeds: map[string]*liveWindowFeed{},
	}
	did := "did:example:feed-lock"

	feed := mm.lockLiveWindowFeed(did)
	mm.unlockLiveWindowFeed(did, feed)
	require.NotContains(t, mm.liveWindowFeeds, did,
		"an idle feed lock without a live window should be reclaimed")

	mm.liveWindows[did] = livehls.NewWriter()
	feed = mm.lockLiveWindowFeed(did)
	mm.unlockLiveWindowFeed(did, feed)
	require.Contains(t, mm.liveWindowFeeds, did,
		"a feed lock must remain while its live window exists")

	require.Nil(t, mm.GetLiveWindow(did))
	require.NotContains(t, mm.liveWindowFeeds, did,
		"the lock should be reclaimed after the window expires")
}
