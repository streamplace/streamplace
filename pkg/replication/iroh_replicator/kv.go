package iroh_replicator

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/bluesky-social/indigo/util"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/iroh/generated/iroh_streamplace"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/spmetrics"
	"stream.place/streamplace/pkg/streamplace"
)

type IrohSwarm struct {
	Node             *iroh_streamplace.Node
	DB               *iroh_streamplace.Db
	w                *iroh_streamplace.WriteScope
	mm               *media.MediaManager
	segChan          chan *media.NewSegmentNotification
	NodeID           string
	NodeTicket       string
	bus              *bus.Bus
	originMutex      sync.Mutex
	mod              model.Model
	cli              *config.CLI
	activeSubs       map[string]*SwarmOriginInfo
	handleDataScoped func(pubKey *iroh_streamplace.PublicKey, topic string, data []byte)
}

// A message saying "hey I ingested node data at this time"
type SwarmOriginInfo struct {
	Type     string `json:"$type"`
	NodeID   string `json:"node_id"`
	Time     string `json:"time"`
	Streamer string `json:"streamer"`
}

type SwarmViewerCount struct {
	Type     string `json:"$type"`
	Server   string `json:"server"`
	Streamer string `json:"streamer"`
	Viewers  int    `json:"viewers"`
}

func NewSwarm(ctx context.Context, cli *config.CLI, secret []byte, topic []byte, mm *media.MediaManager, bus *bus.Bus, mod model.Model) (*IrohSwarm, error) {
	ctx = log.WithLogValues(ctx, "func", "StartKV")

	if topic == nil {
		topic = make([]byte, 32)
		_, err := rand.Read(topic)
		if err != nil {
			return nil, fmt.Errorf("failed to generate random topic: %w", err)
		}
	}

	log.Log(ctx, "Starting with tickets", "tickets", cli.Tickets)
	config := iroh_streamplace.Config{
		Key:             secret,
		Topic:           topic,
		MaxSendDuration: 1000_000_000, // 1s
		DisableRelay:    cli.DisableIrohRelay,
	}
	log.Log(ctx, "Config created", "config", config)

	swarm := IrohSwarm{
		mm:         mm,
		activeSubs: make(map[string]*SwarmOriginInfo),
		bus:        bus,
		mod:        mod,
		cli:        cli,
	}

	// workaround to get context into the HandleData callback
	swarm.handleDataScoped = func(_ *iroh_streamplace.PublicKey, topic string, data []byte) {
		if ctx.Err() != nil {
			return
		}
		err := swarm.mm.ValidateMP4(context.Background(), bytes.NewReader(data), false)
		if err != nil {
			log.Error(ctx, "could not validate segment", "error", err, "topic", topic, "data", len(data))
		}
	}

	node, err := iroh_streamplace.NodeReceiver(config, &swarm)
	if err != nil {
		return nil, fmt.Errorf("failed to create NodeSender: %w", err)
	}

	db := node.Db()
	w := node.NodeScope()

	swarm.DB = db
	swarm.w = w
	swarm.Node = node

	nodeId, err := node.NodeId()
	if err != nil {
		return nil, fmt.Errorf("failed to get NodeId: %w", err)
	}
	log.Log(ctx, "Node ID:", "node_id", nodeId)
	swarm.NodeID = nodeId.String()

	ticket, err := node.Ticket()
	if err != nil {
		return nil, fmt.Errorf("failed to get Ticket: %w", err)
	}
	swarm.NodeTicket = ticket

	return &swarm, nil
}

func (swarm *IrohSwarm) BuildOriginRecord(origin *streamplace.BroadcastOrigin) error {
	origin.IrohTicket = &swarm.NodeTicket
	return nil
}

func (swarm *IrohSwarm) Start(ctx context.Context, cli *config.CLI) error {
	if len(cli.Tickets) > 0 {
		err := swarm.Node.JoinPeers(cli.Tickets)
		if err != nil {
			return fmt.Errorf("failed to join peers: %w", err)
		}
	}
	nodeId, err := swarm.Node.NodeId()
	if err != nil {
		return fmt.Errorf("failed to get node id: %w", err)
	}
	nodeIdStr := nodeId.String()
	log.Log(ctx, "Node ID:", "node_id", nodeIdStr)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return swarm.startKV(ctx)
	})
	g.Go(func() error {
		return swarm.startSegmentSender(ctx)
	})
	g.Go(func() error {
		<-ctx.Done()
		return swarm.Node.Shutdown()
	})
	g.Go(func() error {
		return swarm.startBusSubscribe(ctx)
	})
	g.Go(func() error {
		return swarm.startViewerCountSubscribe(ctx)
	})
	return g.Wait()
}

func (swarm *IrohSwarm) startKV(ctx context.Context) error {
	sub := swarm.DB.Subscribe(iroh_streamplace.NewFilter())
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		ev, err := sub.NextRaw()
		if err != nil {
			return fmt.Errorf("failed to get next subscription event: %w", err)
		}

		if ev == nil {
			log.Warn(ctx, "Got empty event from sub.NextRaw(), pausing for a second and continuing")
			time.Sleep(1 * time.Second)
			continue
		}

		switch item := (*ev).(type) {
		case iroh_streamplace.SubscribeItemEntry:
			err := swarm.handleIrohMessage(ctx, item)
			if err != nil {
				log.Error(ctx, "could not handle iroh message", "error", err)
				continue
			}

		case iroh_streamplace.SubscribeItemCurrentDone:
			log.Debug(ctx, "SubscribeItemCurrentDone", "currentDone", item)
		case iroh_streamplace.SubscribeItemExpired:
			log.Debug(ctx, "SubscribeItemExpired", "expired", item)
		case iroh_streamplace.SubscribeItemOther:
			log.Debug(ctx, "SubscribeItemOther", "other", item)
		}
	}
}

func (swarm *IrohSwarm) handleIrohMessage(ctx context.Context, item iroh_streamplace.SubscribeItemEntry) error {
	keyStr := string(item.Key)
	valueStr := string(item.Value)
	log.Debug(ctx, "SubscribeItemEntry", "key", keyStr, "value", valueStr)
	if len(valueStr) > 0 && valueStr[0] != '{' {
		// not JSON, it's one of the rust messages
		log.Debug(ctx, "not JSON", "key", keyStr, "value", valueStr)
		return nil
	}
	rawMessage, err := decodeIrohMessage(item.Key, item.Value)
	if err != nil {
		return fmt.Errorf("could not decode iroh message: %w", err)
	}
	switch message := rawMessage.(type) {
	case SwarmOriginInfo:
		err = swarm.checkOrigins(ctx, message.Streamer, message.NodeID)
		if err != nil {
			return fmt.Errorf("could not check origins: %w", err)
		}
	case SwarmViewerCount:
		log.Log(ctx, "got viewer count", "viewerCount", message)
		if message.Server == swarm.NodeID {
			// no infinite loops allowed
			return nil
		}
		swarm.bus.SetViewerCount(message.Streamer, message.Server, message.Viewers)
		log.Log(ctx, "set viewer count", "viewerCount", message)
		return nil
	default:
		return fmt.Errorf("unknown message type: %s", reflect.TypeOf(rawMessage))
	}
	return nil
}

func decodeIrohMessage(key, value []byte) (any, error) {
	keyStr := string(key)
	if strings.HasPrefix(keyStr, "origin::") {
		var originInfo SwarmOriginInfo
		err := json.Unmarshal(value, &originInfo)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal origin info: %w", err)
		}
		return originInfo, nil
	}
	if strings.HasPrefix(keyStr, "viewers::") {
		var viewerCount SwarmViewerCount
		err := json.Unmarshal(value, &viewerCount)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal viewer count: %w", err)
		}
		return viewerCount, nil
	}
	return nil, fmt.Errorf("unknown key: %s", keyStr)
}

// subscribe to all streams
func (swarm *IrohSwarm) startBusSubscribe(ctx context.Context) error {
	// start subscription first so we're buffering new origins
	busCh := swarm.bus.Subscribe("")
	originViews, err := swarm.mod.GetRecentBroadcastOrigins(ctx)
	if err != nil {
		return fmt.Errorf("failed to get recent broadcast origins: %w", err)
	}
	for _, view := range originViews {
		err = swarm.handleOriginMessage(ctx, view)
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
				err = swarm.handleOriginMessage(ctx, view)
				if err != nil {
					log.Error(ctx, "could not handle origin message", "error", err)
				}
			}
		}
	}
}

func (swarm *IrohSwarm) startViewerCountSubscribe(ctx context.Context) error {
	ch := swarm.bus.SubscribeToViewerCount()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-ch:
			log.Log(ctx, "got viewer count update", "viewerCount", msg)
			if msg.Origin != "local" {
				continue
			}
			swarmMsg := SwarmViewerCount{
				Type:     "place.stream.swarm.viewerCount",
				Server:   swarm.NodeID,
				Streamer: msg.Streamer,
				Viewers:  msg.Count,
			}
			bs, err := json.Marshal(swarmMsg)
			if err != nil {
				log.Error(ctx, "could not marshal viewer count", "error", err)
				continue
			}
			key := fmt.Sprintf("viewers::%s::%s", swarm.NodeID, msg.Streamer)
			err = swarm.w.Put(nil, []byte(key), bs)
			if err != nil {
				log.Error(ctx, "could not put viewer count to swarm", "error", err)
				continue
			}
			log.Log(ctx, "put viewer count to swarm", "viewerCount", msg)

		}
	}
}

func (swarm *IrohSwarm) handleOriginMessage(ctx context.Context, view *streamplace.BroadcastDefs_BroadcastOriginView) error {
	origin, ok := view.Record.Val.(*streamplace.BroadcastOrigin)
	if !ok {
		return fmt.Errorf("record is not a BroadcastOrigin")
	}
	if view.Author.Did != origin.Streamer {
		// currently, only streamers are allowed to advertise origins
		return nil
	}
	if origin.IrohTicket == nil {
		return fmt.Errorf("origin has no iroh ticket")
	}
	pubKey, err := iroh_streamplace.NodeIdFromTicket(*origin.IrohTicket)
	if err != nil {
		return fmt.Errorf("could not get node id from ticket: %w", err)
	}
	err = swarm.Node.AddTickets([]string{*origin.IrohTicket})
	if err != nil {
		return fmt.Errorf("could not add tickets: %w", err)
	}
	pubKeyStr := pubKey.String()
	err = swarm.checkOrigins(ctx, origin.Streamer, pubKeyStr)
	if err != nil {
		return fmt.Errorf("could not check origin: %w", err)
	}
	return nil
}

func (swarm *IrohSwarm) checkOrigins(ctx context.Context, streamer string, nodeID string) error {
	ctx = log.WithLogValues(ctx, "streamer", streamer, "nodeID", nodeID, "func", "checkOrigins")
	err := swarm.cli.StreamIsAllowed(streamer)
	if err != nil {
		return fmt.Errorf("user %s is not allowlisted on this node: %w", streamer, err)
	}
	swarm.originMutex.Lock()
	defer swarm.originMutex.Unlock()
	oldSub, ok := swarm.activeSubs[streamer]
	if ok {
		if oldSub.NodeID == nodeID {
			log.Debug(ctx, "node hasn't changed", "streamer", streamer)
			// mmyep. same node still has the stream. great news.
			return nil
		}
		log.Log(ctx, "Stream origin changed, swapping to new node", "old_node", oldSub.NodeID, "new_node", nodeID, "streamer", streamer)
		pubKey, err := iroh_streamplace.PublicKeyFromString(oldSub.NodeID)
		if err != nil {
			log.Error(ctx, "could not create public key", "error", err)
			return err
		}
		// different node has the stream. we need to unsubscribe from the old node.
		err = swarm.Node.Unsubscribe(streamer, pubKey)
		if err != nil {
			log.Error(ctx, "could not unsubscribe from key", "error", err)
			return err
		}
		delete(swarm.activeSubs, streamer)
	}
	if nodeID == swarm.NodeID {
		log.Debug(ctx, "I already have this stream", "streamer", streamer)
		// oh, i have this stream. cool. do nothing.
		return nil
	}
	log.Log(ctx, "Subscribing to stream start", "new_node", nodeID, "streamer", streamer)
	pubKey, err := iroh_streamplace.PublicKeyFromString(nodeID)
	if err != nil {
		log.Error(ctx, "could not create public key", "error", err)
		return err
	}
	err = swarm.Node.Subscribe(streamer, pubKey)
	log.Log(ctx, "Subscribing to stream done", "new_node", nodeID, "streamer", streamer, "pubKey", pubKey, "error", err)
	if err != nil {
		log.Error(ctx, "could not subscribe to key", "error", err)
		return err
	}
	swarm.activeSubs[streamer] = &SwarmOriginInfo{
		Type:     "place.stream.swarm.originInfo",
		NodeID:   nodeID,
		Time:     time.Now().Format(util.ISO8601),
		Streamer: streamer,
	}
	return nil
}

func (swarm *IrohSwarm) startSegmentSender(ctx context.Context) error {
	ch := swarm.mm.NewSegment()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case not := <-ch:
			err := swarm.SendSegment(ctx, not)
			if err != nil {
				log.Error(ctx, "could not send segment to swarm", "error", err)
			}
			continue
		}
	}
}

func (swarm *IrohSwarm) HandleData(pubKey *iroh_streamplace.PublicKey, topic string, data []byte) {
	swarm.handleDataScoped(pubKey, topic, data)
}

func (swarm *IrohSwarm) SendSegment(ctx context.Context, not *media.NewSegmentNotification) error {
	if !not.Local {
		return nil
	}
	originInfo := SwarmOriginInfo{
		Type:     "place.stream.swarm.originInfo",
		NodeID:   swarm.NodeID,
		Time:     not.Segment.StartTime.Format(util.ISO8601),
		Streamer: not.Segment.RepoDID,
	}
	bs, err := json.Marshal(originInfo)
	if err != nil {
		log.Error(ctx, "could not marshal origin info", "error", err)
		return err
	}
	keyBs := []byte(fmt.Sprintf("origin::%s", not.Segment.RepoDID))
	go func() {
		spmetrics.SwarmPutCalls.WithLabelValues(not.Segment.RepoDID).Inc()
		defer spmetrics.SwarmPutCalls.WithLabelValues(not.Segment.RepoDID).Dec()
		err = swarm.w.Put(nil, keyBs, bs)
		if err != nil {
			log.Error(ctx, "could not put segment to swarm", "error", err)
		}
	}()
	go func() {
		spmetrics.SendSegmentCalls.Inc()
		defer spmetrics.SendSegmentCalls.Dec()
		err = swarm.Node.SendSegment(not.Segment.RepoDID, not.Data)
		if err != nil {
			log.Error(ctx, "could not send segment to swarm", "error", err)
		}
	}()
	return nil
}
