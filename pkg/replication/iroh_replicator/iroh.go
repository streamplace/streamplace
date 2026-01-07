package iroh_replicator

import (
	"context"
)

// IrohReplicator implements the replication mechanism using iroh
type IrohReplicator struct {
}

func NewIrohReplicator(ctx context.Context) (*IrohReplicator, error) {

	return &IrohReplicator{}, nil
}

func (rep *IrohReplicator) NewSegment(ctx context.Context, bs []byte) {

}
