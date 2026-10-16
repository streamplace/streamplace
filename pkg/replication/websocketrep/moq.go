package websocketrep

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	official "moq.dev/moq"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/moq"
	"stream.place/streamplace/pkg/spmetrics"
)

// MoqSourceTrack is the MoQ track carrying a stream's signed source
// segments; the rendition addenda ride media.RenditionsChannel. The
// broadcast path is the streamer's DID. One group per segment, one frame
// per group: the bare canonical MUXL segment, exactly the bytes the
// websocket path ships (see docs/moq.md).
const MoqSourceTrack = "source"

// moqTracks are the tracks a broadcast serves, each with how many of the
// bus's recent segments a new subscriber is primed with, like the
// websocket path, and its priority: under contention the source, which
// carries the audio, goes first.
var moqTracks = map[string]struct {
	buf      int
	priority uint8
}{
	MoqSourceTrack:          {buf: 2, priority: 200},
	media.RenditionsChannel: {buf: 4, priority: 150},
}

// moqMaxAgeUs is how long a group that is no longer the newest stays
// deliverable, on both ends: a subscriber that falls behind by more skips
// ahead rather than validating stale segments.
const moqMaxAgeUs = uint64(10 * time.Second / time.Microsecond)

// moqLivenessInterval is how often a served broadcast checks that its
// stream is still live on the bus.
var moqLivenessInterval = time.Second

// moqTLSRetry is how often a listener without a certificate yet, a new
// node's ACME one still on its way, asks for it again.
var moqTLSRetry = 5 * time.Second

// serveMoQ runs the node's Media over QUIC listener on --moq-addr: every
// stream live on this node's bus, for the peers that pull it.
func (r *WebsocketReplicator) serveMoQ(ctx context.Context) error {
	if r.MoqTLS == nil {
		return fmt.Errorf("moq listener has no TLS files")
	}
	for {
		err := r.MoqTLS.Sync()
		if err == nil {
			break
		}
		log.Warn(ctx, "moq listener is waiting for its TLS certificate", "error", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(moqTLSRetry):
		}
	}
	origin := official.NewOriginProducer()
	requests, err := origin.Dynamic("", official.Route{})
	if err != nil {
		return err
	}
	defer requests.Cancel()
	srv, err := official.Listen(ctx, r.cli.MoqAddr,
		official.WithTLSCert(r.MoqTLS.CertPath), official.WithTLSKey(r.MoqTLS.KeyPath),
		official.WithServerPublishOrigin(origin))
	if err != nil {
		return fmt.Errorf("listening for moq: %w", err)
	}
	defer srv.Close()
	log.Log(ctx, "moq server starting", "addr", srv.LocalAddr())
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		r.MoqTLS.Refresh(gctx, func(err error) { log.Error(gctx, "refreshing moq TLS", "error", err) })
		return nil
	})
	g.Go(func() error { return srv.Serve(gctx) })
	g.Go(func() error {
		var wg sync.WaitGroup
		defer wg.Wait()
		for req, err := range requests.Requests(gctx) {
			if err != nil {
				return err
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				r.serveBroadcast(gctx, req)
			}()
		}
		return nil
	})
	return g.Wait()
}

// serveBroadcast answers a peer's request for a streamer's broadcast: a
// stream that is not live here is refused rather than waited on, and a
// live one is served until it ends on the bus.
func (r *WebsocketReplicator) serveBroadcast(ctx context.Context, req *official.BroadcastRequest) {
	streamer, err := req.Path()
	if err != nil {
		return
	}
	if r.cli.DisableSyndication || !r.bus.HasSegments(streamer, MoqSourceTrack) {
		_ = req.Reject(404)
		return
	}
	broadcast, err := official.NewBroadcastProducer()
	if err != nil {
		_ = req.Reject(500)
		return
	}
	defer broadcast.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		ticker := time.NewTicker(moqLivenessInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !r.bus.HasSegments(streamer, MoqSourceTrack) {
					cancel()
					return
				}
			}
		}
	}()
	r.serveTracks(ctx, streamer, broadcast, func() error { return req.Accept(broadcast) })
}

// serveTracks answers broadcast's track requests from the bus until ctx
// ends; publish makes broadcast reachable once it can take requests.
func (r *WebsocketReplicator) serveTracks(ctx context.Context, streamer string, broadcast *official.BroadcastProducer, publish func() error) {
	requests, err := broadcast.Dynamic()
	if err != nil {
		log.Warn(ctx, "moq: could not serve tracks", "streamer", streamer, "error", err)
		return
	}
	defer requests.Cancel()
	if err := publish(); err != nil {
		log.Warn(ctx, "moq: could not publish broadcast", "streamer", streamer, "error", err)
		return
	}
	var wg sync.WaitGroup
	defer wg.Wait()
	for req, err := range requests.Requests(ctx) {
		if err != nil {
			return
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.serveTrack(ctx, streamer, req)
		}()
	}
}

// serveTrack feeds one requested track from the bus for as long as ctx,
// the broadcast, lasts: the transport shares a served track among all its
// subscribers, so their coming and going is not this track's to follow. A
// segment that is not MUXL aborts the track: this transport carries
// nothing else.
func (r *WebsocketReplicator) serveTrack(ctx context.Context, streamer string, req *official.TrackRequest) {
	name, err := req.Name()
	spec, ok := moqTracks[name]
	if err != nil || !ok {
		_ = req.Abort(404)
		return
	}
	maxAge := moqMaxAgeUs
	track, err := req.Accept(&official.TrackInfo{Priority: spec.priority, MaxAgeUs: &maxAge})
	if err != nil {
		return
	}
	defer func() { _ = track.Finish() }()
	ch := r.bus.SubscribeSegmentBuf(ctx, streamer, name, spec.buf)
	defer r.bus.UnsubscribeSegment(context.Background(), streamer, name, ch)
	for {
		select {
		case <-ctx.Done():
			return
		case seg := <-ch.C:
			if !seg.Published {
				continue
			}
			if !isMuxl(seg.Muxl) {
				log.Warn(ctx, "moq: segment is not MUXL, ending the track", "streamer", streamer, "track", name, "segment", seg.Filepath)
				_ = track.Abort(500)
				return
			}
			frame := official.Frame{Payload: seg.Muxl, TimestampUs: uint64(time.Now().UnixMicro())}
			if err := track.WriteFrame(frame); err != nil {
				return
			}
		}
	}
}

// dialMoq opens a session to a moqt:// URL, pinning the certificate its
// certhash parameter names when it has one.
func (r *WebsocketReplicator) dialMoq(ctx context.Context, rawURL string) (*official.Client, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse moq URL: %w", err)
	}
	opts := []official.ClientOption{
		official.WithPublishOrigin(official.NewOriginProducer()),
		official.WithSubscribeOrigin(official.NewOriginProducer()),
		// The pull and push loops own reconnects, so each attempt
		// rediscovers the URL and its certificate hash.
		official.WithReconnect(false),
		official.WithWebSocketEnabled(false),
	}
	if q := u.Query(); q.Has(moq.CertHashParam) {
		pin := q.Get(moq.CertHashParam)
		if decoded, err := hex.DecodeString(pin); err != nil || len(decoded) != 32 {
			return nil, fmt.Errorf("invalid moq certificate SHA-256 fingerprint")
		}
		opts = append(opts, official.WithTLSFingerprints(pin))
		q.Del(moq.CertHashParam)
		u.RawQuery = q.Encode()
	}
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	client, err := official.Dial(dialCtx, u.String(), opts...)
	if err != nil {
		spmetrics.ReplicationConnectErrorsTotal.Inc()
		return nil, fmt.Errorf("%w (%s): %w", errMoqUnreachable, rawURL, err)
	}
	return client, nil
}

// requestMoqBroadcast resolves streamer's broadcast on a session to a peer.
// The peer's route arrives just after the session opens, so this waits for
// it, but not forever; the peer then serves the broadcast or refuses it.
func requestMoqBroadcast(ctx context.Context, client *official.Client, streamer string) (*official.BroadcastConsumer, error) {
	announced, err := client.AnnouncedBroadcast(streamer)
	if err != nil {
		return nil, err
	}
	defer announced.Cancel()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return announced.Available(ctx)
}

// consume subscribes to one streamer's source and rendition tracks and
// feeds them through validation until the broadcast or ctx ends; it always
// returns an error saying why. Anything that fails validation ends it: the
// wire carries MUXL and nothing else.
func (r *WebsocketReplicator) consume(ctx context.Context, broadcast *official.BroadcastConsumer, streamer string) error {
	spmetrics.ReplicationOutboundOpen.WithLabelValues(streamer).Inc()
	defer spmetrics.ReplicationOutboundOpen.WithLabelValues(streamer).Dec()
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return consumeTrack(gctx, broadcast, MoqSourceTrack, func(payload []byte) error {
			if err := r.mm.ValidateReplicatedMP4(context.Background(), bytes.NewReader(payload), streamer); err != nil {
				return fmt.Errorf("could not validate segment: %w", err)
			}
			return nil
		})
	})
	g.Go(func() error {
		return consumeTrack(gctx, broadcast, media.RenditionsChannel, func(payload []byte) error {
			return r.feedRenditions(gctx, streamer, payload)
		})
	})
	return g.Wait()
}

// consumeTrack hands each segment of a track to handle, in sequence order,
// until the track ends, handle fails or ctx ends.
func consumeTrack(ctx context.Context, broadcast *official.BroadcastConsumer, name string, handle func([]byte) error) error {
	track, err := broadcast.SubscribeTrack(ctx, name, &official.Subscription{Priority: moqTracks[name].priority, MaxAgeUs: moqMaxAgeUs})
	if err != nil {
		spmetrics.ReplicationConnectErrorsTotal.Inc()
		return fmt.Errorf("could not subscribe to %s: %w", name, err)
	}
	defer track.Cancel()
	for {
		group, err := track.NextGroup(ctx)
		if err != nil {
			return fmt.Errorf("%s track: %w", name, err)
		}
		if group == nil {
			return fmt.Errorf("%s track ended", name)
		}
		payload, err := readMoqGroup(ctx, group)
		if err != nil {
			return fmt.Errorf("%s track: %w", name, err)
		}
		if err := handle(payload); err != nil {
			return err
		}
	}
}

// readMoqGroup reads a group's one frame, the bare canonical segment.
func readMoqGroup(ctx context.Context, group *official.GroupConsumer) ([]byte, error) {
	defer group.Cancel()
	frame, err := group.ReadFrame(ctx)
	if err != nil {
		return nil, err
	}
	if frame == nil || !isMuxl(frame.Payload) {
		return nil, fmt.Errorf("group %d is not a signed MUXL segment", group.Sequence())
	}
	extra, err := group.ReadFrame(ctx)
	if err != nil {
		return nil, err
	}
	if extra != nil {
		return nil, fmt.Errorf("group %d holds more than one segment", group.Sequence())
	}
	return frame.Payload, nil
}

// isMuxl recognises a bare canonical segment: it leads with the c2pa or
// muxl uuid box, never with ftyp/moov (a presentation MP4) or anything else.
func isMuxl(b []byte) bool {
	return len(b) >= 8 && string(b[4:8]) == "uuid"
}

// errMoqUnreachable is a peer whose MoQ listener could not be dialled.
var errMoqUnreachable = errors.New("could not dial moq")
