package spid

import (
	"crypto/rand"
	"math/big"

	"github.com/bluesky-social/indigo/atproto/syntax"
)

var TIDClock *syntax.TIDClock

func init() {
	id, err := rand.Int(rand.Reader, big.NewInt(1024))
	if err != nil {
		panic(err)
	}
	clock := syntax.NewTIDClock(uint(id.Uint64()))
	TIDClock = &clock
}
