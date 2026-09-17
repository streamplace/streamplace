package mistconfig

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

func TestGenerateUsesPushSourceForFMP4PullIngest(t *testing.T) {
	configBytes, err := Generate(&config.CLI{
		HTTPInternalAddr: "127.0.0.1:39090",
		MistHTTPPort:     28080,
		MistRTMPPort:     11935,
	})
	require.NoError(t, err)

	var generated map[string]any
	require.NoError(t, json.Unmarshal(configBytes, &generated))
	streams, ok := generated["streams"].(map[string]any)
	require.True(t, ok)
	stream, ok := streams[StreamName].(map[string]any)
	require.True(t, ok)

	require.Equal(t, float64(LiveSegmentTargetMS), stream["segmentsize"],
		"Mist should target segments that leave processing headroom under one second")
	require.Equal(t, "push://", stream["source"],
		"the running node pulls Mist's fMP4 output after a push is accepted")
	require.NotContains(t, stream["source"], "mkv-exec",
		"the legacy MKV bridge reconstructs timestamps and bypasses fMP4 pull ingest")
}
