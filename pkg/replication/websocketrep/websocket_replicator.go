package websocketrep

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/streamplace"
)

type WebsocketReplicator struct {
	bus        *bus.Bus
	cli        *config.CLI
	mod        model.Model
	conns      map[string]bool
	connsMutex sync.RWMutex
	group      *errgroup.Group
	mm         *media.MediaManager
}

func NewWebsocketReplicator(bus *bus.Bus, mod model.Model, mm *media.MediaManager) *WebsocketReplicator {
	return &WebsocketReplicator{
		bus:        bus,
		mod:        mod,
		conns:      make(map[string]bool),
		connsMutex: sync.RWMutex{},
		mm:         mm,
	}
}

func (r *WebsocketReplicator) Start(ctx context.Context, cli *config.CLI) error {
	r.cli = cli
	_ = r.getMyWebsocketURL() // panic check
	r.group, ctx = errgroup.WithContext(ctx)
	return r.startBusSubscribe(ctx)
}

func (r *WebsocketReplicator) startBusSubscribe(ctx context.Context) error {
	// start subscription first so we're buffering new origins
	busCh := r.bus.Subscribe("")
	originViews, err := r.mod.GetRecentBroadcastOrigins(ctx)
	if err != nil {
		return fmt.Errorf("failed to get recent broadcast origins: %w", err)
	}
	for _, view := range originViews {
		err = r.handleOriginMessage(ctx, view)
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
			if view, ok := msg.(*streamplace.BroadcastDefs_BroadcastOriginView); ok {
				log.Debug(ctx, "got broadcast origin view", "view", view)
				err = r.handleOriginMessage(ctx, view)
				if err != nil {
					log.Error(ctx, "could not handle origin message", "error", err)
				}
			}
		}
	}
}

func (r *WebsocketReplicator) handleOriginMessage(ctx context.Context, view *streamplace.BroadcastDefs_BroadcastOriginView) error {
	origin, ok := view.Record.Val.(*streamplace.BroadcastOrigin)
	if !ok {
		return fmt.Errorf("record is not a BroadcastOrigin")
	}
	ctx = log.WithLogValues(ctx, "streamer", view.Author.Did)
	if origin.WebsocketURL == nil {
		return fmt.Errorf("origin has no websocket URL author=%s", view.Author.Did)
	}
	if !r.cli.ShouldSyndicate(origin.Streamer) {
		log.Debug(ctx, "not replicating streamer", "streamer", origin.Streamer)
		return nil
	}
	if r.hasConnection(origin.Streamer) {
		log.Debug(ctx, "already has connection")
		return nil
	}
	myURL := r.getMyWebsocketURL()
	u, err := url.Parse(*origin.WebsocketURL)
	if err != nil {
		return fmt.Errorf("could not parse origin websocket URL: %w", err)
	}
	if u.Host == myURL.Host {
		log.Debug(ctx, "origin websocket URL is on this node, skipping")
		return nil
	}
	r.group.Go(func() error {
		err := r.openWebsocket(ctx, view)
		log.Error(ctx, "websocket connection error", "error", err)
		return nil
	})
	return nil
}

func (r *WebsocketReplicator) openWebsocket(ctx context.Context, view *streamplace.BroadcastDefs_BroadcastOriginView) error {
	err := r.tryConnection(view.Author.Did)
	if err != nil {
		return err
	}
	defer r.removeConnection(view.Author.Did)
	origin, ok := view.Record.Val.(*streamplace.BroadcastOrigin)
	if !ok {
		return fmt.Errorf("record is not a BroadcastOrigin")
	}
	if origin.WebsocketURL == nil {
		return fmt.Errorf("origin has no websocket URL")
	}
	conn, _, err := websocket.DefaultDialer.Dial(*origin.WebsocketURL, nil)
	if err != nil {
		return fmt.Errorf("could not dial websocket: %w", err)
	}
	defer conn.Close()
	for {
		typ, msg, err := conn.ReadMessage()
		if err != nil {
			log.Error(ctx, "could not read message", "error", err)
			return fmt.Errorf("could not read message: %w", err)
		}
		if typ != websocket.BinaryMessage {
			log.Error(ctx, "expected binary message", "type", typ)
			return fmt.Errorf("expected binary message")
		}
		log.Debug(ctx, "received message", "type", typ, "length", len(msg))
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

func (r *WebsocketReplicator) BuildOriginRecord(origin *streamplace.BroadcastOrigin) error {
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
