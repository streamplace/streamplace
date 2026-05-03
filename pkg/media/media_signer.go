package media

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/atproto"
	c2patypes "stream.place/streamplace/pkg/c2patypes"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/aqpub"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/iroh/generated/iroh_streamplace"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/muxl"
	"stream.place/streamplace/pkg/spmetrics"
)

var signerTracer = otel.Tracer("signer")

type MediaSigner interface {
	SignMP4(ctx context.Context, input io.ReadSeeker, start int64) ([]byte, error)
	Pub() aqpub.Pub
	Streamer() string
	DID() string
	SignConcatMP4(ctx context.Context, input io.ReadSeeker, ingredients []io.ReadSeeker, output io.ReadWriteSeeker) error
}

var DoReplay = false

type MediaSignerLocal struct {
	StreamerName     string
	Signer           crypto.Signer
	AQPub            aqpub.Pub
	Cert             []byte
	TAURL            string
	did              string
	manifestBuilder  *ManifestBuilder
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

func (ms *MediaSignerLocal) SignMP4(ctx context.Context, input io.ReadSeeker, start int64) ([]byte, error) {
	startTime := time.Now()
	ctx, span := signerTracer.Start(ctx, "SignMP4", trace.WithAttributes(
		attribute.String("streamer", ms.StreamerName),
		attribute.Int64("segment_start_ms", start),
	))
	defer span.End()

	// --- 1. Build manifest. -------------------------------------------------
	var manifestBs []byte
	var err error
	switch {
	case len(ms.PrebuiltManifest) > 0:
		manifestBs = ms.PrebuiltManifest
		span.AddEvent("manifest: prebuilt", trace.WithAttributes(attribute.Int("bytes", len(manifestBs))))
		log.Debug(ctx, "SignMP4: using prebuilt manifest", "manifestLength", len(manifestBs))
	case ms.manifestBuilder != nil:
		_, span2 := signerTracer.Start(ctx, "SignMP4.BuildManifest")
		manifestBs, err = ms.manifestBuilder.BuildManifest(ctx, ms.StreamerName, start)
		if manifestBs != nil {
			span2.SetAttributes(attribute.Int("bytes", len(manifestBs)))
		}
		span2.End()
		if err != nil {
			return nil, fmt.Errorf("failed to build manifest: %w", err)
		}
	default:
		log.Warn(ctx, "SignMP4: manifestBuilder is nil, using fallback manifest - this indicates model was not passed to MakeMediaSigner", "streamer", ms.StreamerName)
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
		manifestBs, err = json.Marshal(mani)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal basic manifest: %w", err)
		}
		span.AddEvent("manifest: fallback", trace.WithAttributes(attribute.Int("bytes", len(manifestBs))))
	}

	// --- 2. Read segment bytes. ---------------------------------------------
	_, readSpan := signerTracer.Start(ctx, "SignMP4.ReadInput")
	bs, err := io.ReadAll(input)
	readSpan.SetAttributes(attribute.Int("bytes", len(bs)))
	readSpan.End()
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}

	// --- 3. Pick signing backend. -------------------------------------------
	signerInput := muxl.SignerInput{
		Segment:         bs,
		CertPEM:         ms.Cert,
		TrackManifest:   manifestBs,
		WrapperManifest: manifestBs,
	}
	if _, ok := ms.Signer.(*ecdsa.PrivateKey); ok {
		_, marshalSpan := signerTracer.Start(ctx, "SignMP4.MarshalKeyPEM")
		keyPEM, err := signers.MarshalES256KPrivateKeyPEM(ms.Signer)
		marshalSpan.End()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal signing key: %w", err)
		}
		signerInput.KeyPEM = keyPEM
		span.SetAttributes(attribute.String("backend", "pem"))
	} else {
		signerInput.Sign = muxl.SignerToCallback(ms.Signer, 32)
		span.SetAttributes(
			attribute.String("backend", "host-callback"),
			attribute.String("signer_type", fmt.Sprintf("%T", ms.Signer)),
		)
	}

	// --- 4. Sign via wasm. --------------------------------------------------
	signed, err := muxl.RunMuxlSigner(ctx, signerInput)
	if err != nil {
		return nil, fmt.Errorf("muxl-sign failed: %w", err)
	}
	span.SetAttributes(attribute.Int("signed_bytes", len(signed)))

	spmetrics.SigningDuration.WithLabelValues(ms.StreamerName).Observe(float64(time.Since(startTime).Milliseconds()))
	return signed, nil
}

func (ms *MediaSignerLocal) SignConcatMP4(ctx context.Context, input io.ReadSeeker, ingredients []io.ReadSeeker, output io.ReadWriteSeeker) error {
	startTime := time.Now()
	ctx, span := otel.Tracer("signer").Start(ctx, "SignMP4")
	defer span.End()
	// for _, ingredient := range ingredients {
	// 	_, err := iroh_streamplace.GetManifestAndCert(c2patypes.NewReader(aqio.NewReadWriteSeeker(ingredient)))
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }
	// title := "livestream"
	mani := obj{
		"title": "Livestream Clip",
		// "assertions": []obj{
		// 	{
		// 		"label": "c2pa.actions",
		// 		"data": obj{
		// 			"actions": []obj{
		// 				{"action": "c2pa.created"},
		// 				{"action": "c2pa.published"},
		// 			},
		// 		},
		// 	},
		// 	{
		// 		"label": StreamplaceMetadata,
		// 		"data": obj{
		// 			"@context": obj{
		// 				"dc": "http://purl.org/dc/elements/1.1/",
		// 			},
		// 			"dc:creator": ms.StreamerName,
		// 			"dc:title":   []string{title},
		// 			"dc:date":    []string{aqtime.FromMillis(start).String()},
		// 		},
		// 	},
		// },
	}
	ctx, span = otel.Tracer("signer").Start(ctx, "SignMP4_MarshalManifest")
	manifestBs, err := json.Marshal(mani)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}
	var manifest c2patypes.ManifestDefinition
	err = json.Unmarshal(manifestBs, &manifest)
	if err != nil {
		return fmt.Errorf("failed to unmarshal manifest: %w", err)
	}
	span.End()

	ctx, span = otel.Tracer("signer").Start(ctx, "SignMP4_Sign")
	rustCallbackSigner := &RustCallbackSigner{
		Signer: ms.Signer,
	}
	many := c2patypes.NewManyStreams()
	for _, ingredient := range ingredients {
		many.AddStream(ingredient)
	}
	err = iroh_streamplace.SignWithIngredients(string(manifestBs), c2patypes.NewReader(input), base64.StdEncoding.EncodeToString(ms.Cert), many, rustCallbackSigner, c2patypes.NewWriter(output))
	if err != nil {
		return err
	}
	span.End()

	ctx, span = otel.Tracer("signer").Start(ctx, "SignMP4_OutputBytes")
	defer ctx.Done()
	if err != nil {
		return fmt.Errorf("failed to get output bytes: %w", err)
	}
	span.End()
	spmetrics.SigningDuration.WithLabelValues(ms.StreamerName).Observe(float64(time.Since(startTime).Milliseconds()))
	return nil
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
