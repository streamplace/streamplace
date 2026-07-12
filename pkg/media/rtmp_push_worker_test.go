package media

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	glex "github.com/streamplace/glex/runtime"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/ingestframe"
	"stream.place/streamplace/pkg/placestream"
)

// runRTMPPushWorkerHelper is what the test binary becomes when re-exec'd with
// the `rtmp-push-worker` arg (see TestMain). It mirrors makeRTMPPushWorkerCommand:
// config on fd 3, the fMP4 source on stdin, status Event frames on fd 4; a clean
// run ends with End, a fatal error with an Error frame and a non-zero exit.
func runRTMPPushWorkerHelper() int {
	cfgFile := os.NewFile(3, "push-config")
	if cfgFile == nil {
		return 2
	}
	cfgBytes, err := io.ReadAll(cfgFile)
	cfgFile.Close()
	if err != nil {
		return 2
	}
	var cfg RTMPPushWorkerConfig
	if err := json.Unmarshal(cfgBytes, &cfg); err != nil {
		return 2
	}
	eventsFile := os.NewFile(4, "push-events")
	if eventsFile == nil {
		return 2
	}
	defer eventsFile.Close()
	events := ingestframe.NewWriter(eventsFile)
	if err := RunRTMPPushWorker(context.Background(), cfg, os.Stdin, events); err != nil {
		_ = events.Error(err.Error())
		return 1
	}
	if err := events.End(); err != nil {
		return 1
	}
	return 0
}

// TestConsumePushEvents covers the main-side status translation deterministically
// (no gst, no subprocess): Event frames are reported in order, an Error frame is
// surfaced as the returned error, and a clean End yields nil.
func TestConsumePushEvents(t *testing.T) {
	t.Run("events then clean end", func(t *testing.T) {
		pr, pw := io.Pipe()
		w := ingestframe.NewWriter(pw)
		go func() {
			_ = w.Event([]byte(`{"status":"active","message":"wrote 5 bytes"}`))
			_ = w.Event([]byte(`{"status":"active","message":"wrote 99 bytes"}`))
			_ = w.End()
			pw.Close()
		}()

		var got []pushEvent
		err := consumePushEvents(context.Background(), pr, func(status, message string) {
			got = append(got, pushEvent{Status: status, Message: message})
		})
		require.NoError(t, err)
		require.Equal(t, []pushEvent{
			{Status: "active", Message: "wrote 5 bytes"},
			{Status: "active", Message: "wrote 99 bytes"},
		}, got)
	})

	t.Run("error frame becomes the returned error", func(t *testing.T) {
		pr, pw := io.Pipe()
		w := ingestframe.NewWriter(pw)
		go func() {
			_ = w.Error("invalid target URL scheme: http")
			pw.Close()
		}()

		called := false
		err := consumePushEvents(context.Background(), pr, func(string, string) { called = true })
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid target URL scheme")
		require.False(t, called, "report must not fire on an error frame")
	})

	t.Run("torn stream is a non-nil error (worker died)", func(t *testing.T) {
		// A frame header with a payload length that never arrives = an abrupt death
		// mid-frame, which must NOT look like a clean end.
		var buf bytes.Buffer
		require.NoError(t, ingestframe.NewWriter(&buf).Event([]byte("0123456789")))
		torn := buf.Bytes()[:buf.Len()-5] // lop off the tail of the payload
		err := consumePushEvents(context.Background(), bytes.NewReader(torn), func(string, string) {})
		require.Error(t, err)
	})
}

// TestRTMPPushWorkerContainsFailure proves the containment property end to end:
// a worker whose push pipeline fails (here an invalid target scheme, which the
// shared pipeline core rejects before streaming) dies in its own subprocess and
// the supervisor returns an error — with the test process (the node) still
// running to make the assertion. In-process this same failure path stays inside
// the node; isolated, it can't take the node down.
func TestRTMPPushWorkerContainsFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// A bus is needed so the source goroutine (writeRTMPSource) doesn't nil-panic;
	// it just blocks on the empty bus until the worker fails and we cancel.
	mm := &MediaManager{bus: bus.NewBus()}
	targetView := &placestream.MultistreamDefs_TargetView{
		Uri: "at://did:plc:test/place.stream.multistream.target/abc",
		Record: &glex.LexiconTypeDecoder{
			Val: &placestream.MultistreamTarget{Url: "http://127.0.0.1:1/nope"},
		},
	}

	err := mm.RTMPPushIsolated(ctx, "did:plc:test", "source", targetView)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid target URL scheme",
		"the worker should reach the pipeline core and reject the bad scheme")
}
