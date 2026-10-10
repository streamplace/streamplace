package moq

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// CertHashParam is the URL query parameter carrying a server certificate's
// SHA-256 (hex) for a subscriber to pin instead of consulting the system
// roots. A node that terminates TLS elsewhere has no publicly trusted
// certificate of its own to serve QUIC with, so it serves a self-signed
// one and advertises its hash alongside its URL.
const CertHashParam = "certhash"

// Browsers accept a pinned certificate (serverCertificateHashes) only if it
// is ECDSA and valid for at most two weeks, so a self-signed one is minted
// on that schedule and rotated at half its life.
const (
	selfSignedLifetime = 14 * 24 * time.Hour
	selfSignedRotate   = 7 * 24 * time.Hour
)

// SelfSignedCert is a rotating self-signed server certificate: TLSConfig
// always serves a certificate with at least a week left, and Hash names the
// one being served right now.
type SelfSignedCert struct {
	mu   sync.Mutex
	cert tls.Certificate
	hash string
	exp  time.Time
}

// NewSelfSignedCert mints the first certificate.
func NewSelfSignedCert() (*SelfSignedCert, error) {
	c := &SelfSignedCert{}
	if err := c.mint(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *SelfSignedCert) mint() error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "streamplace moq"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(selfSignedLifetime - time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("minting self-signed certificate: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cert = tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	c.hash = certHash(der)
	c.exp = tmpl.NotAfter
	return nil
}

// Hash is the SHA-256 (hex) of the certificate currently served.
func (c *SelfSignedCert) Hash() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hash
}

// TLSConfig serves the certificate, re-minting it once it is a week old.
func (c *SelfSignedCert) TLSConfig() *tls.Config {
	return &tls.Config{
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			c.mu.Lock()
			rotate := time.Until(c.exp) < selfSignedLifetime-selfSignedRotate
			c.mu.Unlock()
			if rotate {
				if err := c.mint(); err != nil {
					return nil, err
				}
			}
			c.mu.Lock()
			defer c.mu.Unlock()
			cert := c.cert
			return &cert, nil
		},
	}
}

func certHash(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

// PinnedTLSConfig trusts exactly the server certificate with the given
// SHA-256 (hex), whatever it says about itself.
func PinnedTLSConfig(hash string) *tls.Config {
	return &tls.Config{
		// Verification is replaced, not skipped: VerifyPeerCertificate
		// below still runs and rejects anything but the pinned leaf.
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return errors.New("moq: server presented no certificate")
			}
			if got := certHash(rawCerts[0]); got != hash {
				return fmt.Errorf("moq: server certificate %s is not the pinned %s", got, hash)
			}
			return nil
		},
	}
}
