package media

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/mr-tron/base58"
	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/aqpub"
	"stream.place/streamplace/pkg/spmetrics"
)

type MediaSignerExt struct {
	cli      *config.CLI
	signer   crypto.Signer
	pub      aqpub.Pub
	certPath string
	streamer string
	keyBs    []byte
	taURL    string
	did      string
}

func MakeMediaSignerExt(ctx context.Context, cli *config.CLI, streamer string, keyBs []byte) (MediaSigner, error) {
	key, _ := secp256k1.PrivKeyFromBytes(keyBs)
	if key == nil {
		return nil, fmt.Errorf("invalid authorization key (not valid secp256k1)")
	}
	var signer crypto.Signer = key.ToECDSA()
	_, certPath, err := prepareCert(ctx, cli, signer)
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
	return &MediaSignerExt{
		// cli:        cli,
		signer:   signer,
		certPath: certPath,
		streamer: streamer,
		pub:      pub,
		keyBs:    keyBs,
		taURL:    cli.TAURL,
		did:      did.DIDKey(),
	}, nil
}

func (ms *MediaSignerExt) SignMP4(ctx context.Context, input io.ReadSeeker, start int64) ([]byte, error) {
	startTime := time.Now()
	_, span := otel.Tracer("signer").Start(ctx, "SignMP4_Ext")
	defer span.End()
	// Get the path to the current executable
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	enc := base58.Encode(ms.keyBs)

	// Prepare command
	cmd := exec.Command(execPath, "sign",
		"--key", enc,
		"--cert", ms.certPath,
		"--ta-url", ms.taURL,
		"--streamer", ms.streamer,
		"--start-time", fmt.Sprintf("%d", start))

	// overwrite so that our subprocesses don't do their own leak checking
	cmd.Env = append(os.Environ(), "LD_PRELOAD=")

	// Set up pipes for stdin and stdout
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout := &bytes.Buffer{}
	cmd.Stdout = stdout
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	// Copy input to stdin
	_, err = io.Copy(stdin, input)
	if err != nil {
		return nil, fmt.Errorf("failed to write to stdin: %w stderr=%s", err, stderr.String())
	}
	stdin.Close()

	// Wait for the command to complete
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("command failed: %w, stderr: %s", err, stderr.String())
	}
	spmetrics.SigningDuration.WithLabelValues(ms.streamer).Observe(float64(time.Since(startTime).Milliseconds()))
	return stdout.Bytes(), nil
}

func (ms *MediaSignerExt) Pub() aqpub.Pub {
	return ms.pub
}

func (ms *MediaSignerExt) Streamer() string {
	return ms.streamer
}

func (ms *MediaSignerExt) DID() string {
	return ms.did
}
