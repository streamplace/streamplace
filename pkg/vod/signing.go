package vod

import (
	"encoding/json"
	"fmt"
	"time"

	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/crypto/spkey"
	"stream.place/streamplace/pkg/muxl"
)

// uploadSigner is the per-upload ephemeral signing material. The private
// key exists only for the lifetime of one ProcessVOD call: it signs
// every segment of the upload, then the caller drops it — so once
// processing finishes nobody can mint more signatures under DIDKey.
type uploadSigner struct {
	// SignerInput carries the cert + PKCS#8 key PEM + manifest handed to
	// muxl-sign's sign-segment subcommand.
	SignerInput muxl.SignerInput
	// DIDKey is the did:key derived from the public half. It's recorded
	// on every place.stream.media.track for this upload so verifiers can
	// match the embedded C2PA signatures back to a key.
	DIDKey string
}

// newUploadSigner generates a fresh secp256k1 keypair, self-signs an
// ES256K cert for it, and bundles everything sign-segment needs. The
// private key material lives only in the returned SignerInput; it's
// never persisted.
func newUploadSigner(processedAt time.Time) (*uploadSigner, error) {
	priv, pub, err := spkey.GenerateStreamKey()
	if err != nil {
		return nil, fmt.Errorf("generate signing key: %w", err)
	}
	signer, err := spkey.KeyToSigner(priv)
	if err != nil {
		return nil, fmt.Errorf("convert signing key: %w", err)
	}
	cert, err := signers.GenerateES256KCert(signer)
	if err != nil {
		return nil, fmt.Errorf("generate signing cert: %w", err)
	}
	keyPEM, err := signers.MarshalES256KPrivateKeyPEM(signer)
	if err != nil {
		return nil, fmt.Errorf("marshal signing key: %w", err)
	}
	manifest, err := buildSignManifest(processedAt)
	if err != nil {
		return nil, err
	}
	return &uploadSigner{
		SignerInput: muxl.SignerInput{
			CertPEM:         cert,
			KeyPEM:          keyPEM,
			TrackManifest:   manifest,
			WrapperManifest: manifest,
			Alg:             "es256k",
		},
		DIDKey: pub.DIDKey(),
	}, nil
}

// buildSignManifest produces the minimal C2PA manifest embedded in every
// signed VOD segment: the standard c2pa.created/c2pa.published action
// pair stamped with the processing time. muxl-sign applies it to each
// per-track canonical segment.
func buildSignManifest(processedAt time.Time) ([]byte, error) {
	ts := processedAt.UTC().Format(time.RFC3339)
	mani := map[string]any{
		"title": fmt.Sprintf("Streamplace VOD processed at %s", ts),
		"assertions": []map[string]any{
			{
				"label": "c2pa.actions",
				"data": map[string]any{
					"actions": []map[string]any{
						{"action": "c2pa.created", "when": ts},
						{"action": "c2pa.published", "when": ts},
					},
				},
			},
		},
	}
	body, err := json.Marshal(mani)
	if err != nil {
		return nil, fmt.Errorf("marshal sign manifest: %w", err)
	}
	return body, nil
}
