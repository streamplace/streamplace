package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateVODCDN(t *testing.T) {
	ok := func(c CLI) { t.Helper(); require.NoError(t, c.validateVODCDN()) }
	bad := func(c CLI) { t.Helper(); require.Error(t, c.validateVODCDN()) }

	ok(CLI{})
	ok(CLI{VODCDNURL: "https://cdn.example.com"})
	ok(CLI{VODCDNURL: "https://x.b-cdn.net", VODCDNProvider: "bunny"})
	ok(CLI{VODCDNURL: "https://x.b-cdn.net", VODCDNProvider: "bunny", BunnyTokenAuthKey: "k"})
	ok(CLI{VODCDNURL: "https://x.b-cdn.net", VODCDNProvider: "bunny",
		BunnyPullZone: "x", BunnyLogStorageZone: "z", BunnyLogStorageKey: "s"})

	// Provider flags without the provider.
	bad(CLI{BunnyTokenAuthKey: "k"})
	bad(CLI{VODCDNURL: "https://cdn.example.com", BunnyLogStorageZone: "z"})
	// Provider without a CDN URL.
	bad(CLI{VODCDNProvider: "bunny"})
	// Partial log source.
	bad(CLI{VODCDNURL: "https://x.b-cdn.net", VODCDNProvider: "bunny", BunnyLogStorageZone: "z"})
	bad(CLI{VODCDNURL: "https://x.b-cdn.net", VODCDNProvider: "bunny", BunnyPullZone: "x", BunnyLogStorageKey: "s"})
	// Unknown provider.
	bad(CLI{VODCDNURL: "https://cdn.example.com", VODCDNProvider: "akamai"})
}
