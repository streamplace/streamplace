package media

import (
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/llhls"
)

func TestLLWindowIsKeyedByStreamerDID(t *testing.T) {
	mm := &MediaManager{llWindows: map[string]*llhls.Window{}}
	window := mm.llWindow("did:plc:streamer")

	require.Same(t, window, mm.GetLLWindow("did:plc:streamer"))
	require.Nil(t, mm.GetLLWindow("did:key:signing-key"))
}
