package branding

import (
	"context"

	"stream.place/streamplace/pkg/statedb"
)

// A request's brand follows the hostname it arrived on: a custom domain
// (statedb.BrandingDomain) is branded by its owner's record, cached under
// did:web:<domain>; any other hostname gets the node's own brand under
// did:web:<broadcaster host>.

// ResolveHost returns the broadcaster ID whose branding a request for
// reqHost (a Host header, port allowed) is served with, and the custom
// domain behind it, if any.
func ResolveHost(state *statedb.StatefulDB, broadcasterHost, reqHost string) (string, *statedb.BrandingDomain) {
	def := "did:web:" + broadcasterHost
	if state == nil {
		return def, nil
	}
	host := NormalizeHostname(reqHost)
	if host == "" || host == broadcasterHost {
		return def, nil
	}
	d, err := state.GetBrandingDomain(host)
	if err != nil || d == nil {
		return def, nil
	}
	return d.BrandID(), d
}

// DomainForBrandID returns the custom domain a broadcaster ID belongs to,
// or nil when it is not a custom domain's.
func DomainForBrandID(state *statedb.StatefulDB, brandID string) *statedb.BrandingDomain {
	host, ok := cutDidWeb(brandID)
	if !ok || state == nil {
		return nil
	}
	d, err := state.GetBrandingDomain(host)
	if err != nil {
		return nil
	}
	return d
}

func cutDidWeb(id string) (string, bool) {
	const p = "did:web:"
	if len(id) <= len(p) || id[:len(p)] != p {
		return "", false
	}
	return id[len(p):], true
}

type hostKey struct{}

// WithRequestHost remembers the Host a request arrived on, for handlers
// (XRPC) that only see a context.
func WithRequestHost(ctx context.Context, host string) context.Context {
	return context.WithValue(ctx, hostKey{}, host)
}

// RequestHost is the Host WithRequestHost stored, or "".
func RequestHost(ctx context.Context) string {
	h, _ := ctx.Value(hostKey{}).(string)
	return h
}
