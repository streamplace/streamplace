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
	"stream.place/streamplace/test/remote"
)

func TestPacketize(t *testing.T) {
	withNoGSTLeaks(t, func() {
		g, _ := errgroup.WithContext(context.Background())
		for range streamplaceTestCount {
			g.Go(func() error {
				innerTestPacketize(t, getFixture("sample-segment.mp4"))
				return nil
			})
		}
		err := g.Wait()
		require.NoError(t, err)
	})
}

func TestPacketizeMuxl(t *testing.T) {
	withNoGSTLeaks(t, func() {
		filename := remote.RemoteFixture("c6b57a53fc5a2234dbdd388922f0e293d8063d2b30620321e974b7c85640f228/2026-03-17T19-02-08-607Z-muxl_segment_input.fmp4")
		innerTestPacketize(t, filename)
	})
}

func innerTestPacketize(t *testing.T, filename string) {
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
