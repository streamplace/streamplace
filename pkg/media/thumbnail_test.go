package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/test/remote"
)

func TestThumbnail(t *testing.T) {
	withNoGSTLeaks(t, func() {
		// Open input file
		inputFile, err := os.Open(getFixture("sample-segment.mp4"))
		require.NoError(t, err)
		defer inputFile.Close()
		bs, err := io.ReadAll(inputFile)
		require.NoError(t, err)

		ctx := context.Background()
		g, ctx := errgroup.WithContext(ctx)

		for i := 0; i < streamplaceTestCount; i++ {
			g.Go(func() error {
				thumbnail := bytes.Buffer{}
				// thumbnailCtx = log.WithDebugValue(ctx, map[string]map[string]int{"function": {"Thumbnail": 9}})
				err := Thumbnail(ctx, bytes.NewReader(bs), &thumbnail, "png")
				if err != nil {
					return err
				}
				if thumbnail.Len() == 0 {
					return fmt.Errorf("thumbnail buffer is empty")
				}
				require.Equal(t, 1418910, thumbnail.Len())
				return nil
			})
			g.Go(func() error {
				thumbnail := bytes.Buffer{}
				// thumbnailCtx = log.WithDebugValue(ctx, map[string]map[string]int{"function": {"Thumbnail": 9}})
				err := Thumbnail(ctx, bytes.NewReader(bs), &thumbnail, "jpeg")
				if err != nil {
					return err
				}
				if thumbnail.Len() == 0 {
					return fmt.Errorf("thumbnail buffer is empty")
				}
				require.Equal(t, 140969, thumbnail.Len())
				return nil
			})
		}

		err = g.Wait()
		require.NoError(t, err)
	})
}

// This segment once caused a segfault in gst-libav.
// It doesn't gotta work but it does gotta not crash.
func TestThumbnailKryptonite(t *testing.T) {
	withNoGSTLeaks(t, func() {
		inputFile, err := os.Open(remote.RemoteFixture("46c876d5e6c4124275b8856431833adaad31cb5246caca8ded9dc4d37de400a4/kryptonite-screenshot.mp4"))
		require.NoError(t, err)
		defer inputFile.Close()
		bs, err := io.ReadAll(inputFile)
		require.NoError(t, err)

		thumbnail := bytes.Buffer{}
		err = Thumbnail(context.Background(), bytes.NewReader(bs), &thumbnail, "png")
		require.NoError(t, err)
		require.Equal(t, 561486, thumbnail.Len())
	})
}

// This segment once caused the jpeg encoder to stall.
// So now we have snapshot=false.
func TestThumbnailStall(t *testing.T) {
	withNoGSTLeaks(t, func() {
		inputFile, err := os.Open(remote.RemoteFixture("aef704b702d24de7cf2ae453f4def763f3b39f4f353c8a1602f59cb995aafb53/broken-thumbnail.mp4"))
		require.NoError(t, err)
		defer inputFile.Close()
		bs, err := io.ReadAll(inputFile)
		require.NoError(t, err)
		thumbnail := bytes.Buffer{}
		err = Thumbnail(context.Background(), bytes.NewReader(bs), &thumbnail, "jpeg")
		require.NoError(t, err)
		// This is inconsistent. Which is concerning.
		require.Greater(t, thumbnail.Len(), 22000)
		require.Less(t, thumbnail.Len(), 25000)
	})
}
