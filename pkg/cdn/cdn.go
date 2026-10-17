// Package cdn defines the seam between streamplace and whatever CDN an
// operator puts in front of the VOD blob store. Two capabilities are
// provider-specific and everything else is not:
//
//   - Signer: how a blob URL in an HLS playlist gets signed so the CDN
//     only serves what this node handed out. Every CDN has its own
//     recipe (bunny is an HMAC in the query string, CloudFront is a
//     signed policy, Cloudflare is whatever your worker verifies).
//
//   - LogSource: where the CDN's archived access logs live and how a
//     line decodes. Once the CDN serves segments, the node no longer
//     sees segment requests, so view counting depends on pulling them
//     back out of the CDN's logs (see pkg/viewlog RunIngest).
//
// Providers live in subpackages (pkg/cdn/bunny) and are selected by
// --vod-cdn-provider via pkg/cdn/providers.FromConfig. The generic
// --vod-cdn-url flag keeps working on its own as an unsigned, log-less
// static CDN.
package cdn

import (
	"context"
	"net/url"
	"time"
)

// Signer produces the viewer-facing URL for a blob served by the CDN.
type Signer interface {
	// SignURL returns the URL a player should fetch for u, valid until
	// expires. Implementations must not mutate u. Unsigned providers
	// return u.String().
	SignURL(u *url.URL, expires time.Time) string
}

// Static is the no-signature Signer: a plain CDN over a public bucket.
type Static struct{}

func (Static) SignURL(u *url.URL, _ time.Time) string { return u.String() }

// Part is one immutable archived access-log file at the provider.
type Part struct {
	// ID is the provider-unique, stable identity of the part (e.g.
	// its storage path). Ingest cursors key on it, so it must not
	// change between listings.
	ID string
	// Day is the UTC date the provider filed the part under. Events
	// inside are at or after this day's start; the ingester uses it
	// to skip parts older than its lookback.
	Day time.Time
	// Size is the compressed byte length, for logging.
	Size int64
}

// Request is one CDN access-log record in provider-neutral shape.
// Only the fields view counting needs are carried.
type Request struct {
	// Time is when the edge served the request.
	Time time.Time
	// Status is the HTTP status the edge returned.
	Status int
	// BytesSent is the response body size the edge reports. Access
	// logs carry no Range header, so this is the only byte signal.
	BytesSent int64
	// RemoteIP is the viewer's address, hashed by the ingester before
	// it's persisted.
	RemoteIP string
	// URL is what the viewer requested: a path plus query string, or
	// an absolute URL (providers differ; consumers must parse).
	URL string
}

// LogSource lists and decodes a provider's archived access logs.
type LogSource interface {
	// ListParts returns every part filed on or after `since`'s UTC
	// date. Parts already ingested are filtered by the caller.
	ListParts(ctx context.Context, since time.Time) ([]Part, error)
	// ReadPart streams the part's records to emit in file order. An
	// error from emit aborts the read and is returned.
	ReadPart(ctx context.Context, part Part, emit func(Request) error) error
}

// Provider bundles what the configured CDN can do. Logs is nil when
// the provider has no log source configured.
type Provider struct {
	// Name is the --vod-cdn-provider value ("" for a plain static CDN).
	Name   string
	Signer Signer
	Logs   LogSource
}
