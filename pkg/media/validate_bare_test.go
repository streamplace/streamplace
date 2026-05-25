package media

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"os"
	"testing"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/crypto/aqpub"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/muxl"
)

// TestValidateMP4MediaBareSegment exercises the full .m4s-native validate
// path: sign the fragmented fixture per-segment (the live ingest shape),
// reassemble one GoP's bare canonical .m4s, and run ValidateMP4Media over it.
// That wraps the bare segment to a flat MP4 for gstreamer (codec/dimensions)
// and verifies the signatures in-wasm — proving a signed bare .m4s parses
// through qtdemux with the correct co64 offsets wrap-flat synthesizes.
func TestValidateMP4MediaBareSegment(t *testing.T) {
	ctx := context.Background()

	atPriv, err := atcrypto.GeneratePrivateKeyK256()
	require.NoError(t, err)
	secpPriv, _ := secp256k1.PrivKeyFromBytes(atPriv.Bytes())
	require.NotNil(t, secpPriv)
	var signer crypto.Signer = secpPriv.ToECDSA()
	cert, err := signers.GenerateES256KCert(signer)
	require.NoError(t, err)
	pub, err := aqpub.FromPublicKey(secpPriv.ToECDSA().Public().(*ecdsa.PublicKey))
	require.NoError(t, err)

	prebuilt := []byte(`{
		"title": "bare segment test",
		"assertions": [
			{"label":"c2pa.actions","data":{"actions":[{"action":"c2pa.created"}]}},
			{"label":"cawg.metadata","data":{
				"@context":{"dc":"http://purl.org/dc/elements/1.1/"},
				"dc:creator":"did:example","dc:title":"t",
				"dc:date":"1970-01-01T00:00:00.000Z"
			}}
		]
	}`)
	ms := &MediaSignerLocal{
		StreamerName:     "test-streamer",
		Signer:           signer,
		AQPub:            pub,
		Cert:             cert,
		PrebuiltManifest: prebuilt,
	}

	frag, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)

	// Sign per-segment; keep the first GoP's bare .m4s (what ValidateMP4 gets
	// per call in the live path).
	eventCh := make(chan *muxl.MuxlEvent, 16)
	errCh := make(chan error, 1)
	go func() {
		err := ms.SignSegmentStream(ctx, bytes.NewReader(frag), eventCh)
		close(eventCh)
		errCh <- err
	}()
	var m4s []byte
	for ev := range eventCh {
		if ev.Type == "signed-segment" && m4s == nil {
			m4s = concatTracksSorted(ev.Tracks)
		}
	}
	require.NoError(t, <-errCh)
	require.NotEmpty(t, m4s, "expected at least one signed GoP")

	res, err := ValidateMP4Media(ctx, m4s)
	require.NoError(t, err)
	require.NotNil(t, res.MediaData)
	require.NotEmpty(t, res.MediaData.Video, "should parse a video track")
	require.Greater(t, res.MediaData.Video[0].Width, 0, "video width from qtdemux")
	require.Greater(t, res.MediaData.Video[0].Height, 0, "video height from qtdemux")
	require.NotEmpty(t, res.MediaData.Audio, "should parse an audio track")
}
