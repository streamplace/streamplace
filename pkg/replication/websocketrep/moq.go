package websocketrep

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
	"stream.place/streamplace/pkg/moq"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spmetrics"
)

// MoqSourceTrack carries the complete signed source, including required audio
// and metadata. It must not be cancelled as though it were video-only.
const MoqSourceTrack = moq.SourceTrack

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

// Each incoming connection gets a private receive origin. Unvalidated peer
// announcements must never become routes on our public publishing origin.
func (r *WebsocketReplicator) serveMoQ(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if r.MoqTLS == nil {
		return fmt.Errorf("moq listener has no TLS files")
	}
	if err := r.MoqTLS.Sync(); err != nil {
		return err
	}
	srv, err := official.Listen(ctx, r.cli.MoqAddr,
		official.WithTLSCert(r.MoqTLS.CertPath), official.WithTLSKey(r.MoqTLS.KeyPath),
		official.WithServerPublishOrigin(r.moqOrigin))
	if err != nil {
		return fmt.Errorf("listening for moq: %w", err)
	}
	defer srv.Close()
	g, gctx := errgroup.WithContext(ctx)
	defer func() { cancel(); _ = g.Wait() }()
	g.Go(func() error {
		r.MoqTLS.Refresh(gctx, func(err error) { log.Error(ctx, "refreshing moq TLS", "error", err) })
		return nil
	})
	log.Log(ctx, "moq server starting", "addr", srv.LocalAddr())
	for req, err := range srv.Requests(gctx) {
		if err != nil {
			return err
		}
		incoming := official.NewOriginProducer()
		if err := req.SetConsume(incoming); err != nil {
			_ = req.Reject(gctx, 500)
			continue
		}
		g.Go(func() error {
			acceptCtx, cancel := context.WithTimeout(gctx, 15*time.Second)
			sess, err := req.Accept(acceptCtx)
			cancel()
			if err != nil {
				log.Debug(gctx, "moq accept failed", "error", err)
				return nil
			}
			defer sess.Shutdown()
			sctx, stop := context.WithCancel(gctx)
			defer stop()
			closed := make(chan struct{})
			go func() { defer close(closed); _ = sess.Closed(sctx); stop() }()
			r.acceptPush(sctx, incoming.Consume())
			stop()
			<-closed
			return nil
		})
	}
	return nil
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
	client, err := r.dialMoq(ctx, moqURL, r.moqLocalOrigin)
	if err != nil {
		return err
	}
	defer client.Close()
	log.Log(ctx, "moq push: connected to peer", "url", moqURL)
	return client.Session().Closed(ctx)
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

// Go owns reconnects so each attempt refreshes discovery and certificate pins.
func (r *WebsocketReplicator) dialMoq(ctx context.Context, rawURL string, pub *official.OriginProducer) (*official.Client, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse moq URL: %w", err)
	}
	if pub == nil {
		pub = official.NewOriginProducer()
	}
	opts := []official.ClientOption{official.WithPublishOrigin(pub), official.WithSubscribeOrigin(official.NewOriginProducer()), official.WithReconnect(false), official.WithWebSocketEnabled(false)}
	if q := u.Query(); q.Has(moq.CertHashParam) {
		pin := q.Get(moq.CertHashParam)
		decoded, err := hex.DecodeString(pin)
		if err != nil || len(decoded) != 32 {
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
		return nil, fmt.Errorf("could not dial moq (%s): %w", rawURL, err)
	}
	return client, nil
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
const (
	moqPushSessionLimit       = 32
	moqPushGlobalLimit        = 256
	moqPushFirstSourceTimeout = 15 * time.Second
)

type moqPushAdmission struct {
	cancel context.CancelFunc
}

// Admission and release share the node-wide mutex. Retracting a stream cancels
// its worker but retains the reservation until that worker actually exits.
// Otherwise rapid announce/retract cycles could bypass the goroutine bound.
func (r *WebsocketReplicator) admitPush(running map[string]*moqPushAdmission, path string, cancel context.CancelFunc) (*moqPushAdmission, bool) {
	r.pushedMu.Lock()
	defer r.pushedMu.Unlock()
	if running[path] != nil || len(running) >= moqPushSessionLimit || r.pushAdmissions >= moqPushGlobalLimit {
		return nil, false
	}
	entry := &moqPushAdmission{cancel: cancel}
	running[path] = entry
	r.pushAdmissions++
	return entry, true
}

func (r *WebsocketReplicator) releasePush(running map[string]*moqPushAdmission, path string, entry *moqPushAdmission) {
	r.pushedMu.Lock()
	defer r.pushedMu.Unlock()
	if running[path] != entry {
		return
	}
	delete(running, path)
	r.pushAdmissions--
}

func (r *WebsocketReplicator) acceptPush(ctx context.Context, origin *official.OriginConsumer) {
	r.acceptPushWithDeadline(ctx, origin, moqPushFirstSourceTimeout)
}

func (r *WebsocketReplicator) acceptPushWithDeadline(ctx context.Context, origin *official.OriginConsumer, firstSourceTimeout time.Duration) {
	ann, err := origin.Announced(official.AnnounceOptions{})
	if err != nil {
		log.Warn(ctx, "moq push: could not request announcements", "error", err)
		return
	}
	defer ann.Cancel()
	log.Log(ctx, "moq push: peer connected")
	g, gctx := errgroup.WithContext(ctx)
	running := map[string]*moqPushAdmission{}
	for {
		a, err := ann.Next(gctx)
		if err != nil || a == nil {
			if ctx.Err() == nil {
				log.Log(ctx, "moq push: peer session ended", "error", err)
			}
			break
		}
		path := a.Prefix()
		r.pushedMu.Lock()
		entry := running[path]
		r.pushedMu.Unlock()
		if !a.Active() {
			if entry != nil {
				entry.cancel()
			}
			continue
		}
		if entry != nil {
			continue
		}
		if _, err := syntax.ParseDID(path); err != nil {
			continue
		}
		if err := r.cli.StreamIsAllowed(path); err != nil {
			log.Warn(ctx, "moq push: refusing pushed stream", "streamer", path, "error", err)
			continue
		}
		bctx, cancel := context.WithCancel(gctx)
		entry, admitted := r.admitPush(running, path, cancel)
		if !admitted {
			cancel()
			continue
		}
		streamer := path
		g.Go(func() error {
			defer cancel()
			defer r.releasePush(running, streamer, entry)
			firstSource := time.AfterFunc(firstSourceTimeout, cancel)
			defer firstSource.Stop()
			var validated sync.Once
			advertise := r.originAdvertiser(bctx, streamer)
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
			err := r.consume(bctx, origin, streamer, func() {
				validated.Do(func() { firstSource.Stop() })
				if advertise != nil {
					advertise()
				}
			})
			if bctx.Err() != nil {
				// Retracted by the peer, or the session is over.
				return nil
			}
			return err
		})
	}
	r.pushedMu.Lock()
	for _, entry := range running {
		entry.cancel()
	}
	r.pushedMu.Unlock()
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

// consume keeps the source (audio and metadata included) and independently
// catalogued rendition tracks live. Archive FETCH is deliberately separate.
func (r *WebsocketReplicator) consume(ctx context.Context, origin *official.OriginConsumer, streamer string, onSource func()) error {
	if r.cli.DisableSyndication {
		return fmt.Errorf("syndication disabled")
	}
	if err := r.cli.StreamIsAllowed(streamer); err != nil {
		return err
	}
	ann, err := origin.AnnouncedBroadcast(streamer)
	if err != nil {
		return err
	}
	defer ann.Cancel()
	bc, err := ann.Available(ctx)
	if err != nil {
		return err
	}
	catalog, err := bc.SubscribeCatalog(ctx)
	if err != nil {
		return err
	}
	defer catalog.Cancel()
	spmetrics.ReplicationOutboundOpen.WithLabelValues(streamer).Inc()
	defer spmetrics.ReplicationOutboundOpen.WithLabelValues(streamer).Dec()
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return r.consumeTrack(gctx, bc, streamer, MoqSourceTrack, onSource) })
	g.Go(func() error {
		seen := map[string]bool{}
		for {
			cat, err := catalog.Next(gctx)
			if err != nil {
				return err
			}
			if cat == nil {
				return io.EOF
			}
			section, ok, err := moq.ParseCatalog(cat)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			for _, rendition := range section.Renditions {
				name := rendition.Track
				id, valid := moq.ParseRenditionTrack(name)
				if !valid || id != rendition.ID {
					return fmt.Errorf("invalid rendition catalog track %q", name)
				}
				if seen[name] {
					continue
				}
				if len(seen) >= 32 {
					return fmt.Errorf("moq catalog exceeds 32 rendition tracks")
				}
				seen[name] = true
				g.Go(func() error { return r.consumeTrack(gctx, bc, streamer, name, nil) })
			}
		}
	})
	return g.Wait()
}

func (r *WebsocketReplicator) consumeTrack(ctx context.Context, bc *official.BroadcastConsumer, streamer, name string, onSource func()) error {
	priority := uint8(200)
	if name != MoqSourceTrack {
		priority = 150
	}
	track, err := bc.SubscribeTrack(ctx, name, &official.Subscription{Priority: priority, MaxAgeUs: 2_000_000})
	if err != nil {
		return err
	}
	defer track.Cancel()
	for {
		group, err := track.RecvGroup(ctx)
		if err != nil {
			return err
		}
		if group == nil {
			return io.EOF
		}
		sequence := group.Sequence()
		frame, err := readMoqGroup(ctx, group)
		if err != nil {
			return err
		}
		if name == MoqSourceTrack {
			if err := r.mm.ValidateReplicatedMP4(ctx, bytes.NewReader(frame.Payload), streamer); err != nil {
				return err
			}
			if onSource != nil {
				onSource()
			}
		} else {
			if err := validateMoqRenditionTrack(ctx, name, frame.Payload); err != nil {
				return err
			}
			// FETCH source only for cryptographic ingredient binding, never to
			// revive it as live media. Sequence identities are origin-local.
			source, err := fetchMoqFrame(ctx, bc, MoqSourceTrack, sequence)
			if err != nil {
				return err
			}
			if err := r.mm.ValidateReplicatedRenditions(ctx, streamer, source.Payload, frame.Payload); err != nil {
				return err
			}
			meta, err := r.mm.ValidateArchivedMP4(ctx, bytes.NewReader(source.Payload), streamer)
			if err != nil {
				return err
			}
			// A late completion belongs only in the archive, not the live window.
			if err := r.storeMoqArchive(ctx, streamer, MoqSourceTrack, source.Payload, meta); err != nil {
				return err
			}
			if time.Since(meta.StartTime.Time()) > 2*time.Second {
				if err := r.storeMoqArchive(ctx, streamer, name, frame.Payload, meta); err != nil {
					return err
				}
				continue
			}
			if err := r.publishMoqSegment(ctx, streamer, name, frame.Payload, uint64(meta.StartTime.Time().UnixMicro()), false, time.Time{}); err != nil {
				return err
			}
			r.mm.FeedLiveRenditions(ctx, streamer, frame.Payload, meta.StartTime.Time(), true)
			r.mm.PublishRenditionsForPlayback(ctx, streamer, frame.Payload, true)
		}
	}
}

func readMoqGroup(ctx context.Context, group *official.GroupConsumer) (*official.Frame, error) {
	defer group.Cancel()
	frame, err := group.ReadFrame(ctx)
	if err != nil {
		return nil, err
	}
	if frame == nil || !moq.IsMuxl(frame.Payload) {
		return nil, fmt.Errorf("moq group is not a signed MUXL segment")
	}
	extra, err := group.ReadFrame(ctx)
	if err != nil {
		return nil, err
	}
	if extra != nil {
		return nil, fmt.Errorf("moq group contains more than one segment")
	}
	return frame, nil
}
