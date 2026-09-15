package websocketrep

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	glex "github.com/streamplace/glex/runtime"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/muxl"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spmetrics"
	"stream.place/streamplace/pkg/statedb"
)

type WebsocketReplicator struct {
	bus        *bus.Bus
	cli        *config.CLI
	mod        model.Model
	conns      map[string]bool
	connsMutex sync.RWMutex
	// state is the station's shared statedb, whose broadcast_origins table
	// is the second (and, with a shared database, the dependable) way to
	// learn about origins; nil in tests.
	state *statedb.StatefulDB
	// latest is the newest origin heard per streamer (see latestOrigin).
	latest      map[string]latestOrigin
	latestMutex sync.RWMutex
	group       *errgroup.Group
	mm          *media.MediaManager
}

func NewWebsocketReplicator(bus *bus.Bus, mod model.Model, mm *media.MediaManager, state *statedb.StatefulDB) *WebsocketReplicator {
	return &WebsocketReplicator{
		bus:        bus,
		mod:        mod,
		state:      state,
		conns:      make(map[string]bool),
		connsMutex: sync.RWMutex{},
		latest:     make(map[string]latestOrigin),
		mm:         mm,
	}
}

// latestOrigin is the newest broadcast origin heard for a streamer and when
// it was heard. The ingest node re-publishes its origin every 30s or so
// while segments flow, so an origin nobody has refreshed for originStaleAfter
// belongs to a stream that ended (or an ingest node that died) and is not
// worth dialling any more.
type latestOrigin struct {
	view *placestream.BroadcastDefs_BroadcastOriginView
	seen time.Time
}

var (
	pullBackoffMin = time.Second
	pullBackoffMax = 30 * time.Second
	// A pull whose origin nobody has refreshed for originQuietAfter is
	// probably a stream that ended; it is still retried (an ingest node
	// that comes back is picked up without anyone doing anything), just
	// at pullBackoffQuiet rather than pullBackoffMax.
	originQuietAfter = 10 * time.Minute
	pullBackoffQuiet = 5 * time.Minute
	// A pull that lasted this long counts as having worked: the next failure
	// starts the backoff over rather than continuing where it left off.
	pullHealthyAfter = 30 * time.Second
	// How often the shared broadcast_origins table is read, and how far
	// back a row still counts as an origin worth pulling from.
	originPollInterval = 10 * time.Second
	originPollWindow   = 5 * time.Minute
)

func (r *WebsocketReplicator) rememberOrigin(view *placestream.BroadcastDefs_BroadcastOriginView, streamer string) {
	r.latestMutex.Lock()
	defer r.latestMutex.Unlock()
	r.latest[streamer] = latestOrigin{view: view, seen: time.Now()}
}

func (r *WebsocketReplicator) latestFor(streamer string) (latestOrigin, bool) {
	r.latestMutex.RLock()
	defer r.latestMutex.RUnlock()
	l, ok := r.latest[streamer]
	return l, ok
}

func (r *WebsocketReplicator) Start(ctx context.Context, cli *config.CLI) error {
	r.cli = cli
	_ = r.getMyWebsocketURL() // panic check
	r.group, ctx = errgroup.WithContext(ctx)
	if r.state != nil {
		r.group.Go(func() error {
			r.pollOrigins(ctx)
			return nil
		})
	}
	return r.startBusSubscribe(ctx)
}

// pollOrigins learns about origins from the station's shared statedb rather
// than the firehose: the ingest node upserts (streamer, its server DID) on
// every local segment, so the table says who is ingesting whom right now
// regardless of which relays this node follows. (The origin record on the
// firehose lives in the streamer's own repo, which a peer's firehose never
// carries — a station whose nodes relay only each other would otherwise
// never hear of any origin.)
func (r *WebsocketReplicator) pollOrigins(ctx context.Context) {
	ticker := time.NewTicker(originPollInterval)
	defer ticker.Stop()
	for {
		rows, err := r.state.ListBroadcastOriginsSince(time.Now().Add(-originPollWindow))
		if err != nil {
			log.Error(ctx, "could not list broadcast origins", "error", err)
		}
		seen := map[string]bool{}
		for _, row := range rows {
			if seen[row.StreamerRepoDID] {
				continue // newest row per streamer wins
			}
			seen[row.StreamerRepoDID] = true
			view := r.originViewForRow(row)
			if err := r.handleOriginMessage(ctx, view); err != nil {
				log.Error(ctx, "could not handle broadcast origin row", "streamer", row.StreamerRepoDID, "server", row.ServerDID, "error", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// originViewForRow shapes a statedb row like the firehose's origin view, with
// the websocket URL a node at that server DID advertises (same scheme rule
// as our own, see getMyWebsocketURL — the station's nodes share a config).
func (r *WebsocketReplicator) originViewForRow(row statedb.BroadcastOrigin) *placestream.BroadcastDefs_BroadcastOriginView {
	u := url.URL{Scheme: "ws", Host: strings.TrimPrefix(row.ServerDID, "did:web:"), Path: "/xrpc/place.stream.live.subscribeSegments"}
	if r.cli.HasHTTPS() {
		u.Scheme = "wss"
	}
	u.RawQuery = url.Values{"streamer": []string{row.StreamerRepoDID}}.Encode()
	wsURL := u.String()
	return &placestream.BroadcastDefs_BroadcastOriginView{
		Author: appbsky.ActorDefs_ProfileViewBasic{Did: row.StreamerRepoDID},
		Record: &glex.LexiconTypeDecoder{Val: &placestream.BroadcastOrigin{
			Streamer:     row.StreamerRepoDID,
			Server:       row.ServerDID,
			WebsocketURL: &wsURL,
			UpdatedAt:    row.UpdatedAt.UTC().Format(time.RFC3339),
		}},
	}
}

func (r *WebsocketReplicator) startBusSubscribe(ctx context.Context) error {
	// start subscription first so we're buffering new origins
	busCh := r.bus.Subscribe("")
	originViews, err := r.mod.GetRecentBroadcastOrigins(ctx)
	if err != nil {
		return fmt.Errorf("failed to get recent broadcast origins: %w", err)
	}
	for _, view := range originViews {
		err = r.handleOriginMessage(ctx, &view)
		if err != nil {
			log.Error(ctx, "could not check origin", "error", err)
		}
	}
	log.Log(ctx, "Resumed recent broadcast origins", "count", len(originViews))
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-busCh:
			if view, ok := msg.(*placestream.BroadcastDefs_BroadcastOriginView); ok {
				log.Debug(ctx, "got broadcast origin view", "view", view)
				err = r.handleOriginMessage(ctx, view)
				if err != nil {
					log.Error(ctx, "could not handle origin message", "error", err)
				}
			}
		}
	}
}

func (r *WebsocketReplicator) handleOriginMessage(ctx context.Context, view *placestream.BroadcastDefs_BroadcastOriginView) error {
	origin, ok := view.Record.Val.(*placestream.BroadcastOrigin)
	if !ok {
		return fmt.Errorf("record is not a BroadcastOrigin")
	}
	ctx = log.WithLogValues(ctx, "streamer", view.Author.Did)
	if origin.WebsocketURL == nil {
		spmetrics.BroadcastOriginsTotal.WithLabelValues("no_url").Inc()
		return fmt.Errorf("origin has no websocket URL author=%s", view.Author.Did)
	}
	if !r.cli.ShouldSyndicate(origin.Streamer) {
		spmetrics.BroadcastOriginsTotal.WithLabelValues("not_syndicated").Inc()
		log.Debug(ctx, "not replicating streamer", "streamer", origin.Streamer)
		return nil
	}
	myURL := r.getMyWebsocketURL()
	u, err := url.Parse(*origin.WebsocketURL)
	if err != nil {
		spmetrics.BroadcastOriginsTotal.WithLabelValues("no_url").Inc()
		return fmt.Errorf("could not parse origin websocket URL: %w", err)
	}
	if u.Host == myURL.Host {
		spmetrics.BroadcastOriginsTotal.WithLabelValues("self").Inc()
		log.Debug(ctx, "origin websocket URL is on this node, skipping")
		return nil
	}
	// Remembered before the connected check: a running pull re-reads the
	// newest origin on every attempt (the stream may have moved nodes) and
	// each refresh keeps it from concluding the stream is over.
	r.rememberOrigin(view, origin.Streamer)
	if r.hasConnection(origin.Streamer) {
		spmetrics.BroadcastOriginsTotal.WithLabelValues("already_connected").Inc()
		log.Debug(ctx, "already has connection")
		return nil
	}
	spmetrics.BroadcastOriginsTotal.WithLabelValues("connect").Inc()
	log.Log(ctx, "syndicating: pulling from origin", "streamer", origin.Streamer, "origin", *origin.WebsocketURL)
	r.group.Go(func() error {
		r.pull(ctx, origin.Streamer)
		return nil
	})
	return nil
}

// pull keeps one streamer's segments flowing from wherever its origin is:
// dial, read until the connection drops, back off, dial again, for as long
// as the node runs. It never gives up: a stream that is merely taking a
// while to come back is indistinguishable from one that ended, and a
// periodic dial for a stream that ought to be live costs nothing. Each
// attempt re-reads the newest origin for the streamer, so a stream that
// moves to another ingest node is followed. Before this a failed dial was
// logged once and forgotten.
func (r *WebsocketReplicator) pull(ctx context.Context, streamer string) {
	if err := r.tryConnection(streamer); err != nil {
		return
	}
	defer r.removeConnection(streamer)
	ctx = log.WithLogValues(ctx, "streamer", streamer)
	backoff := pullBackoffMin
	attempts := 0
	for {
		latest, ok := r.latestFor(streamer)
		if !ok {
			return
		}
		attempts++
		started := time.Now()
		err := r.openWebsocket(ctx, latest.view)
		if ctx.Err() != nil {
			return
		}
		if time.Since(started) > pullHealthyAfter {
			backoff = pullBackoffMin
		}
		cap := pullBackoffMax
		if time.Since(latest.seen) > originQuietAfter {
			cap = pullBackoffQuiet
		}
		if backoff > cap {
			backoff = cap
		}
		// Every attempt while it's fresh; once it's a slow ping, a line now
		// and then so the log says the stream is still being watched.
		if backoff < cap || attempts%10 == 0 {
			log.Warn(ctx, "syndication: pull from origin ended, retrying", "error", err, "attempt", attempts, "retry_in", backoff, "origin_age", time.Since(latest.seen).Round(time.Second))
		} else {
			log.Debug(ctx, "syndication: pull from origin ended, retrying", "error", err, "attempt", attempts, "retry_in", backoff)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
	}
}

// openWebsocket dials the origin and feeds its segments through validation
// until the connection ends; it always returns an error saying why.
func (r *WebsocketReplicator) openWebsocket(ctx context.Context, view *placestream.BroadcastDefs_BroadcastOriginView) error {
	origin, ok := view.Record.Val.(*placestream.BroadcastOrigin)
	if !ok {
		return fmt.Errorf("record is not a BroadcastOrigin")
	}
	if origin.WebsocketURL == nil {
		return fmt.Errorf("origin has no websocket URL")
	}
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	conn, _, err := websocket.DefaultDialer.DialContext(dialCtx, *origin.WebsocketURL, nil)
	cancel()
	if err != nil {
		spmetrics.ReplicationConnectErrorsTotal.Inc()
		return fmt.Errorf("could not dial websocket (%s): %w", *origin.WebsocketURL, err)
	}
	defer conn.Close()
	// Drop the connection when the pull is cancelled, so ReadMessage returns.
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	log.Log(ctx, "syndication: connected to origin", "origin", *origin.WebsocketURL)
	spmetrics.ReplicationOutboundOpen.WithLabelValues(origin.Streamer).Inc()
	defer spmetrics.ReplicationOutboundOpen.WithLabelValues(origin.Streamer).Dec()
	for {
		typ, msg, err := conn.ReadMessage()
		if err != nil {
			spmetrics.ReplicationConnectErrorsTotal.Inc()
			return fmt.Errorf("could not read message: %w", err)
		}
		if typ != websocket.BinaryMessage {
			log.Error(ctx, "expected binary message", "type", typ)
			return fmt.Errorf("expected binary message")
		}
		log.Debug(ctx, "received message", "type", typ, "length", len(msg))
		if addendum, ok := media.UnframeRenditions(msg); ok {
			// Rendition tracks the origin minted for this stream: checked
			// (every track is a signed transcoded asset) and folded into
			// our live window as HLS variants. Not a segment, so not
			// validated or archived as one.
			if _, err := muxl.RunMuxlVerify(context.Background(), bytes.NewReader(addendum)); err != nil {
				log.Warn(ctx, "syndication: rendition addendum failed verification, dropped", "error", err)
				continue
			}
			r.mm.FeedLiveRenditions(context.WithoutCancel(ctx), origin.Streamer, addendum, true)
			continue
		}
		err = r.mm.ValidateMP4(context.Background(), bytes.NewReader(msg), false)
		if err != nil {
			return fmt.Errorf("could not validate segment: %w", err)
		}
	}
}

func (r *WebsocketReplicator) hasConnection(origin string) bool {
	r.connsMutex.RLock()
	defer r.connsMutex.RUnlock()
	return r.conns[origin]
}

func (r *WebsocketReplicator) tryConnection(origin string) error {
	r.connsMutex.Lock()
	defer r.connsMutex.Unlock()
	if _, ok := r.conns[origin]; ok {
		return fmt.Errorf("connection already exists")
	}
	r.conns[origin] = true
	return nil
}

func (r *WebsocketReplicator) removeConnection(origin string) {
	r.connsMutex.Lock()
	defer r.connsMutex.Unlock()
	delete(r.conns, origin)
}

// we're pull-based, nothing to do here
func (r *WebsocketReplicator) SendSegment(ctx context.Context, seg *media.NewSegmentNotification) error {
	return nil
}

func (r *WebsocketReplicator) BuildOriginRecord(origin *placestream.BroadcastOrigin) error {
	u := r.getMyWebsocketURL()
	u.Path = "/xrpc/place.stream.live.subscribeSegments"
	u.RawQuery = url.Values{
		"streamer": []string{origin.Streamer},
	}.Encode()

	urlStr := u.String()
	origin.WebsocketURL = &urlStr
	return nil
}

func (r *WebsocketReplicator) getMyWebsocketURL() *url.URL {
	if r.cli.WebsocketURL != "" {
		u, err := url.Parse(r.cli.WebsocketURL)
		// chill to panic, we're going to check this on boot
		if err != nil {
			panic("invalid websocket override URL: " + r.cli.WebsocketURL)
		}
		return u
	}
	u := url.URL{
		Scheme: "ws",
		Host:   r.cli.ServerHost,
	}
	if r.cli.HasHTTPS() {
		u.Scheme = "wss"
	}
	return &u
}
