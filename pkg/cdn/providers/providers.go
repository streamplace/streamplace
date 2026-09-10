// Package providers maps the CDN flags onto a cdn.Provider. It sits
// above pkg/cdn and every provider subpackage so the interfaces package
// stays import-free. Two CDNs can be configured: one fronting the VOD
// blob store (--vod-cdn-*) and one fronting the node's own live HLS
// segments (--live-cdn-*); both assemble through the same Spec.
package providers

import (
	"fmt"

	"stream.place/streamplace/pkg/cdn"
	"stream.place/streamplace/pkg/cdn/bunny"
	"stream.place/streamplace/pkg/config"
)

// Provider names accepted by --vod-cdn-provider and --live-cdn-provider.
const (
	Static = ""
	Bunny  = "bunny"
)

// Spec is one CDN's flags, provider-neutral in shape. Log fields are
// only meaningful for the VOD CDN (live view counting doesn't depend
// on segment logs) and are left zero for live.
type Spec struct {
	URL      string
	Provider string

	BunnyTokenAuthKey       string
	BunnyPullZone           string
	BunnyLogStorageZone     string
	BunnyLogStorageEndpoint string
	BunnyLogStorageKey      string
}

// FromSpec builds the Provider a Spec describes. Returns nil when no
// CDN is configured at all (URL empty). Flag consistency is enforced in
// config.Validate; this only assembles.
func FromSpec(spec Spec) (*cdn.Provider, error) {
	if spec.URL == "" {
		return nil, nil
	}
	switch spec.Provider {
	case Static:
		return &cdn.Provider{Name: Static, Signer: cdn.Static{}}, nil
	case Bunny:
		p := &cdn.Provider{Name: Bunny, Signer: cdn.Static{}}
		if spec.BunnyTokenAuthKey != "" {
			p.Signer = bunny.Signer{Key: spec.BunnyTokenAuthKey}
		}
		if spec.BunnyLogStorageZone != "" {
			p.Logs = &bunny.LogStorage{
				Endpoint:  spec.BunnyLogStorageEndpoint,
				Zone:      spec.BunnyLogStorageZone,
				AccessKey: spec.BunnyLogStorageKey,
				PullZone:  spec.BunnyPullZone,
			}
		}
		return p, nil
	default:
		return nil, fmt.Errorf("cdn: unknown provider %q", spec.Provider)
	}
}

// VODSpec is the VOD CDN's flags.
func VODSpec(cli *config.CLI) Spec {
	return Spec{
		URL:                     cli.VODCDNURL,
		Provider:                cli.VODCDNProvider,
		BunnyTokenAuthKey:       cli.BunnyTokenAuthKey,
		BunnyPullZone:           cli.BunnyPullZone,
		BunnyLogStorageZone:     cli.BunnyLogStorageZone,
		BunnyLogStorageEndpoint: cli.BunnyLogStorageEndpoint,
		BunnyLogStorageKey:      cli.BunnyLogStorageKey,
	}
}

// LiveSpec is the live segment CDN's flags: signing only, no logs.
func LiveSpec(cli *config.CLI) Spec {
	return Spec{
		URL:               cli.LiveCDNURL,
		Provider:          cli.LiveCDNProvider,
		BunnyTokenAuthKey: cli.LiveBunnyTokenAuthKey,
	}
}

// FromConfig builds the VOD CDN provider described by the CLI flags.
func FromConfig(cli *config.CLI) (*cdn.Provider, error) {
	p, err := FromSpec(VODSpec(cli))
	if err != nil {
		return nil, fmt.Errorf("--vod-cdn-provider: %w", err)
	}
	return p, nil
}

// LiveFromConfig builds the live segment CDN provider described by the
// CLI flags.
func LiveFromConfig(cli *config.CLI) (*cdn.Provider, error) {
	p, err := FromSpec(LiveSpec(cli))
	if err != nil {
		return nil, fmt.Errorf("--live-cdn-provider: %w", err)
	}
	return p, nil
}
