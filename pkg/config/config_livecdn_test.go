package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidateLiveCDN(t *testing.T) {
	ok := func(c CLI) { t.Helper(); require.NoError(t, c.validateLiveCDN()) }
	bad := func(c CLI) { t.Helper(); require.Error(t, c.validateLiveCDN()) }
	ttl := 5 * time.Minute

	ok(CLI{})
	ok(CLI{LiveCDNURL: "https://live.example.com", LiveCDNTokenTTL: ttl})
	ok(CLI{LiveCDNURL: "https://live.b-cdn.net", LiveCDNProvider: "bunny", LiveCDNTokenTTL: ttl})
	ok(CLI{LiveCDNURL: "https://live.b-cdn.net", LiveCDNProvider: "bunny", LiveBunnyTokenAuthKey: "k", LiveCDNTokenTTL: ttl})
	// The VOD CDN's flags are a separate matter; they don't satisfy or
	// break the live ones.
	ok(CLI{VODCDNURL: "https://vod.b-cdn.net", VODCDNProvider: "bunny", BunnyTokenAuthKey: "k"})

	bad(CLI{LiveBunnyTokenAuthKey: "k"})
	bad(CLI{LiveCDNProvider: "bunny", LiveCDNTokenTTL: ttl})
	bad(CLI{LiveCDNURL: "https://live.example.com", LiveCDNProvider: "akamai", LiveCDNTokenTTL: ttl})
	bad(CLI{LiveCDNURL: "https://live.example.com", LiveCDNTokenTTL: 0})
}
