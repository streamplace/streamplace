package media

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/bus"
)

func TestPacketize(t *testing.T) {
	withNoGSTLeaks(t, func() {
		g, _ := errgroup.WithContext(context.Background())
		for range streamplaceTestCount {
			g.Go(func() error {
				innerTestPacketize(t)
				return nil
			})
		}
		err := g.Wait()
		require.NoError(t, err)
	})
}

func innerTestPacketize(t *testing.T) {
	filename := getFixture("sample-segment.mp4")
	inputFile, err := os.Open(filename)
	require.NoError(t, err)
	defer inputFile.Close()

	bs, err := io.ReadAll(inputFile)
	require.NoError(t, err)

	testSeg := &bus.Seg{
		Data:     bs,
		Filepath: filename,
	}

	packet, err := Packetize(context.Background(), testSeg)
	require.NoError(t, err)
	require.NotNil(t, packet)
	require.Equal(t, 49, len(packet.Video))
	require.Equal(t, 40, len(packet.Audio))
	require.Equal(t, time.Duration(800*time.Millisecond), packet.Duration)
}
