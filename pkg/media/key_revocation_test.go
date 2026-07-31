package media

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/comatproto"
)

// TestStreamKickMarshalsAsPlaceStreamError locks the dashboard wire contract: a
// StreamKick must marshal to the place.stream.error frame the client turns into
// a "problem" (js/.../websocket-consumer.tsx keys on $type/code/message).
func TestStreamKickMarshalsAsPlaceStreamError(t *testing.T) {
	bs, err := json.Marshal(NewStreamKick("bitrate", "too high"))
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(bs, &got))
	require.Equal(t, "place.stream.error", got["$type"])
	require.Equal(t, "bitrate", got["code"])
	require.Equal(t, "too high", got["message"])
}

// TestWatchKeyRevocationBan checks the shared detection core: a banned label
// published to the streamer's bus channel fires onRevoked. (The bus only
// delivers to already-registered subscribers, so we re-publish on a tick until
// the watcher's subscription is live.)
func TestWatchKeyRevocationBan(t *testing.T) {
	mm, _, _ := getStaticTestMediaManager(t)
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

// TestWatchKeyRevocationStreamKick checks that a StreamKick published to the
// streamer's bus channel fires onRevoked with its message — the path the max
// live bitrate enforcement uses to tear a stream down across every ingest path.
func TestWatchKeyRevocationStreamKick(t *testing.T) {
	mm, _, _ := getStaticTestMediaManager(t)
	ms := newBareSegmentSigner(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	revoked := make(chan string, 1)
	go mm.watchKeyRevocation(ctx, ms.Streamer(), ms.DID(), func(reason string) { revoked <- reason })

	kick := NewStreamKick("bitrate", "bitrate too high")
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case reason := <-revoked:
			require.Equal(t, "bitrate too high", reason)
			return
		case <-deadline:
			t.Fatal("StreamKick did not trigger teardown")
		case <-tick.C:
			mm.bus.Publish(ms.Streamer(), kick)
		}
	}
}

// TestMP4IngestIsolatedBanContained proves the fix end to end: banning a streamer
// mid-ingest tears their isolated worker down. The watchdog is set generously
// (60s) and the input is a wedging audio-only fMP4 that never ends on its own —
// so a timely return can only be the ban kill, not the watchdog or a natural
// EOS. (The 4-audio sample-stream.mp4 previously used here now ingests to
// completion in a few seconds, which would race the ban.)
func TestMP4IngestIsolatedBanContained(t *testing.T) {
	old := ingestWorkerWatchdog
	ingestWorkerWatchdog = 60 * time.Second
	defer func() { ingestWorkerWatchdog = old }()

	mm, _, _ := getStaticTestMediaManager(t)
	ms := newBareSegmentSigner(t)
	wedge := makeAudioOnlyAACFMP4(t, context.Background(), 5)

	// Ban the streamer once the worker is up and the watcher has subscribed.
	go func() {
		time.Sleep(3 * time.Second)
		mm.bus.Publish(ms.Streamer(), &comatproto.LabelDefs_Label{
			Val: atproto.LabelDMCAViolation,
			Uri: "did:plc:test-streamer",
		})
	}()

	start := time.Now()
	err := mm.MP4IngestIsolated(context.Background(), bytes.NewReader(wedge), ms)
	elapsed := time.Since(start)

	require.Error(t, err, "a banned stream is torn down, surfaced as an error")
	require.Less(t, elapsed, 30*time.Second, "the ban killed the worker well before the 60s watchdog")
	t.Logf("banned worker contained in %s: %v", elapsed.Round(time.Second), err)
}
