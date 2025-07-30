package media

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestClip(t *testing.T) {
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
	fName := getFixture("sample-segment.mp4")
	inputFiles := []string{fName, fName, fName}
	buf := bytes.NewBuffer(nil)
	err := Clip(context.Background(), inputFiles, buf)
	require.NoError(t, err)
	require.Equal(t, 2986131, buf.Len())
	return nil
}
