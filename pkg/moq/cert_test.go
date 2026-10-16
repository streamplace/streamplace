package moq

import (
	"crypto/tls"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The advertised hash is the fingerprint the transport pins: the SHA-256
// of the leaf's DER, hex. The files a listener reads carry exactly the
// certificate the hash names, and a rotation rewrites them with the new
// one while the hash changes with it.
func TestManagedTLSFilesFollowRotation(t *testing.T) {
	cert, err := NewSelfSignedCert()
	require.NoError(t, err)
	dir := filepath.Join(t.TempDir(), "moq")
	files, err := ManagedTLSFiles(dir, cert.Certificate)
	require.NoError(t, err)

	leaf := func() string {
		pemBytes, err := os.ReadFile(files.CertPath)
		require.NoError(t, err)
		block, _ := pem.Decode(pemBytes)
		require.NotNil(t, block)
		return certHash(block.Bytes)
	}
	require.Equal(t, cert.Hash(), leaf())
	keyInfo, err := os.Stat(files.KeyPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), keyInfo.Mode().Perm())
	_, err = tls.LoadX509KeyPair(files.CertPath, files.KeyPath)
	require.NoError(t, err)

	// Age the certificate past the rotation point. Reading the advertised
	// hash alone must not rotate an identity the listener has not prepared.
	cert.mu.Lock()
	cert.exp = time.Now().Add(selfSignedLifetime - selfSignedRotate - time.Hour)
	cert.mu.Unlock()
	before := cert.Hash()
	require.Equal(t, leaf(), before)
	require.NoError(t, files.Sync())
	require.NotEqual(t, before, leaf())
	require.Equal(t, cert.Hash(), leaf())
	require.NoError(t, files.Sync(), "an unchanged leaf is not rewritten")
}
