package websocketrep

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/bus"
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

// serveMoQ runs the node's Media over QUIC listener on --moq-addr: every
// live stream's MUXL segments for raw QUIC and WebTransport subscribers
// alike, and the other way, peers pushing their streams here.
func (r *WebsocketReplicator) serveMoQ(ctx context.Context) error {
	pc, err := net.ListenPacket("udp", r.cli.MoqAddr)
	if err != nil {
		return fmt.Errorf("listening for moq: %w", err)
	}
	defer pc.Close()
	log.Log(ctx, "moq server starting", "addr", pc.LocalAddr())
	srv := &moq.Server{TLSConfig: r.MoqTLS, Publisher: r.publisher, Publishers: r.acceptPush}
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

// pushPublisher is what a push session serves a peer: the bus, plus the
// streams this node is ingesting as its announced inventory. The listener
// keeps the plain publisher, so a browser asking it for routes gets one
// covering everything it serves, replicas included.
type pushPublisher struct {
	*moqPublisher
	live *liveSet
}

func (p *pushPublisher) Announce(ctx context.Context, prefix string) <-chan moq.Announcement {
	return p.live.announce(ctx, prefix)
}

// liveSet is the streams this node is ingesting right now. A streamer
// enters on its first local segment and leaves once none has arrived for
// ttl, the stream session timeout: the moment the director ends the
// session and the bus forgets the stream.
type liveSet struct {
	ttl time.Duration

	mu        sync.Mutex
	seen      map[string]time.Time
	listeners map[chan moq.Announcement]string // the prefix each asked for
}

func newLiveSet(ttl time.Duration) *liveSet {
	return &liveSet{ttl: ttl, seen: map[string]time.Time{}, listeners: map[chan moq.Announcement]string{}}
}

// touch notes a local segment of streamer's.
func (l *liveSet) touch(streamer string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, known := l.seen[streamer]
	l.seen[streamer] = time.Now()
	if !known {
		l.notify(moq.Announcement{Path: streamer, Active: true})
	}
}

// expire retracts the streams that have gone quiet.
func (l *liveSet) expire() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for streamer, last := range l.seen {
		if time.Since(last) > l.ttl {
			delete(l.seen, streamer)
			l.notify(moq.Announcement{Path: streamer})
		}
	}
}

// notify hands an announcement to every listener whose prefix it matches.
// A listener that is not keeping up is closed rather than left behind: its
// announce stream ends, the peer drops the session, and the next one
// starts from the current set. The caller holds mu.
func (l *liveSet) notify(a moq.Announcement) {
	for ch, prefix := range l.listeners {
		if !strings.HasPrefix(a.Path, prefix) {
			continue
		}
		select {
		case ch <- a:
		default:
			delete(l.listeners, ch)
			close(ch)
		}
	}
}

// announce implements moq.Announcer for the set: the live streams under
// prefix now, then changes, until ctx ends.
func (l *liveSet) announce(ctx context.Context, prefix string) <-chan moq.Announcement {
	l.mu.Lock()
	ch := make(chan moq.Announcement, len(l.seen)+64)
	for streamer := range l.seen {
		if strings.HasPrefix(streamer, prefix) {
			ch <- moq.Announcement{Path: streamer, Active: true}
		}
	}
	l.listeners[ch] = prefix
	l.mu.Unlock()
	go func() {
		<-ctx.Done()
		l.mu.Lock()
		defer l.mu.Unlock()
		if _, ok := l.listeners[ch]; ok {
			delete(l.listeners, ch)
			close(ch)
		}
	}()
	return ch
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

// openPush dials the peer as a publisher and holds the session until it
// ends; it always returns an error saying why.
func (r *WebsocketReplicator) openPush(ctx context.Context, peer string) error {
	moqURL, err := r.resolvePeer(ctx, peer)
	if err != nil {
		return err
	}
	sess, err := r.dialMoq(ctx, moqURL, r.pushPublisher)
	if err != nil {
		return err
	}
	defer sess.Close()
	log.Log(ctx, "moq push: connected to peer", "url", moqURL)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-sess.Done():
		return sess.Err()
	}
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

// dialMoq opens a session to a moqt:// URL, pinning the certificate its
// certhash parameter names when it has one; pub, if set, is what the
// session serves (a push), else it subscribes (a pull).
func (r *WebsocketReplicator) dialMoq(ctx context.Context, rawURL string, pub moq.Publisher) (*moq.Session, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse moq URL: %w", err)
	}
	opts := moq.DialOptions{Publisher: pub}
	if q := u.Query(); q.Has(moq.CertHashParam) {
		opts.TLS = moq.PinnedTLSConfig(q.Get(moq.CertHashParam))
		q.Del(moq.CertHashParam)
		u.RawQuery = q.Encode()
	}
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	sess, err := moq.Dial(dialCtx, u.String(), opts)
	if err != nil {
		spmetrics.ReplicationConnectErrorsTotal.Inc()
		return nil, fmt.Errorf("could not dial moq (%s): %w", rawURL, err)
	}
	return sess, nil
}

// acceptPush is the receiving end of push: a peer has dialled in as a
// publisher, and its announcements say which streams it is ingesting. Each
// one is subscribed to and fed through the same validation as a pull;
// anything that fails validation ends the session, the wire carrying MUXL
// and nothing else. While a pushed stream's segments validate this node
// also puts itself in the station's broadcast_origins for it, so peers
// sharing the statedb pull it from here: the pusher is typically behind
// NAT, and its own origin record names a listener nobody can reach. The
// row follows validated segments, never the announcement alone: naming a
// DID proves nothing, a signed segment does.
func (r *WebsocketReplicator) acceptPush(ctx context.Context, sess *moq.Session) {
	ann, err := sess.Announced(ctx, "")
	if err != nil {
		log.Warn(ctx, "moq push: could not request announcements", "error", err)
		return
	}
	defer ann.Close()
	log.Log(ctx, "moq push: peer connected")
	g, gctx := errgroup.WithContext(ctx)
	running := map[string]context.CancelFunc{}
	for {
		a, err := ann.Next(gctx)
		if err != nil {
			if ctx.Err() == nil {
				log.Log(ctx, "moq push: peer session ended", "error", err)
			}
			break
		}
		stop, ok := running[a.Path]
		if !a.Active {
			if ok {
				stop()
				delete(running, a.Path)
			}
			continue
		}
		if ok {
			continue
		}
		if err := r.cli.StreamIsAllowed(a.Path); err != nil {
			log.Warn(ctx, "moq push: refusing pushed stream", "streamer", a.Path, "error", err)
			continue
		}
		bctx, cancel := context.WithCancel(gctx)
		running[a.Path] = cancel
		streamer := a.Path
		g.Go(func() error {
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
			err := r.consume(bctx, sess, streamer, r.originAdvertiser(bctx, streamer))
			if bctx.Err() != nil {
				// Retracted by the peer, or the session is over.
				return nil
			}
			return err
		})
	}
	for _, stop := range running {
		stop()
	}
	if err := g.Wait(); err != nil {
		log.Warn(ctx, "moq push: pushed stream failed, dropping the session", "error", err)
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

// consume subscribes to one streamer's source and rendition tracks on a
// session and feeds them through validation until the session or ctx ends;
// it always returns an error saying why. onSource, if set, runs after each
// source segment validates.
func (r *WebsocketReplicator) consume(ctx context.Context, sess *moq.Session, streamer string, onSource func()) error {
	source, err := sess.Subscribe(ctx, streamer, MoqSourceTrack)
	if err != nil {
		spmetrics.ReplicationConnectErrorsTotal.Inc()
		return fmt.Errorf("could not open source subscription: %w", err)
	}
	defer source.Close()
	renditions, err := sess.Subscribe(ctx, streamer, media.RenditionsChannel)
	if err != nil {
		spmetrics.ReplicationConnectErrorsTotal.Inc()
		return fmt.Errorf("could not open renditions subscription: %w", err)
	}
	defer renditions.Close()
	spmetrics.ReplicationOutboundOpen.WithLabelValues(streamer).Inc()
	defer spmetrics.ReplicationOutboundOpen.WithLabelValues(streamer).Dec()
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		for {
			f, err := source.Next(gctx)
			if err != nil {
				return fmt.Errorf("source track: %w", err)
			}
			if err := r.mm.ValidateMP4(context.Background(), bytes.NewReader(f.Payload), false); err != nil {
				return fmt.Errorf("could not validate segment: %w", err)
			}
			if onSource != nil {
				onSource()
			}
			if err := r.mm.ValidateMP4(context.Background(), bytes.NewReader(f.Payload), false); err != nil {
				return fmt.Errorf("could not validate segment: %w", err)
			}
		}
	})
	g.Go(func() error {
		for {
			f, err := renditions.Next(gctx)
			if err != nil {
				return fmt.Errorf("renditions track: %w", err)
			}
			if err := r.feedRenditions(gctx, streamer, f.Payload); err != nil {
				return err
			}
		}
	})
	return g.Wait()
}
