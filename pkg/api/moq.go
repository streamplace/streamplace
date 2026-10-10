package api

import (
	"context"
	"fmt"
	"net"
	"time"

	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/moq"
)

// MoqSourceTrack is the MoQ track carrying a stream's signed source
// segments; the rendition addenda ride media.RenditionsChannel. The
// broadcast path is the streamer's DID. One group per segment, one frame
// per group: the bare canonical MUXL segment, exactly the bytes the
// websocket path ships (see docs/moq.md).
const MoqSourceTrack = "source"

// ServeMoQ publishes every live stream's MUXL segments over Media over QUIC
// on --moq-addr, to raw QUIC and WebTransport subscribers alike. It is the
// transport peers prefer to place.stream.live.subscribeSegments.
func (a *StreamplaceAPI) ServeMoQ(ctx context.Context) error {
	pc, err := net.ListenPacket("udp", a.CLI.MoqAddr)
	if err != nil {
		return fmt.Errorf("listening for moq: %w", err)
	}
	defer pc.Close()
	log.Log(ctx, "moq server starting", "addr", pc.LocalAddr())
	srv := &moq.Server{TLSConfig: a.MoqTLS, Publisher: &moqPublisher{bus: a.Bus, cli: a.CLI}}
	return srv.Serve(ctx, pc)
}

// moqPublisher serves the segment bus: a subscriber to a live streamer's
// tracks gets what the bus publishes from then on, primed with the last
// few segments like the websocket path.
type moqPublisher struct {
	bus *bus.Bus
	cli *config.CLI
}

func (p *moqPublisher) Track(ctx context.Context, broadcast, name string) (moq.Track, error) {
	if p.cli.DisableSyndication {
		return nil, moq.ErrNotFound
	}
	var buf int
	switch name {
	case MoqSourceTrack:
		buf = 2
	case media.RenditionsChannel:
		buf = 4
	default:
		return nil, moq.ErrNotFound
	}
	// A stream that is not live here is refused rather than waited on;
	// renditions are checked against the source, since a stream need not
	// have any.
	if !p.bus.HasSegments(broadcast, MoqSourceTrack) {
		return nil, moq.ErrNotFound
	}
	ctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	t := &moqTrack{bus: p.bus, broadcast: broadcast, name: name, cancel: cancel}
	t.ch = p.bus.SubscribeSegmentBuf(ctx, broadcast, name, buf)
	return t, nil
}

type moqTrack struct {
	bus       *bus.Bus
	broadcast string
	name      string
	ch        *bus.SegChan
	cancel    context.CancelFunc
	seq       uint64
}

func (t *moqTrack) Info() moq.TrackInfo {
	return moq.TrackInfo{MaxAge: 10 * time.Second, Timescale: 1000}
}

// Next waits for the next published segment. A segment that is not MUXL is
// an error, which resets the subscription: this transport carries nothing
// else.
func (t *moqTrack) Next(ctx context.Context) (*moq.Group, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case seg := <-t.ch.C:
			if !seg.Published {
				continue
			}
			if !isMuxl(seg.Muxl) {
				return nil, fmt.Errorf("segment %s of %s/%s is not a MUXL segment", seg.Filepath, t.broadcast, t.name)
			}
			g := &moq.Group{Sequence: t.seq, Frames: []moq.Frame{{Timestamp: time.Now().UnixMilli(), Payload: seg.Muxl}}}
			t.seq++
			return g, nil
		}
	}
}

func (t *moqTrack) Close() {
	t.cancel()
	t.bus.UnsubscribeSegment(context.Background(), t.broadcast, t.name, t.ch)
}

// isMuxl recognises a bare canonical segment: it leads with the c2pa or
// muxl uuid box, never with ftyp/moov (a presentation MP4) or anything else.
func isMuxl(b []byte) bool {
	return len(b) >= 8 && string(b[4:8]) == "uuid"
}
