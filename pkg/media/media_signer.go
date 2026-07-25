package media

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/aqpub"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/muxl"
)

var signerTracer = otel.Tracer("signer")

type MediaSigner interface {
	// SignSegmentStream streams an fMP4 input through muxl-sign's per-segment
	// signer, emitting one signed-segment event per GoP on eventCh. The
	// segment bytes routed in each event are bare canonical .m4s
	// ([c2pa-uuid][muxl-uuid][moof][mdat] per track) — no flat wrapper.
	SignSegmentStream(ctx context.Context, input io.Reader, eventCh chan *muxl.MuxlEvent) error
	Pub() aqpub.Pub
	Streamer() string
	DID() string
}

var DoReplay = false

// Manifester builds the C2PA manifest JSON SignSegmentStream embeds in each
// signed GoP. Production uses *ManifestBuilder (model-driven); tests can plug
// in a stub to drive mid-stream manifest changes.
type Manifester interface {
	BuildManifest(ctx context.Context, streamerName string, start int64) ([]byte, error)
}

type MediaSignerLocal struct {
	StreamerName     string
	Signer           crypto.Signer
	AQPub            aqpub.Pub
	Cert             []byte
	TAURL            string
	did              string
	manifestBuilder  Manifester
	PrebuiltManifest []byte // Optional: use this manifest instead of building one
	sigs             [][]byte
}

func prepareCert(ctx context.Context, cli *config.CLI, signer crypto.Signer) ([]byte, error) {

	cert, err := signers.GenerateES256KCert(signer)
	if err != nil {
		return nil, err
	}

	return cert, nil
}

func MakeMediaSigner(ctx context.Context, cli *config.CLI, streamer string, signer crypto.Signer, model model.Model) (MediaSigner, error) {
	cert, err := prepareCert(ctx, cli, signer)
	if err != nil {
		return nil, err
	}
	pub, err := aqpub.FromPublicKey(signer.Public().(*ecdsa.PublicKey))
	if err != nil {
		return nil, err
	}
	did, err := atproto.ParsePubKey(signer.Public().(*ecdsa.PublicKey))
	if err != nil {
		return nil, err
	}
	return &MediaSignerLocal{
		Signer:          signer,
		Cert:            cert,
		StreamerName:    streamer,
		TAURL:           cli.TAURL,
		AQPub:           pub,
		did:             did.DIDKey(),
		manifestBuilder: NewManifestBuilder(model, cli),
	}, nil
}

func (ms *MediaSignerLocal) Streamer() string {
	return ms.StreamerName
}

// buildManifest produces the C2PA manifest JSON for this signer: a prebuilt
// manifest if set, else the model-driven ManifestBuilder, else a minimal
// fallback. `start` seeds the cawg.metadata/dc:date — for the streaming
// signer that's a stream-level placeholder the wasm overwrites per segment.
func (ms *MediaSignerLocal) buildManifest(ctx context.Context, start int64) ([]byte, error) {
	switch {
	case len(ms.PrebuiltManifest) > 0:
		return ms.PrebuiltManifest, nil
	case ms.manifestBuilder != nil:
		return ms.manifestBuilder.BuildManifest(ctx, ms.StreamerName, start)
	default:
		log.Warn(ctx, "manifestBuilder is nil, using fallback manifest - this indicates model was not passed to MakeMediaSigner", "streamer", ms.StreamerName)
		title := "livestream"
		ts := aqtime.FromMillis(start).String()
		mani := obj{
			"title": fmt.Sprintf("Livestream Segment at %s", ts),
			"assertions": []obj{
				{
					"label": "c2pa.actions",
					"data": obj{
						"actions": []obj{
							{"action": "c2pa.created", "when": ts},
							{"action": "c2pa.published", "when": ts},
						},
					},
				},
				{
					"label": "cawg.metadata",
					"data": obj{
						"@context":   obj{"dc": "http://purl.org/dc/elements/1.1/"},
						"dc:creator": ms.StreamerName,
						"dc:title":   title,
						"dc:date":    ts,
					},
				},
			},
		}
		return json.Marshal(mani)
	}
}

// SignSegmentStream streams an fMP4 input through muxl-sign's per-segment
// signer, emitting one signed-segment event per GoP on eventCh.
//
// The manifest is fetched FRESH from buildManifest once per GoP via muxl's
// host_get_manifest callback, so connection-lifetime fields (livestream
// EndedAt → c2pa.published, title, metadata config) update mid-stream without
// needing to terminate the RTMP session. Before this, the manifest was sealed
// at connection start and a pre-live → live transition stayed invisible until
// the streamer reconnected.
//
// muxl-sign stamps each segment's signing time into cawg.metadata/dc:date as
// it signs. Signing backend: an *ecdsa.PrivateKey is marshaled to PEM and
// signed in-wasm, otherwise the host-callback path keeps the key out of the
// sandbox.
func (ms *MediaSignerLocal) SignSegmentStream(ctx context.Context, input io.Reader, eventCh chan *muxl.MuxlEvent) error {
	ctx, span := signerTracer.Start(ctx, "SignSegmentStream", trace.WithAttributes(
		attribute.String("streamer", ms.StreamerName),
	))
	defer span.End()

	// One callback shared by both kinds — track and wrapper manifests are the
	// same JSON in Streamplace today, so a single buildManifest call per GoP
	// covers both. If they ever diverge we split this in two.
	fetchManifest := func() ([]byte, error) {
		return ms.buildManifest(ctx, time.Now().UnixMilli())
	}
	in := muxl.SignerInput{
		CertPEM:           ms.Cert,
		TrackManifestFn:   fetchManifest,
		WrapperManifestFn: fetchManifest,
	}
	if _, ok := ms.Signer.(*ecdsa.PrivateKey); ok {
		keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
		if err != nil {
			return fmt.Errorf("failed to marshal signing key: %w", err)
		}
		in.KeyPEM = keyPEM
		span.SetAttributes(attribute.String("backend", "pem"))
	} else {
		in.Sign = muxl.SignerToCallback(ms.Signer, 32)
		span.SetAttributes(attribute.String("backend", "host-callback"))
	}

	return muxl.RunMuxlSignSegment(ctx, input, in, nil, nil, eventCh)
}

// don't call externally! this is used as a callback for the rust library

func (ms *MediaSignerLocal) Pub() aqpub.Pub {
	return ms.AQPub
}

func (ms *MediaSignerLocal) DID() string {
	return ms.did
}

type RustCallbackSigner struct {
	Signer crypto.Signer
}

func (rcs *RustCallbackSigner) Sign(data []byte) ([]byte, error) {
	digest := sha256.Sum256(data)
	sig, err := rcs.Signer.Sign(rand.Reader, digest[:], nil)
	if err != nil {
		return nil, fmt.Errorf("failed to sign data: %w", err)
	}
	return sig, nil
}
