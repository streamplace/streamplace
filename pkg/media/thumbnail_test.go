package media

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/log"
)

func TestThumbnail(t *testing.T) {
	gstinit.InitGST()
	before := getLeakCount(t)
	defer checkGStreamerLeaks(t, before+1)
	ignore := goleak.IgnoreCurrent()
	defer goleak.VerifyNone(t, ignore)

	// Open input file
	inputFile, err := os.Open(getFixture("sample-segment.mp4"))
	require.NoError(t, err)
	defer inputFile.Close()

	thumbnail := bytes.Buffer{}
	ctx := log.WithDebugValue(context.Background(), map[string]map[string]int{"function": {"Thumbnail": 9}})
	err = Thumbnail(ctx, inputFile, &thumbnail)
	require.NoError(t, err)
	require.NotNil(t, thumbnail)
	require.Greater(t, thumbnail.Len(), 0, "Thumbnail buffer should not be empty")
}
