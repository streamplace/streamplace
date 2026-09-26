package branding

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"

	"stream.place/streamplace/pkg/placestream"
)

// ErrNoRecord is FetchRecord's answer when the owner has published no brand
// for the hostname (yet, or any more).
var ErrNoRecord = errors.New("no brand record")

// Fetched is a brand record pulled from its owner's repo.
type Fetched struct {
	URI      string
	CID      string
	Values   Values
	Warnings []string
}

// FetchRecord pulls the brand record at://did/place.stream.branding.brand/rkey
// and every image it references from the owner's PDS, and validates it like
// an import. dir resolves the owner's DID (a node passes one on its own PLC);
// client makes the HTTP requests (aqhttp.Client in a node, which refuses
// private addresses unless the node trusts them).
func FetchRecord(ctx context.Context, dir identity.Directory, client *http.Client, did, rkey string) (*Fetched, error) {
	parsed, err := syntax.ParseDID(did)
	if err != nil {
		return nil, err
	}
	ident, err := dir.LookupDID(ctx, parsed)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", did, err)
	}
	service := strings.TrimSuffix(ident.PDSEndpoint(), "/")
	if service == "" {
		return nil, fmt.Errorf("%s has no PDS", did)
	}

	q := url.Values{"repo": {did}, "collection": {RecordNSID}, "rkey": {rkey}}
	body, status, err := get(ctx, client, service+"/xrpc/com.atproto.repo.getRecord?"+q.Encode(), 256*1024)
	if err != nil {
		return nil, err
	}
	if status == http.StatusBadRequest || status == http.StatusNotFound {
		var xe struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &xe) == nil && (xe.Error == "RecordNotFound" || status == http.StatusNotFound) {
			return nil, ErrNoRecord
		}
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("getRecord: status %d: %s", status, truncate(string(body), 200))
	}
	var out struct {
		URI   string                    `json:"uri"`
		CID   string                    `json:"cid"`
		Value placestream.BrandingBrand `json:"value"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("getRecord: %w", err)
	}
	content, err := FromRecord(&out.Value)
	if err != nil {
		return nil, err
	}
	f := &Fetched{URI: out.URI, CID: out.CID, Values: Values{}, Warnings: content.Warnings}
	for key, text := range content.Text {
		f.Values[key] = Value{MimeType: TextMime, Data: text}
	}
	for key, blob := range content.Blobs {
		if !AcceptsImage(blob.MimeType) {
			f.Warnings = append(f.Warnings, fmt.Sprintf("%s: %s is not an image type a brand may use", key, blob.MimeType))
			continue
		}
		spec, _ := Lookup(key)
		q := url.Values{"did": {did}, "cid": {blob.Ref.String()}}
		data, status, err := get(ctx, client, service+"/xrpc/com.atproto.sync.getBlob?"+q.Encode(), spec.MaxSize)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("%s: getBlob status %d", key, status)
		}
		got, err := BlobCID(data)
		if err != nil {
			return nil, err
		}
		if !got.Equals(blob.Ref.Cid()) {
			return nil, fmt.Errorf("%s: blob does not match its CID", key)
		}
		f.Values[key] = Value{MimeType: blob.MimeType, Data: data}
	}
	return f, nil
}

// BlobCID is the CID atproto gives a blob: CIDv1, raw codec, sha-256.
func BlobCID(data []byte) (cid.Cid, error) {
	return cid.Prefix{Version: 1, Codec: cid.Raw, MhType: multihash.SHA2_256, MhLength: -1}.Sum(data)
}

func get(ctx context.Context, client *http.Client, u string, limit int) ([]byte, int, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(limit)+1))
	if err != nil {
		return nil, 0, err
	}
	if len(body) > limit {
		return nil, 0, fmt.Errorf("response larger than %d bytes", limit)
	}
	return body, resp.StatusCode, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
