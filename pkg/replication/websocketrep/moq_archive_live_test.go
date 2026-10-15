package websocketrep

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/moq"
)

func TestArchiveFirstDoesNotSuppressLiveSource(t *testing.T) {
	did, payload := signedMoqArchiveSource(t, 3600)
	r := archiveTestReplicator(t.TempDir())
	defer closeArchiveTestReplicator(r)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	meta, err := r.mm.ValidateArchivedMP4(ctx, bytes.NewReader(payload), did)
	require.NoError(t, err)
	require.NoError(t, r.storeMoqArchive(ctx, did, moq.SourceTrack, payload, meta))
	broadcast := r.moqBroadcasts[did]
	consumer, err := broadcast.public.Consume()
	require.NoError(t, err)
	live, err := consumer.SubscribeTrack(ctx, moq.SourceTrack, nil)
	require.NoError(t, err)
	defer live.Cancel()
	require.NoError(t, r.PublishMoqSource(ctx, did, payload, meta, true))
	group, err := live.RecvGroup(ctx)
	require.NoError(t, err)
	require.NotNil(t, group)
	frame, err := readMoqGroup(ctx, group)
	require.NoError(t, err)
	require.Equal(t, payload, frame.Payload)
	require.NotNil(t, broadcast.catalog.SourceSequence)
	require.Equal(t, uint64(0), *broadcast.catalog.SourceSequence)
	require.False(t, broadcast.lastPublic.IsZero())
}
