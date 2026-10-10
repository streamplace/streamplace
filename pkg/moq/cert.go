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

// SelfSignedCert is a rotating self-signed server certificate. The served
// certificate and the advertised hash change together, under one lock,
// whichever of them notices the week is up; the origin record and the
// statedb row re-read Hash every few seconds, so in practice the new hash
// is advertised before any handshake sees the new certificate, and a
// peer that pinned the old one reconnects with the refreshed URL on its
// next pull attempt.
type SelfSignedCert struct {
	mu   sync.Mutex
	cert tls.Certificate
	hash string
	exp  time.Time
}

// NewSelfSignedCert mints the first certificate.
func NewSelfSignedCert() (*SelfSignedCert, error) {
	c := &SelfSignedCert{}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.mint(); err != nil {
		return nil, err
	}
	return c, nil
}

// mint replaces the certificate; the caller holds mu.
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
	c.cert = tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	c.hash = certHash(der)
	c.exp = tmpl.NotAfter
	return nil
}

// current returns the certificate to serve and advertise, re-minting it
// once it is a week old. A mint failure keeps the old one in service.
func (c *SelfSignedCert) current() (tls.Certificate, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var err error
	if time.Until(c.exp) < selfSignedLifetime-selfSignedRotate {
		err = c.mint()
	}
	return c.cert, c.hash, err
}

// Hash is the SHA-256 (hex) of the certificate currently served.
func (c *SelfSignedCert) Hash() string {
	_, hash, _ := c.current()
	return hash
}

// TLSConfig serves the certificate.
func (c *SelfSignedCert) TLSConfig() *tls.Config {
	return &tls.Config{
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			cert, _, err := c.current()
			if err != nil {
				return nil, err
			}
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
