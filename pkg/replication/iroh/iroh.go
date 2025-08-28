package iroh

import (
	"context"
	"time"

	"stream.place/streamplace/pkg/log"

	irohStreamplace "stream.place/streamplace/pkg/iroh/generated/iroh_streamplace"
)

// IrohReplicator implements the replication mechanism using iroh
type IrohReplicator struct {
	topic  string
	sender *irohStreamplace.Sender
}

func NewIrohReplicator(ctx context.Context, ep *irohStreamplace.Endpoint, topic string, anchorNodes []*irohStreamplace.PublicKey) (*IrohReplicator, error) {
	sender, rustErr := irohStreamplace.NewSender(ep, anchorNodes)
	if rustErr.AsError() != nil {
		return nil, rustErr.AsError()
	}

	go func() {
		for {
			peers, rustErr := sender.Peers()
			if rustErr.AsError() != nil {
				log.Error(ctx, "error getting iroh peers", "error", rustErr.AsError())
			} else {
				allPeers := make([]string, len(peers))
				for i, peer := range peers {
					allPeers[i] = peer.NodeAddr().NodeId().String()
				}
				log.Warn(ctx, "iroh peers", "peers", allPeers)
			}
			time.Sleep(time.Second * 5)
		}
	}()

	return &IrohReplicator{
		topic:  topic,
		sender: sender,
	}, nil
}

func (rep *IrohReplicator) NewSegment(ctx context.Context, bs []byte) {
	log.Log(ctx, "replicating segment", "topic", rep.topic)
	go func(topic string) {
		err := sendSegment(rep.sender, topic, bs)
		if err != nil {
			log.Log(ctx, "error replicating segment", "error", err)
		} else {
			log.Log(ctx, "replicated segment", "topic", topic)
		}
	}(rep.topic)
}

func sendSegment(endpoint *irohStreamplace.Sender, topic string, bs []byte) error {
	return endpoint.Send(topic, bs).AsError()
}
