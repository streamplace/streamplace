package media

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/aqio"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/test/remote"
)

func TestCombineSegmentsUnsigned(t *testing.T) {
	withNoGSTLeaks(t, func() {
		g, _ := errgroup.WithContext(context.Background())
		for range streamplaceTestCount {
			g.Go(func() error {
				return innerTestClip(t)
			})
		}
		err := g.Wait()
		require.NoError(t, err)
	})
}

func innerTestClip(t *testing.T) error {
	ctx := log.WithDebugValue(context.Background(), map[string]map[string]int{"func": {"ConcatDemuxBin": 9, "ConcatBin": 9}})
	dirname := remote.RemoteArchive("c21e9352e72ca0729c66af2fcabec1b8997b509601241e8d38d5728f9687386b/threesegs.tar.gz")
	inputFiles := []string{
		fmt.Sprintf("%s/2025-11-15T21-05-00-399Z.mp4", dirname),
		fmt.Sprintf("%s/2025-11-15T21-05-01-385Z.mp4", dirname),
		fmt.Sprintf("%s/2025-11-15T21-05-02-393Z.mp4", dirname),
	}
	inputFds := make([]io.ReadSeeker, len(inputFiles))
	for i, fName := range inputFiles {
		fd, err := os.Open(fName)
		if err != nil {
			return fmt.Errorf("unable to open segment file: %w", err)
		}
		inputFds[i] = fd
	}
	buf := aqio.NewReadWriteSeeker([]byte{})
	err := CombineSegmentsUnsigned(ctx, inputFds, buf, true)
	require.NoError(t, err)
	slice, err := buf.Bytes()
	require.NoError(t, err)
	require.Equal(t, 4725181, len(slice))
	return nil
}
