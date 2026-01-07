package signers

import (
	gocrypto "crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

var OID_SECP256K1 asn1.ObjectIdentifier = []int{1, 3, 132, 0, 10}

// uses Go code to generate a es256p cert, then rewrites and resigns it into an es256k cert
func GenerateES256KCert(signer gocrypto.Signer) ([]byte, error) {
	keyUsage := x509.KeyUsageDigitalSignature

	notBefore := time.Now()
	notAfter := notBefore.Add((100 * 365 * 24) * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate P256 key: %w", err)
	}

	// pub := priv.Public().(*ecdsa.PublicKey)
	// publicKeyBytes := elliptic.Marshal(elliptic.P256(), pub.X, pub.Y)
	pub := signer.Public().(*ecdsa.PublicKey)
	publicKeyBytes := elliptic.Marshal(crypto.S256(), pub.X, pub.Y) //nolint:all
	atpub, err := atcrypto.ParsePublicUncompressedBytesK256(publicKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}
	idhash := sha1.Sum(publicKeyBytes)
	subjectKeyID := idhash[:]

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   atpub.DIDKey(),
			Organization: []string{"Streamplace"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageEmailProtection},
		BasicConstraintsValid: true,
		SubjectKeyId:          subjectKeyID,
		AuthorityKeyId:        subjectKeyID,
	}

	p256DERBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public(), priv)
	if err != nil {
		return nil, fmt.Errorf("failed to create p256 certificate: %w", err)
	}

	var p256cert certificate
	if _, err := asn1.Unmarshal(p256DERBytes, &p256cert); err != nil {
		return nil, fmt.Errorf("failed to unmarshal p256 cert: %w", err)
	}

	old := p256cert.TBSCertificate

	var paramBytes []byte
	paramBytes, err = asn1.Marshal(OID_SECP256K1)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OID_SECP256K1: %w", err)

	}
	var signingAlg pkix.AlgorithmIdentifier
	signingAlg.Algorithm = old.PublicKey.Algorithm.Algorithm
	signingAlg.Parameters.FullBytes = paramBytes

	k256info := publicKeyInfo{
		Algorithm: signingAlg,
		PublicKey: asn1.BitString{
			Bytes:     publicKeyBytes,
			BitLength: len(publicKeyBytes) * 8,
		},
	}

	tbs := tbsCertificate{
		Version:            old.Version,
		SerialNumber:       old.SerialNumber,
		SignatureAlgorithm: old.SignatureAlgorithm,
		Issuer:             old.Issuer,
		Validity:           old.Validity,
		Subject:            old.Subject,
		PublicKey:          k256info,
		UniqueID:           old.UniqueID,
		SubjectUniqueID:    old.SubjectUniqueID,
		Extensions:         old.Extensions,
	}

	toSign, err := asn1.Marshal(tbs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal p256 cert: %w", err)
	}
	h := gocrypto.SHA256.New()
	h.Write(toSign)
	digest := h.Sum(nil)
	sig, err := signer.Sign(rand.Reader, digest, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to sign k256k cert: %w", err)
	}
	k256cert := certificate{
		TBSCertificate:     tbs,
		SignatureAlgorithm: p256cert.SignatureAlgorithm,
		SignatureValue:     asn1.BitString{Bytes: sig, BitLength: len(sig) * 8},
	}

	k256DERBytes, err := asn1.Marshal(k256cert)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal k256k cert: %w", err)
	}

	bs := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: k256DERBytes})
	return bs, nil
}

func ParseES256KCert(pembs []byte) (*atcrypto.PublicKeyK256, error) {
	// todo: there may be a chain here
	block, _ := pem.Decode(pembs)

	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block containing the cert")
	}

	k256cert := certificate{}
	_, err := asn1.Unmarshal(block.Bytes, &k256cert)
	if err != nil {
		return nil, err
	}

	pub, err := atcrypto.ParsePublicUncompressedBytesK256(k256cert.TBSCertificate.PublicKey.PublicKey.Bytes)
	if err != nil {
		return nil, err
	}

	return pub, nil
}

func HexAddr(pub *ecdsa.PublicKey) string {
	addr := crypto.PubkeyToAddress(*pub)
	hex := hexutil.Encode(addr.Bytes())
	return hex
}

func HexAddrFromSigner(signer gocrypto.Signer) (string, error) {
	pub := signer.Public()
	ecpub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("keypair is not an ecdsa key")
	}
	return HexAddr(ecpub), nil
}

type certificate struct {
	TBSCertificate     tbsCertificate
	SignatureAlgorithm pkix.AlgorithmIdentifier
	SignatureValue     asn1.BitString
}

type tbsCertificate struct {
	Raw                asn1.RawContent
	Version            int `asn1:"optional,explicit,default:0,tag:0"`
	SerialNumber       *big.Int
	SignatureAlgorithm pkix.AlgorithmIdentifier
	Issuer             asn1.RawValue
	Validity           validity
	Subject            asn1.RawValue
	PublicKey          publicKeyInfo
	UniqueID           asn1.BitString   `asn1:"optional,tag:1"`
	SubjectUniqueID    asn1.BitString   `asn1:"optional,tag:2"`
	Extensions         []pkix.Extension `asn1:"omitempty,optional,explicit,tag:3"`
}

type dsaAlgorithmParameters struct {
	P, Q, G *big.Int
}

type validity struct {
	NotBefore, NotAfter time.Time
}

type publicKeyInfo struct {
	Raw       asn1.RawContent
	Algorithm pkix.AlgorithmIdentifier
	PublicKey asn1.BitString
}
