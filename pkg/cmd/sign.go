package cmd

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"io"
	"os"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/mr-tron/base58"
	"stream.place/streamplace/pkg/crypto/aqpub"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
)

func Sign(ctx context.Context, certPath string, key string, streamerName string, taURL string, startTime int64, manifestJSON string) error {
	log.Debug(ctx, "Sign command: starting",
		"streamer", streamerName,
		"startTime", startTime,
		"hasManifest", len(manifestJSON) > 0)

	keyBs, err := base58.Decode(key)
	if err != nil {
		return err
	}

	if streamerName == "" {
		return fmt.Errorf("streamer name is required")
	}

	secpSigner, _ := secp256k1.PrivKeyFromBytes(keyBs)
	if secpSigner == nil {
		return fmt.Errorf("invalid key")
	}
	signer := secpSigner.ToECDSA()

	certBs, err := os.ReadFile(certPath)
	if err != nil {
		return err
	}

	pub, err := aqpub.FromPublicKey(signer.Public().(*ecdsa.PublicKey))
	if err != nil {
		return err
	}

	ms := &media.MediaSignerLocal{
		Signer:           signer,
		Cert:             certBs,
		StreamerName:     streamerName,
		TAURL:            taURL,
		AQPub:            pub,
		PrebuiltManifest: []byte(manifestJSON), // Pass the manifest from parent process
	}

	if len(manifestJSON) > 0 {
		log.Debug(ctx, "Sign command: using provided manifest", "manifestLength", len(manifestJSON))
	}

	inputBs, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}

	mp4, err := ms.SignMP4(ctx, bytes.NewReader(inputBs), startTime)
	if err != nil {
		return err
	}
	_, err = io.Copy(os.Stdout, bytes.NewReader(mp4))
	if err != nil {
		return err
	}

	return nil
}
