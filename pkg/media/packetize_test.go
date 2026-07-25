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
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/test/remote"
)

func TestPacketize(t *testing.T) {
	withNoGSTLeaks(t, func() {
		g, _ := errgroup.WithContext(context.Background())
		for range streamplaceTestCount {
			g.Go(func() error {
				innerTestPacketize(t, getFixture("sample-segment.mp4"), 49, 40, time.Duration(800*time.Millisecond))
				return nil
			})
		}
		err := g.Wait()
		require.NoError(t, err)
	})
}

func TestPacketizeMuxl(t *testing.T) {
	t.Run("BasicMuxl", func(t *testing.T) {
		withNoGSTLeaks(t, func() {
			filename := remote.RemoteFixture("c6b57a53fc5a2234dbdd388922f0e293d8063d2b30620321e974b7c85640f228/2026-03-17T19-02-08-607Z-muxl_segment_input.fmp4")
			innerTestPacketize(t, filename, 60, 50, time.Duration(1000*time.Millisecond))
		})
	})
	t.Run("ThreeSecondSeg", func(t *testing.T) {
		withNoGSTLeaks(t, func() {
			filename2 := remote.RemoteFixture("5e2bca8cd42ad624d505c73f7b54ec761639416ef556d83145d37b730ed3606a/2026-04-11T22-24-11-527Z-packetize-input-019d7ea5-3d07-7606-b088-1a7bc315d009.mp4")
			innerTestPacketize(t, filename2, 180, 150, time.Duration(3000*time.Millisecond))
		})
	})
	t.Run("TenSecondSeg", func(t *testing.T) {
		withNoGSTLeaks(t, func() {
			filename3 := remote.RemoteFixture("82d20ee62b02f1c3a727b3001f1fa939afb757f9f205fa438d7b5753e1253eef/2026-04-11T22-39-41-861Z-packetize-input-019d7eb3-6f24-776c-ba1b-2f909a2379d7.mp4")
			innerTestPacketize(t, filename3, 300, 502, time.Duration(10040*time.Millisecond))
		})
	})
}

func innerTestPacketize(t *testing.T, filename string, expectedVideo int, expectedAudio int, expectedDuration time.Duration) {
	inputFile, err := os.Open(filename)
	require.NoError(t, err)
	defer inputFile.Close()

	bs, err := io.ReadAll(inputFile)
	require.NoError(t, err)

	testSeg := &bus.Seg{
		Data:     bs,
		Filepath: filename,
	}

	packet, err := Packetize(context.Background(), &config.CLI{}, testSeg)
	require.NoError(t, err)
	require.NotNil(t, packet)
	require.Equal(t, expectedVideo, len(packet.Video))
	require.Equal(t, expectedAudio, len(packet.Audio))
	require.Equal(t, expectedDuration, packet.Duration)
}

func TestPacketizeInvalid(t *testing.T) {
	// cur := goleak.IgnoreCurrent()
	// defer goleak.VerifyNone(t, cur)
	withNoGSTLeaks(t, func() {
		rng := rand.New(rand.NewSource(42))
		randomData := make([]byte, 1024*1024) // 1MB
		_, err := rng.Read(randomData)
		require.NoError(t, err)
		packet, err := Packetize(context.Background(), &config.CLI{}, &bus.Seg{
			Data: randomData,
		})
		require.Error(t, err)
		require.Nil(t, packet)
	})
}
