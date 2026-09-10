package providers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/cdn"
	"stream.place/streamplace/pkg/cdn/bunny"
	"stream.place/streamplace/pkg/config"
)

func TestFromConfig(t *testing.T) {
	p, err := FromConfig(&config.CLI{})
	require.NoError(t, err)
	require.Nil(t, p, "no CDN URL → no provider")

	p, err = FromConfig(&config.CLI{VODCDNURL: "https://cdn.example.com"})
	require.NoError(t, err)
	require.Equal(t, Static, p.Name)
	require.IsType(t, cdn.Static{}, p.Signer)
	require.Nil(t, p.Logs)

	p, err = FromConfig(&config.CLI{VODCDNURL: "https://x.b-cdn.net", VODCDNProvider: "bunny"})
	require.NoError(t, err)
	require.IsType(t, cdn.Static{}, p.Signer, "bunny without a token key is unsigned")

	p, err = FromConfig(&config.CLI{
		VODCDNURL: "https://x.b-cdn.net", VODCDNProvider: "bunny",
		BunnyTokenAuthKey: "k", BunnyPullZone: "x", BunnyLogStorageZone: "z", BunnyLogStorageKey: "s",
		BunnyLogStorageEndpoint: "https://storage.bunnycdn.com",
	})
	require.NoError(t, err)
	require.Equal(t, bunny.Signer{Key: "k"}, p.Signer)
	require.IsType(t, &bunny.LogStorage{}, p.Logs)

	_, err = FromConfig(&config.CLI{VODCDNURL: "https://cdn.example.com", VODCDNProvider: "akamai"})
	require.Error(t, err)
}
