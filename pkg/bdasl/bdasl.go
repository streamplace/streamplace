// Package bdasl implements BDASL content identifiers (CIDs) using BLAKE3.
//
// A BDASL CID is a DASL CID with BLAKE3 as the hash function:
//
//	string form: "b" + base32lower(binary)
//	binary form: [0x01 version] [0x55 raw codec] [0x1e BLAKE3] [0x20 size] [32-byte digest]
//
// See https://dasl.ing/bdasl.html and https://dasl.ing/cid.html
package bdasl

import (
	"encoding/base32"
	"fmt"
	"io"
	"strings"

	"lukechampine.com/blake3"
)

// base32 lowercase encoder per RFC 4648 §6
var b32 = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

// CID computes a BDASL CID (BLAKE3, raw codec) for the given data.
func CID(data []byte) string {
	digest := blake3.Sum256(data)
	return encodeCID(digest[:])
}

// encodeCID wraps a 32-byte BLAKE3 digest in the BDASL multibase/multihash
// framing. The caller is responsible for ensuring digest is exactly 32
// bytes long.
func encodeCID(digest []byte) string {
	var bin [36]byte
	bin[0] = 0x01 // version
	bin[1] = 0x55 // raw codec
	bin[2] = 0x1e // BLAKE3
	bin[3] = 0x20 // 32-byte hash
	copy(bin[4:], digest)
	return "b" + b32.EncodeToString(bin[:])
}

// Writer is an io.Writer that incrementally BLAKE3-hashes everything
// written to it and exposes the final BDASL CID via CID(). It's intended
// for streaming pipelines that need to compute a content identifier for a
// payload too large to hold in memory.
type Writer struct {
	h *blake3.Hasher
}

// NewWriter returns a fresh streaming BDASL hasher.
func NewWriter() *Writer {
	return &Writer{h: blake3.New(32, nil)}
}

// Write feeds bytes into the running BLAKE3 hash. Never returns a short
// write or error.
func (w *Writer) Write(p []byte) (int, error) {
	return w.h.Write(p)
}

// CID returns the BDASL CID for everything written so far. Calling it
// does not finalize the hasher — further Write calls are valid and a
// subsequent CID call returns the updated identifier.
func (w *Writer) CID() string {
	return encodeCID(w.h.Sum(nil))
}

var _ io.Writer = (*Writer)(nil)

// Verify checks that data matches the given CID.
func Verify(cid string, data []byte) error {
	expected := CID(data)
	if cid != expected {
		return fmt.Errorf("CID mismatch: got %s, expected %s", cid, expected)
	}
	return nil
}

// Parse extracts the 32-byte BLAKE3 digest from a BDASL CID string.
// Returns an error if the CID is malformed or uses an unsupported hash type.
func Parse(cid string) ([32]byte, error) {
	var digest [32]byte
	if !strings.HasPrefix(cid, "b") {
		return digest, fmt.Errorf("unsupported CID prefix: %q", cid[:1])
	}
	bin, err := b32.DecodeString(cid[1:])
	if err != nil {
		return digest, fmt.Errorf("base32 decode: %w", err)
	}
	if len(bin) != 36 {
		return digest, fmt.Errorf("unexpected CID length: %d", len(bin))
	}
	if bin[0] != 0x01 {
		return digest, fmt.Errorf("unsupported CID version: %d", bin[0])
	}
	if bin[2] != 0x1e {
		return digest, fmt.Errorf("unsupported hash type: 0x%02x (expected 0x1e BLAKE3)", bin[2])
	}
	if bin[3] != 0x20 {
		return digest, fmt.Errorf("unsupported hash size: %d", bin[3])
	}
	copy(digest[:], bin[4:])
	return digest, nil
}
