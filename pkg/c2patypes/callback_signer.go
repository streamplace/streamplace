package c2patypes

import (
	"crypto"
	"crypto/rand"
	"crypto/sha256"
)

type CallbackSigner struct {
	signer crypto.Signer
}

func (c *CallbackSigner) Sign(data []byte) ([]byte, error) {
	digest := sha256.New().Sum(data)

	return c.signer.Sign(rand.Reader, digest, nil)
}
