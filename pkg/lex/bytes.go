package lex

import (
	"bytes"
	"encoding/base64"
	"encoding/json"

	xerrors "golang.org/x/xerrors"

	cbg "github.com/whyrusleeping/cbor-gen"
)

// MaxByteArraySize bounds decoded $bytes values (matches indigo's lexutil).
const MaxByteArraySize = 128 * 1024 * 1024

// Bytes is a DAG-CBOR byte string (`{"$bytes": base64}` in JSON). It is the
// go-dasl-native replacement for indigo's lexutil.LexBytes.
type Bytes []byte

type jsonBytes struct {
	Bytes string `json:"$bytes"`
}

func (lb Bytes) MarshalJSON() ([]byte, error) {
	if lb == nil {
		return nil, xerrors.Errorf("tried to marshal nil $bytes")
	}
	return json.Marshal(jsonBytes{Bytes: base64.RawStdEncoding.EncodeToString([]byte(lb))})
}

func (lb *Bytes) UnmarshalJSON(raw []byte) error {
	var jb jsonBytes
	if err := json.Unmarshal(raw, &jb); err != nil {
		return xerrors.Errorf("parsing $bytes JSON: %v", err)
	}
	out, err := base64.RawStdEncoding.DecodeString(jb.Bytes)
	if err != nil {
		return xerrors.Errorf("parsing $bytes base64: %v", err)
	}
	*lb = Bytes(out)
	return nil
}

// MarshalCBOR implements go-dasl's Marshaler, byte-identical to cbg.WriteByteArray.
func (lb Bytes) MarshalCBOR() ([]byte, error) {
	var buf bytes.Buffer
	if err := cbg.WriteByteArray(cbg.NewCborWriter(&buf), []byte(lb)); err != nil {
		return nil, xerrors.Errorf("failed to write $bytes as CBOR: %w", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalCBOR implements go-dasl's Unmarshaler.
func (lb *Bytes) UnmarshalCBOR(b []byte) error {
	out, err := cbg.ReadByteArray(cbg.NewCborReader(bytes.NewReader(b)), MaxByteArraySize)
	if err != nil {
		return xerrors.Errorf("failed to read $bytes from CBOR: %w", err)
	}
	*lb = Bytes(out)
	return nil
}
