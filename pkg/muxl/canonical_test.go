package muxl

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// MUXL canonical-segment uuid (canonical-form.md § MUXL Canonical Segment).
var muxlUUID = [16]byte{
	0xe6, 0x40, 0x4e, 0xa2, 0x8f, 0x01, 0x43, 0x05,
	0x98, 0xda, 0x7b, 0xec, 0x3c, 0x2a, 0x91, 0x73,
}

// c2pa BMFF uuid (RFC-spec'd C2PA box).
var c2paUUID = [16]byte{
	0xd8, 0xfe, 0xc3, 0xd6, 0x1b, 0x0e, 0x48, 0x3c,
	0x92, 0x97, 0x58, 0x28, 0x87, 0x7e, 0xc4, 0x81,
}

// signSampleSegment runs the muxl-sign wasm against the muxl repo's
// sample-segment fixture using the muxl test keys. Skipped when the
// sibling muxl repo isn't available.
func signSampleSegment(t *testing.T, ctx context.Context) []byte {
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
	segment, err := os.ReadFile(filepath.Join(repoRoot, "test/fixtures/sample-segment.mp4"))
	require.NoError(t, err)

	manifest := []byte(`{
		"title": "StripFlatMP4WrapperToBundle test",
		"assertions": [
			{"label": "c2pa.actions",
			 "data": {"actions": [{"action": "c2pa.created"}]}}
		]
	}`)

	signed, err := RunMuxlSigner(ctx, SignerInput{
		Segment:         segment,
		CertPEM:         certPEM,
		KeyPEM:          keyPEM,
		TrackManifest:   manifest,
		WrapperManifest: manifest,
	})
	require.NoError(t, err)
	require.NotEmpty(t, signed)
	return signed
}

// TestStripFlatMP4WrapperToBundle confirms that the canonical Streamplace
// archive shape (the bundle returned by StripFlatMP4WrapperToBundle) is
// what we expect: a concatenation of `[c2pa-uuid + muxl-uuid + moof+mdat]`
// canonical segments and nothing else. The bundle is smaller than the
// input by exactly (ftyp + wrapper-c2pa-uuid + moov + mdat-envelope-header)
// bytes.
func TestStripFlatMP4WrapperToBundle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signed := signSampleSegment(t, ctx)

	bundle, err := StripFlatMP4WrapperToBundle(signed)
	require.NoError(t, err)
	require.NotEmpty(t, bundle)
	require.Less(t, len(bundle), len(signed), "bundle should be smaller than the flat MP4")

	// Bundle begins with a c2pa-uuid (per-canonical-segment signature).
	require.GreaterOrEqual(t, len(bundle), 24, "bundle must be long enough for a uuid box")
	require.Equal(t, "uuid", string(bundle[4:8]),
		"bundle's first box must be a uuid (c2pa per-segment manifest)")
	require.Equal(t, c2paUUID[:], bundle[8:24],
		"bundle's first uuid must be the c2pa identifier")

	// Walking past the c2pa-uuid, the next box must be the muxl uuid.
	c2paSize := boxSize(t, bundle, 0)
	require.GreaterOrEqual(t, len(bundle), c2paSize+24, "bundle must continue past c2pa-uuid")
	require.Equal(t, "uuid", string(bundle[c2paSize+4:c2paSize+8]),
		"box after c2pa-uuid must be a uuid (muxl catalog)")
	require.Equal(t, muxlUUID[:], bundle[c2paSize+8:c2paSize+24],
		"second uuid must be the muxl identifier")

	// Walking the bundle as a sequence of top-level boxes must consume
	// it completely with no leftover bytes — that's how we know the
	// bundle is well-formed (no orphan tail, no ftyp/moov contamination).
	off := 0
	sawC2pa := 0
	sawMuxl := 0
	sawMoofMdat := 0
	for off < len(bundle) {
		s := boxSize(t, bundle, off)
		kind := string(bundle[off+4 : off+8])
		if kind == "uuid" {
			id := bundle[off+8 : off+24]
			switch {
			case bytes.Equal(id, c2paUUID[:]):
				sawC2pa++
			case bytes.Equal(id, muxlUUID[:]):
				sawMuxl++
			default:
				t.Fatalf("unexpected uuid identifier at %d: %x", off, id)
			}
		} else if kind == "moof" || kind == "mdat" {
			sawMoofMdat++
		} else {
			t.Fatalf("unexpected box %q in bundle at %d", kind, off)
		}
		off += s
	}
	require.Equal(t, len(bundle), off, "bundle walk must consume the full buffer")
	require.GreaterOrEqual(t, sawC2pa, 1, "bundle has at least one c2pa per-segment uuid")
	require.Equal(t, sawC2pa, sawMuxl,
		"every c2pa-signed canonical segment carries exactly one muxl uuid")
	require.Greater(t, sawMoofMdat, 0, "bundle has at least one moof+mdat pair")

	// Sanity: the original signed flat MP4 has ftyp at the top followed
	// by the wrapper c2pa-uuid. The bundle starts with a *per-segment*
	// c2pa-uuid instead — they're at different positions in their
	// containers and serve different purposes.
	require.Equal(t, "ftyp", string(signed[4:8]),
		"signed flat MP4 begins with ftyp")
	ftypSize := boxSize(t, signed, 0)
	require.GreaterOrEqual(t, len(signed), ftypSize+24)
	require.Equal(t, "uuid", string(signed[ftypSize+4:ftypSize+8]),
		"the box after ftyp is the wrapper c2pa-uuid")
	require.Equal(t, c2paUUID[:], signed[ftypSize+8:ftypSize+24])
}

func boxSize(t *testing.T, buf []byte, off int) int {
	t.Helper()
	require.GreaterOrEqual(t, len(buf)-off, 8, "box header at %d truncated", off)
	size32 := binary.BigEndian.Uint32(buf[off : off+4])
	if size32 == 1 {
		require.GreaterOrEqual(t, len(buf)-off, 16, "largesize header truncated at %d", off)
		return int(binary.BigEndian.Uint64(buf[off+8 : off+16]))
	}
	if size32 == 0 {
		return len(buf) - off
	}
	return int(size32)
}
