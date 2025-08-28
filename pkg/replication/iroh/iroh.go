package iroh

import (
	"context"

	"stream.place/streamplace/pkg/log"

	irohStreamplace "stream.place/streamplace/pkg/iroh/generated/iroh_streamplace"
)

// IrohReplicator implements the replication mechanism using iroh
type IrohReplicator struct {
	topic  string
	sender *irohStreamplace.Sender
}

func NewIrohReplicator(ctx context.Context, ep *irohStreamplace.Endpoint, topic string) (*IrohReplicator, error) {
	sender, err := irohStreamplace.NewSender(ep)
	if err.AsError() != nil {
		return nil, err.AsError()
	}

	return &IrohReplicator{
		topic:  topic,
		sender: sender,
	}, nil
}

func (rep *IrohReplicator) NewSegment(ctx context.Context, bs []byte) {
	go func(topic string) {
		err := sendSegment(rep.sender, topic, bs)
		if err != nil {
			log.Log(ctx, "error replicating segment", "error", err)
		}
	}(rep.topic)
}

func sendSegment(endpoint *irohStreamplace.Sender, topic string, bs []byte) error {
	return endpoint.Send(topic, bs).AsError()
}
