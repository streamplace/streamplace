package cmd

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/mr-tron/base58"
	"stream.place/streamplace/pkg/crypto/aqpub"
	"stream.place/streamplace/pkg/media"
)

func Sign(ctx context.Context) error {
	fs := flag.NewFlagSet("streamplace", flag.ExitOnError)
	certPath := fs.String("cert", "", "path to the certificate file")
	key := fs.String("key", "", "base58-encoded secp256k1 private key")
	streamerName := fs.String("streamer", "", "streamer name")
	taURL := fs.String("ta-url", "http://timestamp.digicert.com", "timestamp authority server for signing")
	startTime := fs.Int64("start-time", 0, "start time of the stream")
	fs.Parse(os.Args[2:])

	keyBs, err := base58.Decode(*key)
	if err != nil {
		return err
	}

	secpSigner, _ := secp256k1.PrivKeyFromBytes(keyBs)
	if secpSigner == nil {
		return fmt.Errorf("invalid key")
	}
	signer := secpSigner.ToECDSA()

	certBs, err := os.ReadFile(*certPath)
	if err != nil {
		return err
	}

	pub, err := aqpub.FromPublicKey(signer.Public().(*ecdsa.PublicKey))

	ms := &media.MediaSignerLocal{
		Signer:       signer,
		Cert:         certBs,
		StreamerName: *streamerName,
		TAURL:        *taURL,
		AQPub:        pub,
	}

	inputBs, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}

	mp4, err := ms.SignMP4(ctx, bytes.NewReader(inputBs), *startTime)
	io.Copy(os.Stdout, bytes.NewReader(mp4))

	return nil
}
