package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCheckConfig covers the pure, error-collecting configuration checks:
// every problem is reported at once, nothing is mutated, and a sound
// configuration passes without touching anything.
func TestCheckConfig(t *testing.T) {
	ok := func(c CLI) {
		t.Helper()
		require.NoError(t, c.CheckConfig())
	}
	bad := func(c CLI, want ...string) {
		t.Helper()
		err := c.CheckConfig()
		require.Error(t, err)
		for _, w := range want {
			require.Contains(t, err.Error(), w)
		}
	}

	ok(CLI{DataDir: t.TempDir()})

	bad(CLI{}, "data-dir")
	bad(CLI{DataDir: t.TempDir(), LivepeerGateway: true, LivepeerGatewayURL: "http://127.0.0.1:8935"},
		"livepeer-gateway")
	bad(CLI{DataDir: t.TempDir(), TranscodeRenditions: "not-a-rendition"},
		"--transcode-renditions")
	bad(CLI{DataDir: t.TempDir(),
		FirebaseServiceAccount: "{}", FirebaseServiceAccountFile: "whatever.json"},
		"firebase-service-account")
	// The crashloop case this whole thing exists for: provider flags without
	// the provider.
	bad(CLI{DataDir: t.TempDir(), BunnyTokenAuthKey: "k"},
		"--vod-cdn-provider=bunny")
	// An unreadable firebase file is reported on its own and alongside an
	// unrelated problem, not swallowed by the other failure.
	bad(CLI{DataDir: t.TempDir(),
		FirebaseServiceAccountFile: filepath.Join(t.TempDir(), "nope.json")},
		"--firebase-service-account-file")
	bad(CLI{TranscodeRenditions: "not-a-rendition",
		FirebaseServiceAccountFile: filepath.Join(t.TempDir(), "nope.json")},
		"--transcode-renditions", "--firebase-service-account-file")

	// Multiple problems are reported together, not one at a time.
	multi := CLI{LivepeerGateway: true, LivepeerGatewayURL: "http://127.0.0.1:8935",
		VODCDNProvider: "bunny"}
	err := multi.CheckConfig()
	require.Error(t, err)
	require.Contains(t, err.Error(), "livepeer-gateway")
	require.Contains(t, err.Error(), "vod-cdn-url")
}

// TestCheckConfigSigningKey proves that a broken --signing-key file — a
// classic restart-crashloop cause — is caught by the checks, not discovered
// mid-boot by the app-updates path that reads it.
func TestCheckConfigSigningKey(t *testing.T) {
	dir := t.TempDir()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	good := filepath.Join(dir, "good.pem")
	bs := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	require.NoError(t, os.WriteFile(good, bs, 0o600))
	garbage := filepath.Join(dir, "garbage.pem")
	require.NoError(t, os.WriteFile(garbage, []byte("not a key"), 0o600))

	ok := CLI{DataDir: t.TempDir(), SigningKeyPath: good}
	require.NoError(t, ok.CheckConfig())

	for name, path := range map[string]string{
		"garbage": garbage,
		"missing": filepath.Join(dir, "nope.pem"),
	} {
		cli := CLI{DataDir: t.TempDir(), SigningKeyPath: path}
		err := cli.CheckConfig()
		require.Error(t, err, name)
		require.Contains(t, err.Error(), "--signing-key", name)
	}
}

// TestValidateIsIdempotent pins the reason Validate runs its checks exactly
// once: urfavecli invokes it from both the Before hook and the action path,
// and a second pass must not mistake the defaults PrepareConfig filled in
// (the gateway URL, the firebase account) for operator-set conflicts.
func TestValidateIsIdempotent(t *testing.T) {
	cli := CLI{DataDir: t.TempDir(), LivepeerGateway: true}
	require.NoError(t, cli.Validate(nil))
	require.NoError(t, cli.Validate(nil))
	require.Equal(t, "http://127.0.0.1:8935", cli.LivepeerGatewayURL)

	fbFile := filepath.Join(t.TempDir(), "firebase.json")
	require.NoError(t, os.WriteFile(fbFile, []byte("{}"), 0o600))
	cli = CLI{DataDir: t.TempDir(), FirebaseServiceAccountFile: fbFile}
	require.NoError(t, cli.Validate(nil))
	require.Equal(t, "{}", cli.FirebaseServiceAccount)
}

// TestValidatePreparesDefaults checks that the starting-node path still
// applies the in-memory defaults after the check/prepare split.
func TestValidatePreparesDefaults(t *testing.T) {
	cli := CLI{DataDir: t.TempDir(), BroadcasterHost: "example.com"}
	require.NoError(t, cli.Validate(nil))
	require.Equal(t, []string{ReplicatorWebsocket}, cli.Replicators)
	require.Equal(t, "example.com", cli.ServerHost)
}
