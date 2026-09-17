package cmd

import (
	"testing"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
)

func TestWHIPViewerBaseRemovesIngestRoute(t *testing.T) {
	require.Equal(t, "http://127.0.0.1:48082", whipViewerBase("http://127.0.0.1:48082/api/ingest/webrtc"))
	require.Equal(t, "https://example.test", whipViewerBase("https://example.test/api/ingest/webrtc/key"))
	require.Equal(t, "http://127.0.0.1:48082", whipViewerBase("http://127.0.0.1:48082"))
}

func TestWHIPICEFailureStates(t *testing.T) {
	for _, state := range []webrtc.ICEConnectionState{
		webrtc.ICEConnectionStateFailed,
		webrtc.ICEConnectionStateDisconnected,
		webrtc.ICEConnectionStateClosed,
	} {
		require.Contains(t, failureStates, state)
	}
	require.NotContains(t, failureStates, webrtc.ICEConnectionStateCompleted,
		"ICE completed is a successful connection state")
}
