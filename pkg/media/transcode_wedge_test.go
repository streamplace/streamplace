package media

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/test/remote"
)

// TestStreamTranscoderDoubleGopWedge replays the captured input that wedged a
// live stream: three normal ~1s GoPs to prime the continuous encoder, then one
// double-length (~2s, 120 video / 100 audio samples) GoP. That double-length
// GoP was the sole ~2s inter-arrival in a 10-minute capture, and it's exactly
// where ingest froze.
//
// The failure mode it guards against: gst's default queue caps at
// max-size-time=1s, so a 2s GoP overflowed the video queue's time cap, blocked
// qtdemux (which feeds both branches), starved the audio branch, and
// deadlocked mp4mux — backpressuring appsrc until the synchronous Feed
// (feedW.Write into an unbuffered io.Pipe) blocked, wedging the serial ingest
// path. Before the Queue2Big sizing fix this Fataled on the 25s timeout
// (completing only 2/4); with it the 2s GoP flows through.
func TestStreamTranscoderDoubleGopWedge(t *testing.T) {
	// Captured run-2 window: three 1s primes then the 2s double GoP (#185).
	files := []string{
		remote.RemoteFixture("1c0c04ea0f96a6abbaaf9985ea3691c3ac54728f0a52e0575eb5968e61ca30b2/2026-05-28T18-34-19-607Z-transcoder-feed-aac-00182.m4s"),
		remote.RemoteFixture("536cf3325314e5ecbd88ac482ec5253ee781cc47298de64b2a1820e156d7ba83/2026-05-28T18-34-20-616Z-transcoder-feed-aac-00183.m4s"),
		remote.RemoteFixture("7b3e0d0e33e61eadb549aea9a60347714ed6182858c1e39a2033e4e00383c881/2026-05-28T18-34-21-610Z-transcoder-feed-aac-00184.m4s"),
		remote.RemoteFixture("caada3d7c3226a9fb7ed5b9b20e34efbf7154d8662254a8cbbf33ecff5ca497d/2026-05-28T18-34-23-645Z-transcoder-feed-aac-00185.m4s"),
	}

	ctx := context.Background()
	ms := newBareSegmentSigner(t)
	mm := &MediaManager{cli: &config.CLI{BroadcasterHost: "test.example.com"}}
	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
	require.NoError(t, err)

	var mu sync.Mutex
	completed := 0
	tr := mm.newStreamTranscoder(ctx, "aac", ms.Cert, keyPEM, func(_ any, _ []byte) {
		mu.Lock()
		completed++
		mu.Unlock()
	})

	// Feed in a goroutine so the wedge surfaces as a timeout rather than a hang:
	// Feed blocks (feedW.Write) when the pipeline deadlocks.
	done := make(chan error, 1)
	go func() {
		for i, f := range files {
			b, rerr := os.ReadFile(f)
			if rerr != nil {
				done <- rerr
				return
			}
			if ferr := tr.Feed(b, i); ferr != nil {
				done <- fmt.Errorf("feed %d: %w", i, ferr)
				return
			}
		}
		done <- tr.Close()
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(25 * time.Second):
		mu.Lock()
		n := completed
		mu.Unlock()
		t.Fatalf("WEDGED: feed+close did not finish in 25s (completed %d/%d) — "+
			"a double-length GoP stalled the transcode pipeline and blocked Feed", n, len(files))
	}

	// All four GoPs complete: the three primes plus the 2s GoP (flushed by Close).
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, len(files), completed, "every fed GoP should complete (incl. the 2s double GoP)")
}
