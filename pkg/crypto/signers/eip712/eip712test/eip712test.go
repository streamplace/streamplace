package eip712test

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"stream.place/streamplace/pkg/crypto/signers/eip712"
	v0 "stream.place/streamplace/pkg/schema/v0"
)

// package for setting up a test wallet

var CertBytes = []byte(`-----BEGIN CERTIFICATE-----
MIIChDCCAiugAwIBAgIUBW/ByXEeQ0Qpgc6G1HYKjM2j6JcwCgYIKoZIzj0EAwIw
gYwxCzAJBgNVBAYTAlVTMQswCQYDVQQIDAJDQTESMBAGA1UEBwwJU29tZXdoZXJl
MScwJQYDVQQKDB5DMlBBIFRlc3QgSW50ZXJtZWRpYXRlIFJvb3QgQ0ExGTAXBgNV
BAsMEEZPUiBURVNUSU5HX09OTFkxGDAWBgNVBAMMD0ludGVybWVkaWF0ZSBDQTAe
Fw0yNDA4MTEyMzM0NTZaFw0zNDA4MDkyMzM0NTZaMIGAMQswCQYDVQQGEwJVUzEL
MAkGA1UECAwCQ0ExEjAQBgNVBAcMCVNvbWV3aGVyZTEfMB0GA1UECgwWQzJQQSBU
ZXN0IFNpZ25pbmcgQ2VydDEZMBcGA1UECwwQRk9SIFRFU1RJTkdfT05MWTEUMBIG
A1UEAwwLQzJQQSBTaWduZXIwVjAQBgcqhkjOPQIBBgUrgQQACgNCAAR1RJfnhmsE
HUATmWV+p0fuOPl+G0TwZ5ZisGwWFA/J+fD6wjP6mW44Ob3TTMLMCCFfy5Gl5Cju
XJru19UH0wVLo3gwdjAMBgNVHRMBAf8EAjAAMBYGA1UdJQEB/wQMMAoGCCsGAQUF
BwMEMA4GA1UdDwEB/wQEAwIGwDAdBgNVHQ4EFgQUoEZwqyiVTYCOTjxn9MeCBDvk
hecwHwYDVR0jBBgwFoAUP9auno3ORuwY1JnRQHu3RCiWgi0wCgYIKoZIzj0EAwID
RwAwRAIgaOz0GFjrKWJMs2epuDqUOis7MsH0ivrPfonvwapYpfYCIBqOURwT+pYf
W0VshLAxI/iVw/5eVXtDPZzCX0b0xq3f
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIICZTCCAgygAwIBAgIUIiJUPMeqKEyhrHFdKsVYF6STAqAwCgYIKoZIzj0EAwIw
dzELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAkNBMRIwEAYDVQQHDAlTb21ld2hlcmUx
GjAYBgNVBAoMEUMyUEEgVGVzdCBSb290IENBMRkwFwYDVQQLDBBGT1IgVEVTVElO
R19PTkxZMRAwDgYDVQQDDAdSb290IENBMB4XDTI0MDgxMTIzMzQ1NloXDTM0MDgw
OTIzMzQ1NlowgYwxCzAJBgNVBAYTAlVTMQswCQYDVQQIDAJDQTESMBAGA1UEBwwJ
U29tZXdoZXJlMScwJQYDVQQKDB5DMlBBIFRlc3QgSW50ZXJtZWRpYXRlIFJvb3Qg
Q0ExGTAXBgNVBAsMEEZPUiBURVNUSU5HX09OTFkxGDAWBgNVBAMMD0ludGVybWVk
aWF0ZSBDQTBWMBAGByqGSM49AgEGBSuBBAAKA0IABMi5X2ELOtZ2i19DplQKEgAf
Et6eCXpF+s4M57ak7Rd+1LzpQ+hlRXzvrpW6hLiO+ZaRTmQyqozgWwOBUm52rT2j
YzBhMA8GA1UdEwEB/wQFMAMBAf8wDgYDVR0PAQH/BAQDAgGGMB0GA1UdDgQWBBQ/
1q6ejc5G7BjUmdFAe7dEKJaCLTAfBgNVHSMEGDAWgBSloXNM8yfsV/w3xH7H3pfj
cfWj6jAKBggqhkjOPQQDAgNHADBEAiBievQIsuEy1I3p5XNtpHZ3MBifukoYwo/a
4ZXq8/VK7wIgMseui+Y0BFyDd+d3vd5Jy4d3uhpho6aNFln0qHbhFr8=
-----END CERTIFICATE-----`)

// eth account 0x090C60a4edC5A0078c67542dEf1441B62eaB3B27
var KeyBytes = []byte(`-----BEGIN PRIVATE KEY-----
MIGEAgEAMBAGByqGSM49AgEGBSuBBAAKBG0wawIBAQQgKJyB05ZmsgeVQ/291hKX
mLsopnxVDVAEYoL1vL1jglahRANCAAR1RJfnhmsEHUATmWV+p0fuOPl+G0TwZ5Zi
sGwWFA/J+fD6wjP6mW44Ob3TTMLMCCFfy5Gl5CjuXJru19UH0wVL
-----END PRIVATE KEY-----`)

// creates a test wallet, cleaned up after the function ends
func WithTestSigner(fn func(*eip712.EIP712Signer)) {
	dname, err := os.MkdirTemp("", "sampledir")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dname)
	ks := keystore.NewKeyStore(dname, keystore.StandardScryptN, keystore.StandardScryptP)
	key, err := parsePrivateKey(KeyBytes)
	if err != nil {
		panic(err)
	}
	acct, err := ks.ImportECDSA(key.(*ecdsa.PrivateKey), "aquareumaquareum")
	if err != nil {
		panic(err)
	}
	schema, err := v0.MakeV0Schema()
	if err != nil {
		panic(err)
	}
	signer, err := eip712.MakeEIP712Signer(context.Background(), &eip712.EIP712SignerOptions{
		EthKeystorePassword: "aquareumaquareum",
		EthKeystorePath:     dname,
		EthAccountAddr:      acct.Address.Hex(),
		Schema:              schema,
	})
	if err != nil {
		panic(err)
	}
	fn(signer)
}

func parsePrivateKey(pemBs []byte) (crypto.Signer, error) {
	block, _ := pem.Decode(pemBs)

	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block containing the private key")
	}
	der := block.Bytes
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}

	if key, err := x509.ParseECPrivateKey(der); err == nil {
		return key, nil
	}

	key, err := x509.ParsePKCS8PrivateKey(der)
	if err == nil {
		switch key := key.(type) {
		case *rsa.PrivateKey:
			return key, nil
		case *ecdsa.PrivateKey:
			return key, nil
		case ed25519.PrivateKey:
			return &key, nil
		default:
			return nil, errors.New("crypto/tls: found unknown private key type in PKCS#8 wrapping")
		}
	}

	// Last resort... handle some key types Go doesn't know about.
	return parsePKCS8PrivateKey(der)
}

var OID_RSA_PSS asn1.ObjectIdentifier = []int{1, 2, 840, 113549, 1, 1, 10} //nolint:all
var OID_EC asn1.ObjectIdentifier = []int{1, 2, 840, 10045, 2, 1}           //nolint:all
var OID_SECP256K1 asn1.ObjectIdentifier = []int{1, 3, 132, 0, 10}          //nolint:all

func parsePKCS8PrivateKey(der []byte) (crypto.Signer, error) {
	var privKey pkcs8
	_, err := asn1.Unmarshal(der, &privKey)
	if err != nil {
		return nil, fmt.Errorf("asn1.Unmarshal failed: %s", err.Error())
	}
	if reflect.DeepEqual(privKey.Algo.Algorithm, OID_RSA_PSS) {
		return x509.ParsePKCS1PrivateKey(privKey.PrivateKey)
	} else if reflect.DeepEqual(privKey.Algo.Algorithm, OID_EC) {
		return parseES256KPrivateKey(privKey)
	} else {
		return nil, fmt.Errorf("unknown pkcs8 OID: %s", privKey.Algo.Algorithm)
	}
}

func parseES256KPrivateKey(privKey pkcs8) (crypto.Signer, error) {
	var namedCurveOID asn1.ObjectIdentifier
	if _, err := asn1.Unmarshal(privKey.Algo.Parameters.FullBytes, &namedCurveOID); err != nil {
		return nil, fmt.Errorf("asn1.Unmarshal for oid failed: %w", err)
	}
	if !reflect.DeepEqual(namedCurveOID, OID_SECP256K1) {
		return nil, fmt.Errorf("unknown named curve OID: %s", namedCurveOID.String())
	}
	var curveKey ecPrivateKey
	_, err := asn1.Unmarshal(privKey.PrivateKey, &curveKey)
	if err != nil {
		return nil, fmt.Errorf("asn1.Unmarshal for private key failed: %w", err)
	}
	key, _ := secp256k1.PrivKeyFromBytes(curveKey.PrivateKey)
	return key.ToECDSA(), nil
}

type pkcs8 struct {
	Version    int
	Algo       pkix.AlgorithmIdentifier
	PrivateKey []byte
}

type ecPrivateKey struct {
	Version       int
	PrivateKey    []byte
	NamedCurveOID asn1.ObjectIdentifier `asn1:"optional,explicit,tag:0"`
	PublicKey     asn1.BitString        `asn1:"optional,explicit,tag:1"`
}
