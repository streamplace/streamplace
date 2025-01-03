package aqpub

import (
	"crypto/ecdsa"
	"math/big"

	"github.com/decred/dcrd/dcrec/secp256k1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

type Pub interface {
	Address() common.Address
	String() string
	Equals(Pub) bool
}

type pub struct {
	addr common.Address
}

func FromHexString(str string) (Pub, error) {
	bs, err := hexutil.Decode(str)
	if err != nil {
		return nil, err
	}
	addr := common.BytesToAddress(bs)
	return &pub{addr}, nil
}

func FromPublicKey(key *ecdsa.PublicKey) (Pub, error) {
	addr := crypto.PubkeyToAddress(*key)
	return &pub{addr}, nil
}

func FromBytes(bs []byte) (Pub, error) {
	pubkey, err := secp256k1.ParsePubKey(bs)
	if err != nil {
		return nil, err
	}
	addr := crypto.PubkeyToAddress(*pubkey.ToECDSA())
	return &pub{addr}, nil
}

func FromPoints(x, y *big.Int) (Pub, error) {
	key := ecdsa.PublicKey{Curve: secp256k1.S256(), X: x, Y: y}
	return FromPublicKey(&key)
}

func (p *pub) Address() common.Address {
	return p.addr
}

func (p *pub) String() string {
	addr := p.Address()
	return hexutil.Encode(addr.Bytes())
}

func (p *pub) Equals(other Pub) bool {
	addr1 := p.Address()
	addr2 := other.Address()
	return addr1.Cmp(addr2) == 0
}
