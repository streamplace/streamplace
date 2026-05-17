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

// tid with a random clock id because i'm convinced that's a good idea
// so as to not leak internal infrastructure - don't you not want to
// know where a tid is coming from? idk.
func TID() string {
	id, err := rand.Int(rand.Reader, big.NewInt(1024))
	if err != nil {
		// absence of entropy or ram i guess? seems like a legit panic
		panic(err)
	}
	clock := syntax.NewTIDClock(uint(id.Uint64()))
	return clock.Next().String()
}
