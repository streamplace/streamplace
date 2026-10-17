// Package moq is what Streamplace adds to the official Media over QUIC
// transport (moq.dev/moq): the certificate lifecycle its listener needs,
// and the wire conventions nodes agree on for carrying signed MUXL segments
// (see docs/moq.md). The transport itself, sessions, caching and
// prioritisation included, is the official library's.
package moq

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CertHashParam is the URL query parameter carrying a server certificate's
// SHA-256 (hex) for a subscriber to pin instead of consulting the system
// roots. A node that terminates TLS elsewhere has no publicly trusted
// certificate of its own to serve QUIC with, so it serves a self-signed
// one and advertises its hash alongside its URL. The value is exactly the
// fingerprint the official transport pins (moq.WithTLSFingerprints).
const CertHashParam = "certhash"

// Browsers accept a pinned certificate (serverCertificateHashes) only if it
// is ECDSA and valid for at most two weeks, so a self-signed one is minted
// on that schedule and rotated at half its life.
const (
	selfSignedLifetime = 14 * 24 * time.Hour
	selfSignedRotate   = 7 * 24 * time.Hour
)

// SelfSignedCert owns the self-signed identity copied to the listener's files.
// Certificate rotates it; Hash only reads the most recently prepared identity.
// The native file watcher reloads asynchronously, so peers refresh discovery on
// reconnect when they encounter a certificate rotation.
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

// Hash is the SHA-256 (hex) of the most recently prepared leaf certificate.
func (c *SelfSignedCert) Hash() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hash
}

// Certificate is the certificate to serve right now, for TLSFiles.
func (c *SelfSignedCert) Certificate() (*tls.Certificate, error) {
	cert, _, err := c.current()
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

func certHash(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

// tlsRefreshInterval is how often TLSFiles.Refresh compares the source
// certificate with the files on disk. ACME renews weeks ahead of expiry
// and the self-signed certificate rotates a week ahead of its own, so a
// minute of lag is nothing; a tighter loop would only re-read the source.
const tlsRefreshInterval = time.Minute

// TLSFiles is the identity the listener serves, as the PEM files the
// official transport reads: it takes certificate and key paths, watches
// their directory and reloads them for new connections when they are
// replaced, keeping the last valid pair through a half-written rotation.
// Go owns the certificate lifecycle (ACME, the self-signed rotation, or
// files an operator manages); the transport is only ever told where the
// current pair lives, so nothing has to inspect the listener's identity
// while it is accepting.
type TLSFiles struct {
	CertPath string
	KeyPath  string

	// source, when set, is where the pair comes from; the files are
	// rewritten whenever its leaf changes. Nil for operator-managed files.
	source func() (*tls.Certificate, error)
	mu     sync.Mutex
	leaf   string // hash of the leaf last written
}

// StaticTLSFiles serves a pair someone else maintains (--tls-cert and
// --tls-key); the transport picks up a replacement on its own.
func StaticTLSFiles(certPath, keyPath string) *TLSFiles {
	return &TLSFiles{CertPath: certPath, KeyPath: keyPath}
}

// ManagedTLSFiles writes the certificate source yields into dir on every
// Sync, and on every Refresh tick that finds a new leaf. It writes nothing
// yet: a node's first ACME certificate may still be on its way. dir is
// created if missing; the key file is private to the process owner.
func ManagedTLSFiles(dir string, source func() (*tls.Certificate, error)) (*TLSFiles, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating moq tls directory: %w", err)
	}
	return &TLSFiles{
		CertPath: filepath.Join(dir, "tls.crt"),
		KeyPath:  filepath.Join(dir, "tls.key"),
		source:   source,
	}, nil
}

// Sync writes the current pair if its leaf differs from the one on disk.
// The key lands before the certificate, each atomically (a temporary file
// renamed into place, which is what the transport's watcher reacts to): a
// watcher that reloads between the two sees a mismatched pair, keeps the
// previous identity, and reloads again on the certificate.
func (f *TLSFiles) Sync() error {
	if f.source == nil {
		return nil
	}
	cert, err := f.source()
	if err != nil {
		return fmt.Errorf("moq tls certificate: %w", err)
	}
	if len(cert.Certificate) == 0 {
		return errors.New("moq tls certificate: empty chain")
	}
	leaf := certHash(cert.Certificate[0])
	f.mu.Lock()
	defer f.mu.Unlock()
	if leaf == f.leaf {
		return nil
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		return fmt.Errorf("moq tls key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	var chainPEM []byte
	for _, der := range cert.Certificate {
		chainPEM = append(chainPEM, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})...)
	}
	if err := writeAtomic(f.KeyPath, keyPEM, 0o600); err != nil {
		return err
	}
	if err := writeAtomic(f.CertPath, chainPEM, 0o644); err != nil {
		return err
	}
	f.leaf = leaf
	return nil
}

// Refresh keeps the files current until ctx ends. A failed sync is
// retried on the next tick; the previous pair stays in service meanwhile.
func (f *TLSFiles) Refresh(ctx context.Context, onError func(error)) {
	if f.source == nil {
		return
	}
	ticker := time.NewTicker(tlsRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := f.Sync(); err != nil && onError != nil {
				onError(err)
			}
		}
	}
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(name, mode)
	}
	if werr == nil {
		werr = os.Rename(name, path)
	}
	if werr != nil {
		_ = os.Remove(name)
		return fmt.Errorf("writing %s: %w", path, werr)
	}
	return nil
}
