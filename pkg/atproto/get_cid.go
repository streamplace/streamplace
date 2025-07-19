package atproto

import (
	"bytes"

	"github.com/bluesky-social/indigo/repo"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
)

func GetCID(record repo.CborMarshaler) (*cid.Cid, error) {
	builder := cid.NewPrefixV1(cid.DagCBOR, multihash.SHA2_256)
	buf := bytes.NewBuffer(nil)
	err := record.MarshalCBOR(buf)
	if err != nil {
		return nil, err
	}
	c, err := builder.Sum(buf.Bytes())
	if err != nil {
		return nil, err
	}
	return &c, nil
}
