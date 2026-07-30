package indexdb

import (
	"bytes"
	"fmt"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/repo"
	"stream.place/streamplace/pkg/spid"
)

// recordParts derives the plumbing every record-stored index row shares:
// the repo DID from the AT-URI authority, the canonical CID of the
// record, and its canonical CBOR encoding. Upsert* methods build their
// rows from these so callers only ever pass (record, ATURI).
func recordParts(aturi syntax.ATURI, rec repo.CborMarshaler) (repoDID, cid string, blob []byte, err error) {
	did, err := aturi.Authority().AsDID()
	if err != nil {
		return "", "", nil, fmt.Errorf("invalid ATURI authority: %w", err)
	}
	c, err := spid.GetCID(rec)
	if err != nil {
		return "", "", nil, fmt.Errorf("get record CID: %w", err)
	}
	var buf bytes.Buffer
	if err := rec.MarshalCBOR(&buf); err != nil {
		return "", "", nil, fmt.Errorf("marshal record: %w", err)
	}
	return did.String(), c.String(), buf.Bytes(), nil
}
