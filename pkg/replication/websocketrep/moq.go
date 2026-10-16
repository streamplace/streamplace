package websocketrep

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"golang.org/x/sync/errgroup"
	official "moq.dev/moq"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/moq"
	"stream.place/streamplace/pkg/placestream"
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
// stream live on this node's bus, for the peers that pull it, and the
// other way, peers pushing their streams here.
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
	g.Go(func() error {
		var wg sync.WaitGroup
		defer wg.Wait()
		for req, err := range srv.Requests(gctx) {
			if err != nil {
				return err
			}
			// Each session gets a private origin for what the peer
			// pushes: an announcement proves nothing, so it must never
			// become a route this node serves.
			pushed := official.NewOriginProducer()
			if err := req.SetConsume(pushed); err != nil {
				_ = req.Reject(gctx, 500)
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				r.acceptSession(gctx, req, pushed.Consume())
			}()
		}
		return nil
	})
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
// certhash parameter names when it has one; publish, if set, is what the
// session offers the peer (a push), else nothing (a pull).
func (r *WebsocketReplicator) dialMoq(ctx context.Context, rawURL string, publish *official.OriginProducer) (*official.Client, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse moq URL: %w", err)
	}
	if publish == nil {
		publish = official.NewOriginProducer()
	}
	opts := []official.ClientOption{
		official.WithPublishOrigin(publish),
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
	g.Go(func() error { return r.consumeSource(gctx, broadcast, streamer, nil) })
	g.Go(func() error {
		return consumeTrack(gctx, broadcast, media.RenditionsChannel, func(payload []byte) error {
			return r.feedRenditions(gctx, streamer, payload)
		})
	})
	return g.Wait()
}

// consumeSource feeds streamer's source track through validation until it
// ends; onSource, if set, runs after each segment validates.
func (r *WebsocketReplicator) consumeSource(ctx context.Context, broadcast *official.BroadcastConsumer, streamer string, onSource func()) error {
	return consumeTrack(ctx, broadcast, MoqSourceTrack, func(payload []byte) error {
		if err := r.mm.ValidateReplicatedMP4(context.Background(), bytes.NewReader(payload), streamer); err != nil {
			return fmt.Errorf("could not validate segment: %w", err)
		}
		if onSource != nil {
			onSource()
		}
		return nil
	})
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
			return fmt.Errorf("%s: %w", name, errMoqTrackEnded)
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

// errMoqTrackEnded is a track its publisher finished: the stream is over
// there, not broken.
var errMoqTrackEnded = errors.New("track ended")

// errMoqUnreachable is a peer whose MoQ listener could not be dialled.
var errMoqUnreachable = errors.New("could not dial moq")

// AdvertisedMoqURL is the moqt:// URL peers reach this node's Media over
// QUIC listener at: the --moq-url override or this host on --moq-addr's
// port, carrying the self-signed certificate's hash for the peer to pin;
// "" with no listener.
func AdvertisedMoqURL(cli *config.CLI, cert *moq.SelfSignedCert) string {
	if cli.MoqAddr == "" {
		return ""
	}
	var u *url.URL
	if cli.MoqURL != "" {
		var err error
		u, err = url.Parse(cli.MoqURL)
		// chill to panic, we're going to check this on boot
		if err != nil {
			panic("invalid moq override URL: " + cli.MoqURL)
		}
	} else {
		host := cli.ServerHost
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		_, port, err := net.SplitHostPort(cli.MoqAddr)
		if err != nil {
			panic("invalid moq listen address: " + cli.MoqAddr)
		}
		u = &url.URL{Scheme: "moqt", Host: net.JoinHostPort(host, port)}
	}
	if cert != nil {
		q := u.Query()
		q.Set(moq.CertHashParam, cert.Hash())
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// liveStream is a stream this node is ingesting, announced to --peers.
type liveStream struct {
	last time.Time
	stop context.CancelFunc
}

// touchLive notes a local segment of streamer's. On the first, the stream
// is announced on pushOrigin, its tracks served from the bus like the
// listener's; it is retracted once expireLive finds it quiet.
func (r *WebsocketReplicator) touchLive(ctx context.Context, streamer string) {
	r.liveMu.Lock()
	defer r.liveMu.Unlock()
	if s := r.live[streamer]; s != nil {
		s.last = time.Now()
		return
	}
	broadcast, err := r.pushOrigin.CreateBroadcast(streamer)
	if err != nil {
		log.Warn(ctx, "moq push: could not create broadcast", "streamer", streamer, "error", err)
		return
	}
	ctx, stop := context.WithCancel(ctx)
	r.live[streamer] = &liveStream{last: time.Now(), stop: stop}
	r.group.Go(func() error {
		defer broadcast.Close()
		r.serveTracks(ctx, streamer, broadcast, func() error { return broadcast.Announce(official.Route{}) })
		return nil
	})
}

// expireLive retracts the streams with no local segment for the stream
// session timeout: the moment the director ends the session and the bus
// forgets the stream.
func (r *WebsocketReplicator) expireLive() {
	r.liveMu.Lock()
	defer r.liveMu.Unlock()
	for streamer, s := range r.live {
		if time.Since(s.last) > r.cli.StreamSessionTimeout {
			s.stop()
			delete(r.live, streamer)
		}
	}
}

// push keeps a publishing session open to one --peers entry for as long as
// the node runs: dial, serve the peer's subscriptions until the session
// drops, back off, dial again. The peer learns which streams to subscribe
// to from the announcements, so an idle session (nothing live here) is
// kept too; it is what makes the next stream reach the peer at once.
func (r *WebsocketReplicator) push(ctx context.Context, peer string) {
	ctx = log.WithLogValues(ctx, "peer", peer)
	backoff := pullBackoffMin
	for {
		started := time.Now()
		err := r.openPush(ctx, peer)
		if ctx.Err() != nil {
			return
		}
		if time.Since(started) > pullHealthyAfter {
			backoff = pullBackoffMin
		}
		if backoff > pullBackoffMax {
			backoff = pullBackoffMax
		}
		log.Warn(ctx, "moq push: session to peer ended, retrying", "error", err, "retry_in", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
	}
}

// openPush dials the peer, publishing this node's live streams, and holds
// the session until it ends; it always returns an error saying why.
func (r *WebsocketReplicator) openPush(ctx context.Context, peer string) error {
	moqURL, err := r.resolvePeer(ctx, peer)
	if err != nil {
		return err
	}
	client, err := r.dialMoq(ctx, moqURL, r.pushOrigin)
	if err != nil {
		return err
	}
	defer client.Close()
	log.Log(ctx, "moq push: connected to peer", "url", moqURL)
	if err := client.Session().Closed(ctx); err != nil {
		return err
	}
	return fmt.Errorf("session closed")
}

// resolvePeer turns a --peers entry into the MoQ URL to dial. A moqt://
// URL is dialled as given. An http(s):// URL is a node's web origin, asked
// for its current MoQ URL on every dial: a node behind an HTTPS proxy
// serves MoQ with a self-signed certificate it rotates, so the hash a
// fixed URL would carry goes stale.
func (r *WebsocketReplicator) resolvePeer(ctx context.Context, peer string) (string, error) {
	u, err := url.Parse(peer)
	if err != nil {
		return "", fmt.Errorf("could not parse peer %q: %w", peer, err)
	}
	switch u.Scheme {
	case "moqt", "moq", "moql", "moqs":
		return peer, nil
	case "http", "https":
	default:
		return "", fmt.Errorf("peer %q: want a moqt:// or http(s):// URL", peer)
	}
	u.Path, u.RawQuery = "/xrpc/place.stream.broadcast.getBroadcaster", ""
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	res, err := aqhttp.Do(ctx, req)
	if err != nil {
		return "", fmt.Errorf("asking peer for its moq url: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("asking peer for its moq url: %s", res.Status)
	}
	var out placestream.BroadcastGetBroadcaster_Output
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decoding peer's getBroadcaster: %w", err)
	}
	if out.MoqURL == nil || *out.MoqURL == "" {
		return "", fmt.Errorf("peer %s has no moq listener", u.Host)
	}
	return *out.MoqURL, nil
}

// acceptSession holds one incoming session while acceptPush takes what
// the peer pushes, and ends it when that stops.
func (r *WebsocketReplicator) acceptSession(ctx context.Context, req *official.Request, pushed *official.OriginConsumer) {
	acceptCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	sess, err := req.Accept(acceptCtx)
	cancel()
	if err != nil {
		log.Debug(ctx, "moq: accepting a session failed", "error", err)
		return
	}
	defer sess.Shutdown()
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()
	go func() {
		_ = sess.Closed(ctx)
		cancel()
	}()
	r.acceptPush(ctx, pushed)
}

// moqPushMaxStreams bounds the streams one session may push at once. A
// node ingests a handful; a peer announcing more is not describing
// streams, and is ignored past this rather than allowed to grow our state.
const moqPushMaxStreams = 32

// acceptPush is the receiving end of push: the streams a peer announces
// are the ones it is ingesting. Each one's source is pulled and fed through
// the same validation as a pull; anything that fails validation ends the
// session, the wire carrying MUXL and nothing else. While a pushed
// stream's segments validate this node also puts itself in the station's
// broadcast_origins for it, so peers sharing the statedb pull it from
// here: the pusher is typically behind NAT, and its own origin record
// names a listener nobody can reach. The row follows validated segments,
// never the announcement alone: naming a DID proves nothing, a signed
// segment does.
func (r *WebsocketReplicator) acceptPush(ctx context.Context, pushed *official.OriginConsumer) {
	ann, err := pushed.Announced(official.AnnounceOptions{})
	if err != nil {
		log.Warn(ctx, "moq push: could not watch announcements", "error", err)
		return
	}
	defer ann.Cancel()
	g, gctx := errgroup.WithContext(ctx)
	running := map[string]context.CancelFunc{}
	for {
		a, err := ann.Next(gctx)
		if err != nil || a == nil {
			break
		}
		streamer := a.Prefix()
		stop, ok := running[streamer]
		if !a.Active() {
			if ok {
				stop()
				delete(running, streamer)
			}
			continue
		}
		if ok {
			continue
		}
		if _, err := syntax.ParseDID(streamer); err != nil {
			log.Warn(ctx, "moq push: ignoring an announcement that is not a streamer", "path", streamer)
			continue
		}
		if len(running) >= moqPushMaxStreams {
			log.Warn(ctx, "moq push: peer pushes too many streams, ignoring one", "streamer", streamer)
			continue
		}
		if err := r.cli.StreamIsAllowed(streamer); err != nil {
			log.Warn(ctx, "moq push: refusing pushed stream", "streamer", streamer, "error", err)
			continue
		}
		sctx, stop := context.WithCancel(gctx)
		running[streamer] = stop
		g.Go(func() error {
			log.Log(sctx, "moq push: receiving pushed stream", "streamer", streamer)
			r.pushedMu.Lock()
			r.pushed[streamer]++
			r.pushedMu.Unlock()
			defer func() {
				r.pushedMu.Lock()
				r.pushed[streamer]--
				if r.pushed[streamer] == 0 {
					delete(r.pushed, streamer)
				}
				r.pushedMu.Unlock()
			}()
			// Only the source: renditions a pusher sends are signed by
			// whoever it is, and this node, the stream's origin now, mints
			// its own (see startTranscodeRole).
			broadcast, err := pushed.RequestBroadcast(sctx, streamer)
			if err == nil {
				err = r.consumeSource(sctx, broadcast, streamer, r.originAdvertiser(sctx, streamer))
			}
			if sctx.Err() != nil || errors.Is(err, errMoqTrackEnded) {
				// Retracted or finished by the peer, or the session is over.
				return nil
			}
			return fmt.Errorf("pushed stream %s: %w", streamer, err)
		})
	}
	for _, stop := range running {
		stop()
	}
	if err := g.Wait(); err != nil {
		log.Warn(ctx, "moq push: dropping the session", "error", err)
	}
}

// ReceivesPush reports whether a peer is pushing streamer's stream here.
func (r *WebsocketReplicator) ReceivesPush(streamer string) bool {
	r.pushedMu.Lock()
	defer r.pushedMu.Unlock()
	return r.pushed[streamer] > 0
}

// originRefresh is how often a pushed stream's broadcast_origins row is
// touched while its segments validate; peers read the table every
// originPollInterval.
var originRefresh = 5 * time.Second

// originAdvertiser returns a per-validated-segment hook that keeps this
// node's broadcast_origins row for streamer fresh; nil with no statedb.
func (r *WebsocketReplicator) originAdvertiser(ctx context.Context, streamer string) func() {
	if r.state == nil {
		return nil
	}
	var last time.Time
	return func() {
		if time.Since(last) < originRefresh {
			return
		}
		last = time.Now()
		if err := r.state.UpsertBroadcastOrigin(streamer, r.cli.ServerDID(), r.getMyMoqURL(), last); err != nil {
			log.Warn(ctx, "moq push: could not record this node as origin", "streamer", streamer, "error", err)
		}
	}
}
