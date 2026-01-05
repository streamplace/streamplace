package media

import (
	"context"
	"io"
	"math/rand"
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
	require.Equal(t, 60, len(packet.Video))
	require.Equal(t, 50, len(packet.Audio))
	require.Equal(t, time.Duration(800*time.Millisecond), packet.Duration)
}

func TestPacketizeInvalid(t *testing.T) {
	// cur := goleak.IgnoreCurrent()
	// defer goleak.VerifyNone(t, cur)
	withNoGSTLeaks(t, func() {
		rng := rand.New(rand.NewSource(42))
		randomData := make([]byte, 1024*1024) // 1MB
		_, err := rng.Read(randomData)
		require.NoError(t, err)
		packet, err := Packetize(context.Background(), &bus.Seg{
			Data: randomData,
		})
		require.Error(t, err)
		require.Nil(t, packet)
	})
}
