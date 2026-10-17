package rtcrec

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

// TestNewRecordingPeerConnectionSurvivesSinkFailure pins the best-effort
// contract for the WHIP path: a debug-recording sink that won't open (an S3
// bucket rejecting the upload — billing, credentials — or an unwritable data
// dir) must not fail the WebRTC connection. Failing here used to abort the
// whole ingest session, so a recording problem took the broadcast down with it.
func TestNewRecordingPeerConnectionSurvivesSinkFailure(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	// A regular file where the recording tree belongs makes every
	// DebugRecordingCreate fail with ENOTDIR — the local-disk analogue of an S3
	// rejection, and the same sabotage the e2e harness applies.
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "debug-recordings"), []byte("not a directory"), 0o644))

	pionpc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)

	pc, err := NewRecordingPeerConnection(ctx, config.CLI{DataDir: dataDir}, "did:plc:test", pionpc, true)
	require.NoError(t, err, "a broken recording sink must not fail the stream")

	// The connection carries on, just without recording: nothing to finalize,
	// and Close must not wait on a commit that can never happen.
	rec := pc.(*RecordingPeerConnection)
	require.False(t, rec.enabled, "the connection continues without recording")
	rec.FinalizeRecording(ctx)
	require.NoError(t, pc.Close())
}
