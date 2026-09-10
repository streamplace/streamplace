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

func TestLiveFromConfig(t *testing.T) {
	p, err := LiveFromConfig(&config.CLI{})
	require.NoError(t, err)
	require.Nil(t, p, "no live CDN URL → no provider")

	// The live CDN is independent of the VOD one: VOD flags alone don't
	// turn it on, and its bunny key is its own.
	p, err = LiveFromConfig(&config.CLI{VODCDNURL: "https://vod.b-cdn.net", VODCDNProvider: "bunny", BunnyTokenAuthKey: "vodkey"})
	require.NoError(t, err)
	require.Nil(t, p)

	p, err = LiveFromConfig(&config.CLI{LiveCDNURL: "https://live.b-cdn.net", LiveCDNProvider: "bunny", LiveBunnyTokenAuthKey: "livekey"})
	require.NoError(t, err)
	require.Equal(t, Bunny, p.Name)
	require.Equal(t, bunny.Signer{Key: "livekey"}, p.Signer)
	require.Nil(t, p.Logs, "live never ingests logs")

	p, err = LiveFromConfig(&config.CLI{LiveCDNURL: "https://live.example.com"})
	require.NoError(t, err)
	require.IsType(t, cdn.Static{}, p.Signer)

	_, err = LiveFromConfig(&config.CLI{LiveCDNURL: "https://live.example.com", LiveCDNProvider: "akamai"})
	require.Error(t, err)
}
