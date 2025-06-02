package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/gstinit"
)

func TestThumbnail(t *testing.T) {
	gstinit.InitGST()
	before := getLeakCount(t)
	defer checkGStreamerLeaks(t, before)
	ignore := goleak.IgnoreCurrent()
	defer goleak.VerifyNone(t, ignore)

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
			require.Equal(t, thumbnail.Len(), 1418910)
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
			require.Equal(t, thumbnail.Len(), 140969)
			return nil
		})
	}

	err = g.Wait()
	require.NoError(t, err)
}
