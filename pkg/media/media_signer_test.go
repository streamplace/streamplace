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
	"stream.place/streamplace/pkg/aqio"
	c2patypes "stream.place/streamplace/pkg/c2patypes"
	"stream.place/streamplace/pkg/crypto/aqpub"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/iroh/generated/iroh_streamplace"
)

// TestSignMP4Roundtrip exercises the full muxl-sign integration: build a
// signer, sign a fixture, then read the signed output back through c2pa-rs
// (via the existing iroh_streamplace bindings used by ValidateMP4Media) to
// confirm the manifest is well-formed and carries our streamer's identity.
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

	maniStr, err := iroh_streamplace.GetManifestAndCert(c2patypes.NewReader(aqio.NewReadWriteSeeker(signed)))
	require.NoError(t, err)

	var maniCert ManifestAndCert
	require.NoError(t, json.Unmarshal([]byte(maniStr), &maniCert))

	require.NotNil(t, maniCert.Manifest.Title)
	require.Equal(t, "TestSignMP4Roundtrip", *maniCert.Manifest.Title)
	require.NotEmpty(t, maniCert.Cert, "wrapper manifest should expose a cert chain")
}
