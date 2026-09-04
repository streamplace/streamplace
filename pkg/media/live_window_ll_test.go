package media

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/llhls"
)

func TestLLWindowIsKeyedByStreamerDID(t *testing.T) {
	mm := &MediaManager{llWindows: map[string]*llhls.Window{}}
	window := mm.replaceLLWindow("did:plc:streamer")

	require.Same(t, window, mm.GetLLWindow("did:plc:streamer"))
	require.Nil(t, mm.GetLLWindow("did:key:signing-key"))
}

func TestLLWindowUsesMobilePlaybackHoldBack(t *testing.T) {
	mm := &MediaManager{llWindows: map[string]*llhls.Window{}}
	w := mm.replaceLLWindow("did:plc:streamer")
	require.NoError(t, w.Observe(llhls.Event{Kind: llhls.Init, Presentation: "p", Track: "video", Generation: 1}))
	require.NoError(t, w.Observe(llhls.Event{
		Kind:         llhls.Part,
		Presentation: "p",
		Track:        "video",
		Generation:   1,
		MSN:          1,
		Part:         0,
		Duration:     time.Second,
		Data:         []byte("part"),
	}))

	playlist := w.Playlist("p", "video", func(uint64, uint32) string { return "part.m4s" }, func(uint64) string { return "segment.m4s" }, "init.mp4", nil)
	require.Contains(t, playlist, "#EXT-X-TARGETDURATION:1")
	require.Contains(t, playlist, "PART-HOLD-BACK=5.500000")
	require.Contains(t, playlist, "HOLD-BACK=3.000000")
}

func TestRemoveLLWindowOnlyRemovesMatchingPresentation(t *testing.T) {
	mm := &MediaManager{llWindows: map[string]*llhls.Window{}}
	w := mm.replaceLLWindow("did:plc:streamer")
	require.NoError(t, w.Observe(llhls.Event{Kind: llhls.Init, Presentation: "p2", Session: 2, Track: "video", Generation: 1}))

	mm.removeLLWindow("did:plc:streamer", "p1", w)
	require.Same(t, w, mm.GetLLWindow("did:plc:streamer"))
	mm.removeLLWindow("did:plc:streamer", "p2", newLLWindow())
	require.Same(t, w, mm.GetLLWindow("did:plc:streamer"))
	mm.removeLLWindow("did:plc:streamer", "p2", w)
	require.Nil(t, mm.GetLLWindow("did:plc:streamer"))
}
