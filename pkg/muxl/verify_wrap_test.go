package muxl

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

// signedM4sStream signs the fragmented fixture through sign-segment and
// reassembles the per-track signed canonical-segment bytes into one bare
// .m4s stream — the storage/wire form Streamplace keeps and feeds back to
// verify/wrap. Returns ("", "") via t.Skip if the sibling muxl keys are
// missing.
func signedM4sStream(t *testing.T) []byte {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	muxlRoot := filepath.Join(repoRoot, "..", "muxl")

	keyPEM, err := os.ReadFile(filepath.Join(muxlRoot, "samples/test-keys/es256k-key.pem"))
	if err != nil {
		t.Skipf("skipping; sibling muxl repo with test keys not available at %s: %v", muxlRoot, err)
	}
	certPEM, err := os.ReadFile(filepath.Join(muxlRoot, "samples/test-keys/es256k-cert.pem"))
	require.NoError(t, err)
	frag, err := os.ReadFile(filepath.Join(repoRoot, "test/fixtures/h264-opus-frag.mp4"))
	require.NoError(t, err)

	manifest := []byte(`{
		"title": "muxl verify/wrap integration test",
		"assertions": [
			{"label": "c2pa.actions",
			 "data": {"actions": [{"action": "c2pa.created"}]}}
		]
	}`)

	eventCh := make(chan *MuxlEvent, 16)
	errCh := make(chan error, 1)
	go func() {
		err := RunMuxlSignSegment(context.Background(), bytes.NewReader(frag), SignerInput{
			CertPEM:         certPEM,
			KeyPEM:          keyPEM,
			TrackManifest:   manifest,
			WrapperManifest: manifest,
		}, nil, nil, eventCh)
		close(eventCh)
		errCh <- err
	}()

	var m4s []byte
	for ev := range eventCh {
		if ev.Type != "signed-segment" {
			continue
		}
		tids := make([]string, 0, len(ev.Tracks))
		for tid := range ev.Tracks {
			tids = append(tids, tid)
		}
		sort.Strings(tids)
		for _, tid := range tids {
			m4s = append(m4s, ev.Tracks[tid]...)
		}
	}
	require.NoError(t, <-errCh)
	require.NotEmpty(t, m4s, "sign-segment produced no signed track bytes")
	return m4s
}

// TestRunMuxlVerify confirms in-wasm c2pa validation of bare canonical .m4s:
// the per-track signed segments verify standalone (no synthesized header),
// and the JSON matches the manifest+cert shape the host parses.
func TestRunMuxlVerify(t *testing.T) {
	m4s := signedM4sStream(t)

	out, err := RunMuxlVerify(context.Background(), bytes.NewReader(m4s))
	require.NoError(t, err)

	var doc struct {
		Segments []struct {
			TrackID         uint32          `json:"track_id"`
			Manifest        json.RawMessage `json:"manifest"`
			Cert            string          `json:"cert"`
			ValidationState string          `json:"validation_state"`
		} `json:"segments"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &doc), "verify output is JSON: %s", out)
	require.GreaterOrEqual(t, len(doc.Segments), 2, "expected video+audio segments")
	for i, seg := range doc.Segments {
		require.NotEmpty(t, seg.Manifest, "segment %d has a manifest", i)
		require.Contains(t, seg.Cert, "BEGIN CERTIFICATE", "segment %d has a PEM cert", i)
		require.NotEqual(t, "Invalid", seg.ValidationState, "segment %d validates", i)
	}
}

// TestRunMuxlWrap confirms the inbound header-synthesis: a bare signed .m4s
// stream wraps into a playable fMP4 (and an init-only header), with the
// segment bytes carried through verbatim.
func TestRunMuxlWrap(t *testing.T) {
	m4s := signedM4sStream(t)

	var fmp4 bytes.Buffer
	require.NoError(t, RunMuxlWrap(context.Background(), bytes.NewReader(m4s), "fmp4", &fmp4))
	require.Greater(t, fmp4.Len(), len(m4s), "wrapped fMP4 should be larger than the bare segments")
	require.True(t, bytes.Contains(fmp4.Bytes()[:64], []byte("ftyp")),
		"wrapped output starts with an ftyp box (got %x)", fmp4.Bytes()[:32])
	// The signed segment bytes pass through verbatim — the bare stream is a
	// substring of the wrapped fMP4 (header is prepended, segments untouched).
	require.True(t, bytes.Contains(fmp4.Bytes(), m4s),
		"wrapped fMP4 must contain the verbatim signed segment bytes")

	var init bytes.Buffer
	require.NoError(t, RunMuxlWrapInit(context.Background(), bytes.NewReader(m4s), &init))
	require.True(t, bytes.Contains(init.Bytes()[:64], []byte("ftyp")),
		"init-only output starts with an ftyp box")
	require.Less(t, init.Len(), fmp4.Len(), "init-only is smaller than the full wrap")
}
