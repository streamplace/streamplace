package spid

import (
	"bytes"

	"github.com/bluesky-social/indigo/repo"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
)

func GetCID(record repo.CborMarshaler) (*cid.Cid, error) {
	buf := bytes.NewBuffer(nil)
	err := record.MarshalCBOR(buf)
	if err != nil {
		return nil, err
	}
	return GetCIDFromBytes(buf.Bytes())
}

func GetCIDFromBytes(bs []byte) (*cid.Cid, error) {
	builder := cid.NewPrefixV1(cid.DagCBOR, multihash.SHA2_256)
	c, err := builder.Sum(bs)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
