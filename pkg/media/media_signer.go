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

	"git.aquareum.tv/streamplace/c2pa-go/pkg/c2pa"
	"stream.place/streamplace/pkg/aqio"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/aqpub"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
)

type MediaSigner struct {
	StreamerName string
	Signer       crypto.Signer
	Pub          aqpub.Pub
	Cert         []byte
	TAURL        string
	Model        model.Model
}

func MakeMediaSigner(ctx context.Context, cli *config.CLI, streamer string, signer crypto.Signer, mod model.Model) (*MediaSigner, error) {
	pub, err := aqpub.FromPublicKey(signer.Public().(*ecdsa.PublicKey))
	if err != nil {
		return nil, err
	}
	exists, err := cli.DataFileExists([]string{pub.String(), CERT_FILE})
	if err != nil {
		return nil, err
	}
	if !exists {
		cert, err := signers.GenerateES256KCert(signer)
		if err != nil {
			return nil, err
		}
		r := bytes.NewReader(cert)
		err = cli.DataFileWrite([]string{pub.String(), CERT_FILE}, r, false)
		if err != nil {
			return nil, err
		}
		log.Log(ctx, "wrote new media signing certificate", "file", filepath.Join(pub.String(), CERT_FILE))
	}
	buf := bytes.Buffer{}
	cli.DataFileRead([]string{pub.String(), CERT_FILE}, &buf)
	cert := buf.Bytes()
	return &MediaSigner{
		// cli:        cli,
		Signer:       signer,
		Cert:         cert,
		StreamerName: streamer,
		TAURL:        cli.TAURL,
		Pub:          pub,
		Model:        mod,
	}, nil
}

func (ms *MediaSigner) SignMP4(ctx context.Context, input io.ReadSeeker, start int64) ([]byte, error) {
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
				"label": STREAMPLACE_METADATA,
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
	manifestBs, err := json.Marshal(mani)
	if err != nil {
		return nil, err
	}
	var manifest c2pa.ManifestDefinition
	err = json.Unmarshal(manifestBs, &manifest)
	if err != nil {
		return nil, err
	}
	alg, err := c2pa.GetSigningAlgorithm(string(c2pa.ES256K))
	if err != nil {
		return nil, err
	}
	b, err := c2pa.NewBuilder(&manifest, &c2pa.BuilderParams{
		Cert:      ms.Cert,
		Signer:    ms.Signer,
		Algorithm: alg,
		TAURL:     ms.TAURL,
	})
	if err != nil {
		return nil, err
	}

	output := &aqio.ReadWriteSeeker{}
	err = b.Sign(input, output, "video/mp4")
	if err != nil {
		return nil, err
	}
	bs, err := output.Bytes()
	if err != nil {
		return nil, err
	}
	return bs, nil
}
