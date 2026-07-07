package lex

import (
	"encoding/json"

	"github.com/hyphacoop/go-dasl/drisl"
	"github.com/ipfs/go-cid"
	xerrors "golang.org/x/xerrors"
)

// Blob is the atproto blob reference (`{$type:"blob", ref, mimeType, size}`),
// the go-dasl-native replacement for indigo's lexutil.LexBlob. A Size of -1
// indicates (and serializes as) a legacy blob (string CID, no size).
type Blob struct {
	Ref      Link
	MimeType string
	Size     int64
}

// blobSchema is the wire shape of a modern blob. Field order in the struct is
// irrelevant; go-dasl sorts map keys into canonical DAG-CBOR order.
type blobSchema struct {
	Type     string `json:"$type"`
	Ref      Link   `json:"ref"`
	MimeType string `json:"mimeType"`
	Size     int64  `json:"size"`
}

type legacyBlob struct {
	Cid      string `json:"cid"`
	MimeType string `json:"mimeType"`
}

func (b Blob) MarshalJSON() ([]byte, error) {
	if b.Size < 0 {
		return json.Marshal(legacyBlob{Cid: b.Ref.String(), MimeType: b.MimeType})
	}
	return json.Marshal(blobSchema{Type: "blob", Ref: b.Ref, MimeType: b.MimeType, Size: b.Size})
}

func (b *Blob) UnmarshalJSON(raw []byte) error {
	typ, err := typeExtractJSON(raw)
	if err != nil {
		return xerrors.Errorf("parsing blob type: %v", err)
	}
	if typ == "blob" {
		var bs blobSchema
		if err := json.Unmarshal(raw, &bs); err != nil {
			return xerrors.Errorf("parsing blob JSON: %v", err)
		}
		if bs.Size < 0 {
			return xerrors.Errorf("parsing blob: negative size: %d", bs.Size)
		}
		b.Ref, b.MimeType, b.Size = bs.Ref, bs.MimeType, bs.Size
		return nil
	}
	var legacy legacyBlob
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return xerrors.Errorf("parsing legacy blob: %v", err)
	}
	refCid, err := cid.Decode(legacy.Cid)
	if err != nil {
		return xerrors.Errorf("parsing CID in legacy blob: %v", err)
	}
	b.Ref, b.MimeType, b.Size = Link(refCid), legacy.MimeType, -1
	return nil
}

// MarshalCBOR implements go-dasl's Marshaler, byte-identical to lexutil.LexBlob.
func (b Blob) MarshalCBOR() ([]byte, error) {
	if b.Size < 0 {
		return drisl.Marshal(legacyBlob{Cid: b.Ref.String(), MimeType: b.MimeType})
	}
	return drisl.Marshal(blobSchema{Type: "blob", Ref: b.Ref, MimeType: b.MimeType, Size: b.Size})
}

// UnmarshalCBOR implements go-dasl's Unmarshaler.
func (b *Blob) UnmarshalCBOR(raw []byte) error {
	typ, err := typeExtractCBOR(raw)
	if err != nil {
		return xerrors.Errorf("parsing blob CBOR type: %w", err)
	}
	if typ == "blob" {
		var bs blobSchema
		if err := drisl.Unmarshal(raw, &bs); err != nil {
			return xerrors.Errorf("parsing blob CBOR: %v", err)
		}
		if bs.Size < 0 {
			return xerrors.Errorf("parsing blob CBOR: negative size: %d", bs.Size)
		}
		b.Ref, b.MimeType, b.Size = bs.Ref, bs.MimeType, bs.Size
		return nil
	}
	var legacy legacyBlob
	if err := drisl.Unmarshal(raw, &legacy); err != nil {
		return xerrors.Errorf("parsing legacy blob CBOR: %v", err)
	}
	refCid, err := cid.Decode(legacy.Cid)
	if err != nil {
		return xerrors.Errorf("parsing CID in legacy blob CBOR: %v", err)
	}
	b.Ref, b.MimeType, b.Size = Link(refCid), legacy.MimeType, -1
	return nil
}
