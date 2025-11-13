package media

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestPacketizeDeterministic(t *testing.T) {
	// filename := "/Users/iameli/testvids/hour-of-silksong/04/15/2025-09-28T04-15-01-789Z.mp4"
	dirPath := remote.RemoteArchive("14ba49843a56c0510e2b5059123abd2f98a502b1f4c7d706b0ae1066d438468c/BigBuckBunny_1sGOP_4kp60_NoBframes.1min.tar.gz")
	dir, err := os.Open(dirPath)
	require.NoError(t, err)
	defer dir.Close()

	files, err := dir.Readdirnames(-1)
	require.NoError(t, err)

	var mp4Filenames []string
	for _, f := range files {
		if strings.HasSuffix(f, ".mp4") {
			mp4Filenames = append(mp4Filenames, filepath.Join(dirPath, f))
		}
	}
	require.NotEmpty(t, mp4Filenames, "no .mp4 files found in directory")
	withNoGSTLeaks(t, func() {
		g, ctx := errgroup.WithContext(context.Background())
		for _, filename := range mp4Filenames {
			g.Go(func() error {
				hashMu := sync.Mutex{}
				hashes := []string{}
				g2, _ := errgroup.WithContext(ctx)
				for range 2 {
					g2.Go(func() error {
						hash := innerTestPacketizeDeterminstic(t, filename)
						hashMu.Lock()
						defer hashMu.Unlock()
						hashes = append(hashes, hash)
						return nil
					})
				}
				err := g2.Wait()
				require.NoError(t, err)
				require.NotEmpty(t, hashes)
				for _, hash := range hashes {
					require.Equal(t, hashes[0], hash, "packetize output is not deterministic")
				}
				return nil
			})
		}
		err = g.Wait()
		require.NoError(t, err)
	})
}

func innerTestPacketizeDeterminstic(t *testing.T, filename string) string {
	// filename := getFixture("sample-segment.mp4")
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

	hash := sha256.New()
	for _, packet := range packet.Combined {
		hash.Write([]byte(packet.MediaType))
		hash.Write(packet.Data)
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}
