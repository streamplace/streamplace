package atproto

import (
	"context"
	"fmt"
	"net/url"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"stream.place/streamplace/pkg/config"
)

const AtmoqServiceID = "atmoq"

// UpstreamStation is resolved once before any relay or OAuth session is opened.
type UpstreamStation struct {
	DID         string
	URL         string
	FirehoseURL string
}

func ResolveUpstreamStation(ctx context.Context, did string) (*UpstreamStation, error) {
	parsed, err := syntax.ParseDID(did)
	if err != nil {
		return nil, fmt.Errorf("parse upstream station DID: %w", err)
	}
	ident, err := CustomDirectory("").LookupDID(ctx, parsed)
	if err != nil {
		return nil, fmt.Errorf("resolve upstream station %s: %w", did, err)
	}
	stationURL := ident.GetServiceEndpoint(StreamplaceServiceID)
	u, err := url.Parse(stationURL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("upstream station %s must advertise an HTTPS origin in #%s", did, StreamplaceServiceID)
	}
	firehoseURL := ident.GetServiceEndpoint(AtmoqServiceID)
	if err := config.ValidateMoQOrigin(firehoseURL); err != nil {
		return nil, fmt.Errorf("upstream station %s service #%s: %w", did, AtmoqServiceID, err)
	}
	return &UpstreamStation{
		DID:         did,
		URL:         u.Scheme + "://" + u.Host,
		FirehoseURL: firehoseURL,
	}, nil
}
