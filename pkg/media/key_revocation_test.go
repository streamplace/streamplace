package media

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/atproto"
)

// TestWatchKeyRevocationBan checks the shared detection core: a banned label
// published to the streamer's bus channel fires onRevoked. (The bus only
// delivers to already-registered subscribers, so we re-publish on a tick until
// the watcher's subscription is live.)
func TestWatchKeyRevocationBan(t *testing.T) {
	mm, _ := getStaticTestMediaManager(t)
	ms := newBareSegmentSigner(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	revoked := make(chan string, 1)
	go mm.watchKeyRevocation(ctx, ms.Streamer(), ms.DID(), func(reason string) { revoked <- reason })

	banned := &comatproto.LabelDefs_Label{Val: atproto.LabelDMCAViolation, Uri: "did:plc:test-streamer"}
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case reason := <-revoked:
			require.Contains(t, reason, "user banned")
			return
		case <-deadline:
			t.Fatal("ban label did not trigger key revocation")
		case <-tick.C:
			mm.bus.Publish(ms.Streamer(), banned)
		}
	}
}

// TestMKVIngestIsolatedBanContained proves the fix end to end: banning a streamer
// mid-ingest tears their isolated worker down. The watchdog is set generously
// (60s) and the input is the wedging 4-audio MKV that never ends on its own — so
// a timely return can only be the ban kill, not the watchdog or a natural EOS.
func TestMKVIngestIsolatedBanContained(t *testing.T) {
	old := ingestWorkerWatchdog
	ingestWorkerWatchdog = 60 * time.Second
	defer func() { ingestWorkerWatchdog = old }()

	mm, _ := getStaticTestMediaManager(t)
	ms := newBareSegmentSigner(t)
	wedge, err := os.ReadFile(getFixture("sample-stream.mkv"))
	require.NoError(t, err)

	// Ban the streamer once the worker is up and the watcher has subscribed.
	go func() {
		time.Sleep(3 * time.Second)
		mm.bus.Publish(ms.Streamer(), &comatproto.LabelDefs_Label{
			Val: atproto.LabelDMCAViolation,
			Uri: "did:plc:test-streamer",
		})
	}()

	start := time.Now()
	err = mm.MKVIngestIsolated(context.Background(), bytes.NewReader(wedge), ms)
	elapsed := time.Since(start)

	require.Error(t, err, "a banned stream is torn down, surfaced as an error")
	require.Less(t, elapsed, 30*time.Second, "the ban killed the worker well before the 60s watchdog")
	t.Logf("banned worker contained in %s: %v", elapsed.Round(time.Second), err)
}
