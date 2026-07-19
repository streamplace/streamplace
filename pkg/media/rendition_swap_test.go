package media

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bus"
)

func TestPumpSegmentsRenditionSwitch(t *testing.T) {
	mm, _ := getStaticTestMediaManager(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	swapCh := make(chan renditionSwap, 1)
	out := make(chan *bus.PacketizedSegment, 16)
	var latency atomic.Int64
	go pumpSegments(ctx, mm, "test-user", "source", "test-user", swapCh, &latency, out)

	seg := func(marker string) *bus.Seg {
		return &bus.Seg{
			PacketizedData: &bus.PacketizedSegment{
				Video:    [][]byte{[]byte(marker)},
				Audio:    [][]byte{[]byte("audio")},
				Duration: time.Second,
			},
			Published: true,
		}
	}
	drain := func() {
		for {
			select {
			case <-out:
			default:
				return
			}
		}
	}

	// pump subscribes from its goroutine, so retry until the subscription
	// has landed and a segment comes through
	deadline := time.Now().Add(2 * time.Second)
	for {
		mm.bus.PublishSegment(ctx, "test-user", "source", seg("source"))
		select {
		case <-out:
		case <-time.After(20 * time.Millisecond):
			if time.Now().After(deadline) {
				t.Fatal("timed out waiting for source segment")
			}
			continue
		}
		break
	}
	drain()

	// request a switch to 720p; once acked the new subscription exists
	ack := make(chan error, 1)
	swapCh <- renditionSwap{name: "720p", ack: ack}
	select {
	case err := <-ack:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for swap ack")
	}
	drain()

	// after the swap, the old rendition must not be forwarded anymore...
	mm.bus.PublishSegment(ctx, "test-user", "source", seg("source"))
	// ...and the new one must be
	mm.bus.PublishSegment(ctx, "test-user", "720p", seg("720p"))
	select {
	case p := <-out:
		require.Equal(t, "720p", string(p.Video[0]))
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for 720p segment")
	}

	// and nothing else should be in the pipe
	select {
	case p := <-out:
		t.Fatalf("unexpected segment after swap: %q", string(p.Video[0]))
	case <-time.After(100 * time.Millisecond):
	}
}
