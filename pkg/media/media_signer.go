package media

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"git.stream.place/streamplace/c2pa-go/pkg/c2pa"
	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/aqio"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/aqpub"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/spmetrics"
)

type MediaSigner interface {
	SignMP4(ctx context.Context, input io.ReadSeeker, start int64) ([]byte, error)
	Pub() aqpub.Pub
	Streamer() string
}

type MediaSignerLocal struct {
	StreamerName string
	Signer       crypto.Signer
	AQPub        aqpub.Pub
	Cert         []byte
	TAURL        string
}

func prepareCert(ctx context.Context, cli *config.CLI, signer crypto.Signer) ([]byte, string, error) {
	pub, err := aqpub.FromPublicKey(signer.Public().(*ecdsa.PublicKey))
	if err != nil {
		return nil, "", err
	}
	fSlice := []string{pub.String(), CertFile}
	exists, err := cli.DataFileExists(fSlice)
	if err != nil {
		return nil, "", err
	}
	if !exists {
		cert, err := signers.GenerateES256KCert(signer)
		if err != nil {
			return nil, "", err
		}
		r := bytes.NewReader(cert)
		err = cli.DataFileWrite(fSlice, r, false)
		if err != nil {
			return nil, "", err
		}
		log.Log(ctx, "wrote new media signing certificate", "file", filepath.Join(pub.String(), CertFile))
	}
	buf := bytes.Buffer{}
	if err := cli.DataFileRead(fSlice, &buf); err != nil {
		return nil, "", err
	}

	fPath := cli.DataFilePath(fSlice)
	cert := buf.Bytes()
	return cert, fPath, nil
}

func MakeMediaSigner(ctx context.Context, cli *config.CLI, streamer string, signer crypto.Signer) (MediaSigner, error) {
	cert, _, err := prepareCert(ctx, cli, signer)
	if err != nil {
		return nil, err
	}
	pub, err := aqpub.FromPublicKey(signer.Public().(*ecdsa.PublicKey))
	if err != nil {
		return nil, err
	}
	return &MediaSignerLocal{
		Signer:       signer,
		Cert:         cert,
		StreamerName: streamer,
		TAURL:        cli.TAURL,
		AQPub:        pub,
	}, nil
}

func (ms *MediaSignerLocal) Streamer() string {
	return ms.StreamerName
}

func (ms *MediaSignerLocal) SignMP4(ctx context.Context, input io.ReadSeeker, start int64) ([]byte, error) {
	startTime := time.Now()
	ctx, span := otel.Tracer("signer").Start(ctx, "SignMP4")
	defer span.End()
	title := "livestream"
	mani := obj{
		"title": fmt.Sprintf("Livestream Segment at %s", aqtime.FromMillis(start)),
		"assertions": []obj{
			{
				"label": "c2pa.actions",
				"data": obj{
					"actions": []obj{
						{"action": "c2pa.created"},
						{"action": "c2pa.published"},
					},
				},
			},
			{
				"label": StreamplaceMetadata,
				"data": obj{
					"@context": obj{
						"dc": "http://purl.org/dc/elements/1.1/",
					},
					"dc:creator": ms.StreamerName,
					"dc:title":   []string{title},
					"dc:date":    []string{aqtime.FromMillis(start).String()},
				},
			},
		},
	}
	ctx, span = otel.Tracer("signer").Start(ctx, "SignMP4_MarshalManifest")
	manifestBs, err := json.Marshal(mani)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest: %w", err)
	}
	var manifest c2pa.ManifestDefinition
	err = json.Unmarshal(manifestBs, &manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}
	span.End()

	ctx, span = otel.Tracer("signer").Start(ctx, "SignMP4_GetSigningAlgorithm")
	alg, err := c2pa.GetSigningAlgorithm(string(c2pa.ES256K))
	if err != nil {
		return nil, fmt.Errorf("failed to get signing algorithm: %w", err)
	}
	span.End()

	ctx, span = otel.Tracer("signer").Start(ctx, "SignMP4_NewBuilder")
	b, err := c2pa.NewBuilder(&manifest, &c2pa.BuilderParams{
		Cert:      ms.Cert,
		Signer:    ms.Signer,
		Algorithm: alg,
		TAURL:     ms.TAURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create C2PA builder: %w", err)
	}
	span.End()

	ctx, span = otel.Tracer("signer").Start(ctx, "SignMP4_Sign")
	output := &aqio.ReadWriteSeeker{}
	err = b.Sign(input, output, "video/mp4")
	if err != nil {
		return nil, fmt.Errorf("failed to sign MP4: %w", err)
	}
	span.End()

	ctx, span = otel.Tracer("signer").Start(ctx, "SignMP4_OutputBytes")
	defer ctx.Done()
	bs, err := output.Bytes()
	if err != nil {
		return nil, fmt.Errorf("failed to get output bytes: %w", err)
	}
	span.End()
	spmetrics.SigningDuration.WithLabelValues(ms.StreamerName).Observe(float64(time.Since(startTime).Milliseconds()))
	return bs, nil
}

func (ms *MediaSignerLocal) Pub() aqpub.Pub {
	return ms.AQPub
}
