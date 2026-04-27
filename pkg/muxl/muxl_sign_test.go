package muxl

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func init() {
	stderrWriter = func(_ context.Context, _ uint64) io.Writer { return os.Stderr }
}

// TestRunMuxlSigner feeds a flat MP4 fixture through the muxl-sign wasm with
// the muxl test cert+key and confirms the output is non-empty signed MP4
// data. The smallest integration check that the wasm runs end-to-end from Go.
func TestRunMuxlSigner(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	muxlRoot := filepath.Join(repoRoot, "..", "muxl")

	keyPEM, err := os.ReadFile(filepath.Join(muxlRoot, "samples/test-keys/es256k-key.pem"))
	if err != nil {
		t.Skipf("skipping; sibling muxl repo with test keys not available at %s: %v", muxlRoot, err)
	}
	certPEM, err := os.ReadFile(filepath.Join(muxlRoot, "samples/test-keys/es256k-cert.pem"))
	require.NoError(t, err)

	segment, err := os.ReadFile(filepath.Join(repoRoot, "test/fixtures/sample-segment.mp4"))
	require.NoError(t, err)

	manifest := []byte(`{
		"title": "muxl-sign streamplace integration test",
		"assertions": [
			{"label": "c2pa.actions",
			 "data": {"actions": [{"action": "c2pa.created"}]}}
		]
	}`)

	signed, err := RunMuxlSigner(context.Background(), SignerInput{
		Segment:         segment,
		CertPEM:         certPEM,
		KeyPEM:          keyPEM,
		TrackManifest:   manifest,
		WrapperManifest: manifest,
	})
	require.NoError(t, err)
	require.NotEmpty(t, signed)
	// MP4 magic: bytes 4..8 are "ftyp" for a normal flat MP4.
	require.True(t, bytes.Contains(signed[:64], []byte("ftyp")),
		"signed output should look like an MP4 (got %x...)", signed[:32])
}
