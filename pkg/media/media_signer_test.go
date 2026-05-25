package media

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"encoding/json"
	"os"
	"testing"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/stretchr/testify/require"
	c2patypes "stream.place/streamplace/pkg/c2patypes"
	"stream.place/streamplace/pkg/crypto/aqpub"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/muxl"
)

// TestSignMP4Roundtrip exercises the full muxl-sign integration: build a
// signer, sign a fixture, then read the signed output back through the
// in-wasm verify (muxl.RunMuxlVerify — the same path ValidateMP4 now uses) to
// confirm each canonical segment validates and carries our streamer's
// identity.
func TestSignMP4Roundtrip(t *testing.T) {
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
		"title": "TestSignMP4Roundtrip",
		"assertions": [
			{"label": "c2pa.actions",
			 "data": {"actions": [{"action": "c2pa.created"}]}}
		]
	}`)

	ms := &MediaSignerLocal{
		StreamerName:     "test-streamer",
		Signer:           signer,
		AQPub:            pub,
		Cert:             cert,
		PrebuiltManifest: prebuilt,
	}

	segment, err := os.ReadFile(getFixture("sample-segment.mp4"))
	require.NoError(t, err)

	signed, err := ms.SignMP4(ctx, bytes.NewReader(segment), 0)
	require.NoError(t, err)
	require.NotEmpty(t, signed)

	out, err := muxl.RunMuxlVerify(ctx, bytes.NewReader(signed))
	require.NoError(t, err)

	var doc struct {
		Segments []struct {
			Manifest c2patypes.Manifest `json:"manifest"`
			Cert     string             `json:"cert"`
		} `json:"segments"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &doc))
	require.NotEmpty(t, doc.Segments, "verify should return canonical segments")
	for i, seg := range doc.Segments {
		require.NotNil(t, seg.Manifest.Title, "segment %d has a title", i)
		require.Equal(t, "TestSignMP4Roundtrip", *seg.Manifest.Title)
		require.NotEmpty(t, seg.Cert, "segment %d manifest should expose a cert chain", i)
	}
}
