package muxl

import (
	"bytes"
	"context"
	"crypto"
	"encoding/asn1"
	"encoding/pem"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1"
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

// TestRunMuxlSignerHostCallback exercises the --host-sign path: parse the
// muxl test key Go-side, hand RunMuxlSigner a Sign closure, and confirm the
// wasm comes back with valid signed bytes. This is the path that lets
// PKCS#11/EIP-712 signers work — key never enters the wasm sandbox.
func TestRunMuxlSignerHostCallback(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	muxlRoot := filepath.Join(repoRoot, "..", "muxl")

	keyPEM, err := os.ReadFile(filepath.Join(muxlRoot, "samples/test-keys/es256k-key.pem"))
	if err != nil {
		t.Skipf("skipping; sibling muxl repo with test keys not available: %v", err)
	}
	certPEM, err := os.ReadFile(filepath.Join(muxlRoot, "samples/test-keys/es256k-cert.pem"))
	require.NoError(t, err)
	segment, err := os.ReadFile(filepath.Join(repoRoot, "test/fixtures/sample-segment.mp4"))
	require.NoError(t, err)

	// Pull the secp256k1 scalar out of the PKCS#8 PEM ourselves so the test
	// stands on its own without depending on streamplace's signers package.
	priv := parseES256KPKCS8PEM(t, keyPEM)
	var signer crypto.Signer = priv.ToECDSA()

	manifest := []byte(`{
		"title": "muxl-sign host-callback integration test",
		"assertions": [
			{"label": "c2pa.actions",
			 "data": {"actions": [{"action": "c2pa.created"}]}}
		]
	}`)

	signed, err := RunMuxlSigner(context.Background(), SignerInput{
		Segment:         segment,
		CertPEM:         certPEM,
		Sign:            SignerToCallback(signer, 32),
		TrackManifest:   manifest,
		WrapperManifest: manifest,
	})
	require.NoError(t, err)
	require.NotEmpty(t, signed)
	require.True(t, bytes.Contains(signed[:64], []byte("ftyp")),
		"signed output should look like an MP4 (got %x...)", signed[:32])
}

// parseES256KPKCS8PEM extracts the 32-byte secp256k1 scalar from a v1
// PKCS#8 PEM. Just enough to drive the test — the production code path
// already uses signers.MarshalES256KPrivateKeyPEM in the other direction.
func parseES256KPKCS8PEM(t *testing.T, pemBytes []byte) *secp256k1.PrivateKey {
	t.Helper()
	block, _ := pem.Decode(pemBytes)
	require.NotNil(t, block, "PEM decode")
	require.Equal(t, "PRIVATE KEY", block.Type)

	// PKCS#8 PrivateKeyInfo: SEQUENCE { version INTEGER, algo AlgID, key OCTET STRING }
	var pkcs8 struct {
		Version    int
		Algo       struct {
			Algorithm  asn1.ObjectIdentifier
			Parameters asn1.RawValue
		}
		PrivateKey []byte
	}
	_, err := asn1.Unmarshal(block.Bytes, &pkcs8)
	require.NoError(t, err)

	// Inner ECPrivateKey: SEQUENCE { version INTEGER, privateKey OCTET STRING, ... }
	var ec struct {
		Version    int
		PrivateKey []byte
	}
	_, err = asn1.Unmarshal(pkcs8.PrivateKey, &ec)
	require.NoError(t, err)
	require.Len(t, ec.PrivateKey, 32, "expected 32-byte secp256k1 scalar")

	priv, _ := secp256k1.PrivKeyFromBytes(ec.PrivateKey)
	require.NotNil(t, priv)
	return priv
}
