package atproto

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"stream.place/streamplace/pkg/aqhttp"
)

// StationIdentity separates the station's playback origin from its account PDS.
type StationIdentity struct {
	DID string
	PDS string
	URL string
}

// ResolveStationIdentity returns each usable service independently. A caller
// must check its required field; a DID with neither usable service is an error.
func ResolveStationIdentity(ctx context.Context, plcURL, did string) (*StationIdentity, error) {
	parsed, err := syntax.ParseDID(did)
	if err != nil {
		return nil, fmt.Errorf("parse station DID: %w", err)
	}
	directory := identity.BaseDirectory{PLCURL: plcURL, HTTPClient: aqhttp.Client}
	doc, err := directory.ResolveDID(ctx, parsed)
	if err != nil {
		return nil, fmt.Errorf("resolve station DID: %w", err)
	}

	station := &StationIdentity{DID: did}
	allowLoopback := os.Getenv("SP_TRUST_PRIVATE_NETWORK") == "true"
	for _, service := range doc.Service {
		id := strings.TrimPrefix(service.ID, did+"#")
		if id == service.ID {
			if !strings.HasPrefix(id, "#") {
				continue
			}
			id = id[1:]
		}
		switch id {
		case StreamplaceServiceID:
			if station.URL == "" {
				station.URL = stationServiceOrigin(service.ServiceEndpoint, allowLoopback)
			}
		case "atproto_pds":
			if station.PDS == "" && service.Type == "AtprotoPersonalDataServer" {
				station.PDS = stationServiceOrigin(service.ServiceEndpoint, allowLoopback)
			}
		}
	}
	if station.URL == "" && station.PDS == "" {
		return nil, fmt.Errorf("station DID has neither a usable #streamplace nor #atproto_pds origin")
	}
	return station, nil
}

func stationServiceOrigin(raw string, allowLoopback bool) string {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || strings.Contains(raw, "#") ||
		(u.EscapedPath() != "" && u.EscapedPath() != "/") {
		return ""
	}
	if u.Scheme == "https" || (allowLoopback && u.Scheme == "http" && net.ParseIP(u.Hostname()).IsLoopback()) {
		return u.Scheme + "://" + u.Host
	}
	return ""
}
