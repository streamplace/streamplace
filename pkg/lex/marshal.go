package lex

import (
	"encoding/json"
	"io"

	"github.com/hyphacoop/go-dasl/drisl"
)

// MarshalCBOR encodes v as canonical DAG-CBOR (via go-dasl) and writes it to w.
// Generated record/object types call this from their cbg.CBORMarshaler-shaped
// MarshalCBOR(io.Writer) adapter, so they interoperate with indigo's
// repo/carstore/MST layer while serializing through go-dasl.
func MarshalCBOR(w io.Writer, v any) error {
	b, err := drisl.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

// UnmarshalCBOR reads canonical DAG-CBOR from r and decodes it into v (via go-dasl).
func UnmarshalCBOR(r io.Reader, v any) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return drisl.Unmarshal(b, v)
}

type typeHolder struct {
	Type string `json:"$type"`
}

func typeExtractJSON(b []byte) (string, error) {
	var th typeHolder
	if err := json.Unmarshal(b, &th); err != nil {
		return "", err
	}
	return th.Type, nil
}

func typeExtractCBOR(b []byte) (string, error) {
	var th typeHolder
	if err := drisl.Unmarshal(b, &th); err != nil {
		return "", err
	}
	return th.Type, nil
}
