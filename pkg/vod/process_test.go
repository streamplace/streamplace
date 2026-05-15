package vod

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/pkg/log"
)

func getFixture(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	return filepath.Join(dir, "..", "..", "test", "fixtures", name)
}

// gstWarmup ensures gstreamer is initialized once per test process. The
// VOD-pipeline tests skip the leak-tracing wrapper (see comment in
// pkg/media/vod_pipeline_test.go).
var gstWarmup sync.Once

func warmGST() { gstWarmup.Do(gstinit.InitGST) }

// TestStreamThroughMuxl is the end-to-end test for the gstreamer +
// muxl-sign section of the VOD pipeline. It feeds the h264+opus fixture
// through RunVODPipeline -> mp4mux -> muxl sign-segment and captures the
// result in a bytes.Buffer + bdasl.Writer. Asserts:
//
//   - the output starts with an ftyp box
//   - the output is non-trivial (> 1 KB)
//   - a CID is computable from the output
//
// The output is NOT deterministic across runs: sign-segment embeds COSE
// signatures with real-clock timestamps and nonces, so the CID shifts
// every run. The S3 multipart upload + content-addressed key rename are
// not exercised here; those are tested separately in pkg/s3.
func TestStreamThroughMuxl(t *testing.T) {
	warmGST()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = log.WithLogValues(ctx, "test", "TestStreamThroughMuxl")

	fixture, err := os.ReadFile(getFixture("5sec.mp4"))
	require.NoError(t, err)

	signer, err := newUploadSigner(time.Now())
	require.NoError(t, err)

	out := &bytes.Buffer{}
	hasher := bdasl.NewWriter()

	// streamThroughMuxl writes to dst; we tee that into a hasher so the
	// test can assert the CID without re-reading the output.
	dst := teeWriter{hasher, out}

	_, err = streamThroughMuxl(ctx, bytes.NewReader(fixture), int64(len(fixture)), dst, nil, signer.SignerInput)
	require.NoError(t, err)

	require.GreaterOrEqual(t, out.Len(), 1024, "expected non-trivial fMP4 output, got %d bytes", out.Len())
	require.Equal(t, "ftyp", string(out.Bytes()[4:8]), "expected fMP4 ftyp box at start")

	cid := hasher.CID()
	require.NotEmpty(t, cid)

	// SHA-256 of the output gives us a sanity check that the output is
	// stable across the bdasl/blake3 implementation. We don't pin a
	// specific hash since the gstreamer + muxl wasm output can shift
	// across builds; we just confirm both hashes are computable.
	full := sha256.Sum256(out.Bytes())
	require.NotEmpty(t, hex.EncodeToString(full[:]))
}

// teeWriter is io.MultiWriter inlined to two writers — slightly cheaper
// per Write call than allocating a slice via io.MultiWriter.
type teeWriter [2]interface {
	Write([]byte) (int, error)
}

func (t teeWriter) Write(p []byte) (int, error) {
	for _, w := range t {
		if _, err := w.Write(p); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

// TestMetafileBuilder runs the full streamThroughMuxl path with a
// metafile builder attached and a real (file-backed) blob.Store, then
// verifies the assembled metafile has the shape we promise downstream
// (worker + spxrpc handlers):
//   - tracks keyed by stringified track ID
//   - per-track codec/timescale + video width/height OR audio rate/channels
//   - segments with strictly increasing offsets and non-zero sizes
//   - each track's initCid was written into the store at blobs/<initCid>.mp4
func TestMetafileBuilder(t *testing.T) {
	warmGST()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = log.WithLogValues(ctx, "test", "TestMetafileBuilder")

	fixture, err := os.ReadFile(getFixture("5sec.mp4"))
	require.NoError(t, err)

	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)

	signer, err := newUploadSigner(time.Now())
	require.NoError(t, err)

	out := &bytes.Buffer{}
	hasher := bdasl.NewWriter()
	dst := teeWriter{hasher, out}

	mb := newMetafileBuilder(ctx, store)
	_, err = streamThroughMuxl(ctx, bytes.NewReader(fixture), int64(len(fixture)), dst, mb, signer.SignerInput)
	require.NoError(t, err)

	cid := hasher.CID()
	meta := mb.Finalize(cid, int64(out.Len()))

	require.Equal(t, cid, meta.BlobCID)
	require.Equal(t, int64(out.Len()), meta.BlobSize)
	require.NotEmpty(t, meta.Tracks, "expected at least one track")

	for tid, tr := range meta.Tracks {
		require.NotEmpty(t, tr.InitCID, "track %s missing initCid", tid)
		require.Equal(t, cid, tr.BlobCID)
		require.Equal(t, int64(out.Len()), tr.BlobSize)
		require.NotEmpty(t, tr.Codec, "track %s missing codec", tid)
		require.NotZero(t, tr.Timescale, "track %s missing timescale", tid)
		switch tr.Type {
		case "video":
			require.NotZero(t, tr.Width, "video track %s missing width", tid)
			require.NotZero(t, tr.Height, "video track %s missing height", tid)
		case "audio":
			require.NotZero(t, tr.SampleRate, "audio track %s missing sampleRate", tid)
			require.NotZero(t, tr.Channels, "audio track %s missing channels", tid)
		default:
			t.Fatalf("unexpected track type %q for %s", tr.Type, tid)
		}
		require.NotEmpty(t, tr.Segments, "track %s has no segments", tid)
		// Offsets must be strictly increasing per track; sizes nonzero.
		for i, seg := range tr.Segments {
			require.Positive(t, seg.Size, "track %s segment %d zero size", tid, i)
			if i > 0 {
				require.Greater(t, seg.Offset, tr.Segments[i-1].Offset,
					"track %s segments not strictly increasing in offset", tid)
			}
		}
		// Init blob was actually written into the store.
		r, err := store.Open(ctx, BlobsPrefix+tr.InitCID+".mp4")
		require.NoError(t, err, "init blob for track %s not written", tid)
		require.Positive(t, r.Size())
		require.NoError(t, r.Close())
	}

	// JSON encodes cleanly — sanity-check the shape that the playback
	// handlers and CF worker both consume.
	encoded, err := json.Marshal(meta)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"blobCid"`)
	require.Contains(t, string(encoded), `"tracks"`)
	require.Contains(t, string(encoded), `"durationTicks"`)
	require.Contains(t, string(encoded), `"sampleCount"`)
}
