// Package providers maps --vod-cdn-provider onto a cdn.Provider. It
// sits above pkg/cdn and every provider subpackage so the interfaces
// package stays import-free.
package providers

import (
	"fmt"

	"stream.place/streamplace/pkg/cdn"
	"stream.place/streamplace/pkg/cdn/bunny"
	"stream.place/streamplace/pkg/config"
)

// Provider names accepted by --vod-cdn-provider.
const (
	Static = ""
	Bunny  = "bunny"
)

// FromConfig builds the Provider described by the CLI flags. Returns
// nil when no CDN is configured at all (--vod-cdn-url empty). Flag
// consistency is enforced in config.Validate; this only assembles.
func FromConfig(cli *config.CLI) (*cdn.Provider, error) {
	if cli.VODCDNURL == "" {
		return nil, nil
	}
	switch cli.VODCDNProvider {
	case Static:
		return &cdn.Provider{Name: Static, Signer: cdn.Static{}}, nil
	case Bunny:
		p := &cdn.Provider{Name: Bunny, Signer: cdn.Static{}}
		if cli.BunnyTokenAuthKey != "" {
			p.Signer = bunny.Signer{Key: cli.BunnyTokenAuthKey}
		}
		if cli.BunnyLogStorageZone != "" {
			p.Logs = &bunny.LogStorage{
				Endpoint:  cli.BunnyLogStorageEndpoint,
				Zone:      cli.BunnyLogStorageZone,
				AccessKey: cli.BunnyLogStorageKey,
				PullZone:  cli.BunnyPullZone,
			}
		}
		return p, nil
	default:
		return nil, fmt.Errorf("cdn: unknown --vod-cdn-provider %q", cli.VODCDNProvider)
	}
}
