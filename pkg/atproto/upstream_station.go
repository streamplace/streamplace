package atproto

import (
	"context"
	"fmt"
	"net/url"

	"github.com/bluesky-social/indigo/atproto/syntax"
)

// UpstreamStation is resolved once before any relay or OAuth session is opened.
type UpstreamStation struct {
	DID string
	URL string
}

// ResolveUpstreamStation looks the station up through plcURL's directory, the
// one every other DID lookup uses, so a did:plc station in a private or
// development directory resolves too.
func ResolveUpstreamStation(ctx context.Context, plcURL, did string) (*UpstreamStation, error) {
	parsed, err := syntax.ParseDID(did)
	if err != nil {
		return nil, fmt.Errorf("parse upstream station DID: %w", err)
	}
	ident, err := CustomDirectory(plcURL).LookupDID(ctx, parsed)
	if err != nil {
		return nil, fmt.Errorf("resolve upstream station %s: %w", did, err)
	}
	stationURL := ident.GetServiceEndpoint(StreamplaceServiceID)
	u, err := url.Parse(stationURL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("upstream station %s must advertise an HTTPS origin in #%s", did, StreamplaceServiceID)
	}
	return &UpstreamStation{
		DID: did,
		URL: u.Scheme + "://" + u.Host,
	}, nil
}
