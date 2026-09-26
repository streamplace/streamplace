package branding

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	glex "github.com/streamplace/glex/runtime"
	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/placestream"
)

// fakeUpload stands in for a PDS's uploadBlob: the ref is the blob's real
// CID, and the uploaded bytes are kept so a test can serve them back.
func fakeUpload(store map[string][]byte) BlobUploader {
	return func(ctx context.Context, key string, v Value) (*glex.Blob, error) {
		c, err := BlobCID(v.Data)
		if err != nil {
			return nil, err
		}
		store[c.String()] = v.Data
		return &glex.Blob{Ref: glex.Link(c), MimeType: v.MimeType, Size: int64(len(v.Data))}, nil
	}
}

// A brand survives values -> record -> CBOR (what a PDS stores) -> record ->
// values, images as blob refs to the same bytes.
func TestRecordRoundTrip(t *testing.T) {
	logo := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`)
	in := Values{
		"siteTitle":     {TextMime, []byte("Example Live")},
		"primaryColor":  {TextMime, []byte("#11e8b2")},
		"chatLayout":    {TextMime, []byte("avatar")},
		"navLinks":      {TextMime, []byte(`[{"icon":"home","label":"Home","url":"/"}]`)},
		"navCta":        {TextMime, []byte(`{"label":"Join","url":"https://example.com"}`)},
		"appName":       {TextMime, []byte("Example")},
		"appBundleId":   {TextMime, []byte("com.example.live")},
		"appHost":       {TextMime, []byte("live.example.com")},
		"appColors":     {TextMime, []byte(`{"iconBackground":"#000000","ink":"#ffffff"}`)},
		"appStory":      {TextMime, []byte(`{"geometry":{"grid":2.5},"tagline":"hi"}`)},
		"mainLogo":      {"image/svg+xml", logo},
		"appIcon":       {"image/png", []byte{0x89, 'P', 'N', 'G', 1}},
		"notAKnownKey1": {TextMime, []byte("dropped: a record only has known keys")},
	}
	store := map[string][]byte{}
	rec, err := ToRecord(context.Background(), in, fakeUpload(store))
	require.NoError(t, err)
	require.Equal(t, "Example Live", *rec.SiteTitle)
	require.Len(t, rec.NavLinks, 1)
	require.NotNil(t, rec.MainLogo)

	var buf bytes.Buffer
	require.NoError(t, rec.MarshalCBOR(&buf))
	var back placestream.BrandingBrand
	require.NoError(t, back.UnmarshalCBOR(&buf))

	content, err := FromRecord(&back)
	require.NoError(t, err)
	require.Empty(t, content.Warnings)
	for key, v := range in {
		spec, known := Lookup(key)
		if !known {
			require.NotContains(t, content.Text, key)
			continue
		}
		if spec.Kind == KindImage {
			blob, ok := content.Blobs[key]
			require.True(t, ok, key)
			require.Equal(t, v.MimeType, blob.MimeType)
			require.Equal(t, v.Data, store[blob.Ref.String()], key)
			continue
		}
		want, err := Normalize(key, v.Data)
		require.NoError(t, err)
		require.Equal(t, string(want), string(content.Text[key]), key)
	}
}

// A record written by some other client is held to the admin API's rules:
// a bad value is dropped with a warning, the rest of the brand applies.
func TestFromRecordValidates(t *testing.T) {
	bad := "not-a-color"
	title := "Fine"
	host := "https://has-a-scheme.example"
	content, err := FromRecord(&placestream.BrandingBrand{PrimaryColor: &bad, SiteTitle: &title, AppHost: &host})
	require.NoError(t, err)
	require.Equal(t, "Fine", string(content.Text["siteTitle"]))
	require.NotContains(t, content.Text, "primaryColor")
	require.NotContains(t, content.Text, "appHost")
	require.Len(t, content.Warnings, 2)
}

func TestAppKeyValidation(t *testing.T) {
	for _, tc := range []struct {
		key, value string
		ok         bool
	}{
		{"appBundleId", "com.example.live", true},
		{"appBundleId", "tv.aquareum", true},
		{"appBundleId", "nodots", false},
		{"appBundleId", "com.example.live app", false},
		{"appHost", "live.example.com", true},
		{"appHost", "live.example.com:443", false},
		{"appHost", "https://live.example.com", false},
		{"appMonochrome", "on", true},
		{"appMonochrome", "true", false},
		{"appColors", `{"ink":"#000"}`, true},
		{"appColors", `{"ink":3}`, false},
		{"appColors", `[]`, false},
	} {
		_, err := Normalize(tc.key, []byte(tc.value))
		if tc.ok {
			require.NoError(t, err, "%s=%s", tc.key, tc.value)
		} else {
			require.Error(t, err, "%s=%s", tc.key, tc.value)
		}
	}
	require.False(t, IsRuntime("appIcon"))
	require.True(t, IsRuntime("siteTitle"))
	require.True(t, IsRuntime("keyFromANewerAdmin"))
}

// The repo's own brand directory is a valid bundle: the same files that
// build the apps import into a node, and survive export back to a directory.
func TestBrandDirIsABundle(t *testing.T) {
	dir := filepath.Join("..", "..", "brand")
	zipped, err := ReadBundle(dir)
	require.NoError(t, err)
	p, err := Parse(zipped)
	require.NoError(t, err)
	require.Empty(t, p.Warnings)
	require.Equal(t, "Streamplace", string(p.Values["appName"].Data))
	require.Equal(t, "image/svg+xml", p.Values["mainLogo"].MimeType)
	mark, err := os.ReadFile(filepath.Join(dir, "mark.svg"))
	require.NoError(t, err)
	require.Equal(t, mark, p.Values["mainLogo"].Data)
	require.Contains(t, string(p.Values["appColors"].Data), `"iconBackground":"#ffffff"`)

	out := filepath.Join(t.TempDir(), "brand")
	bundle, err := Bundle(p.Values)
	require.NoError(t, err)
	require.NoError(t, UnzipDir(bundle, out))
	again, err := ReadBundle(out)
	require.NoError(t, err)
	p2, err := Parse(again)
	require.NoError(t, err)
	require.Equal(t, p.Values, p2.Values)
}

func TestResolveHost(t *testing.T) {
	state := newState(t)
	id, d := ResolveHost(state, "node.example", "node.example:8080")
	require.Equal(t, bid, id)
	require.Nil(t, d)
	id, d = ResolveHost(state, "node.example", "live.other.example")
	require.Equal(t, bid, id, "an unknown host gets the node's brand")
	require.Nil(t, d)

	_, err := state.PutBrandingDomain("Live.Other.Example", "did:plc:owner")
	require.NoError(t, err)
	id, d = ResolveHost(state, "node.example", "live.other.example:443")
	require.Equal(t, "did:web:live.other.example", id)
	require.NotNil(t, d)
	require.Equal(t, "did:plc:owner", d.OwnerDID)
	require.Equal(t, d.Hostname, DomainForBrandID(state, id).Hostname)
	require.Nil(t, DomainForBrandID(state, bid))

	// A new owner starts from nothing: the old owner's brand goes.
	require.NoError(t, state.PutBrandingBlob(id, "siteTitle", TextMime, []byte("Old"), nil, nil))
	_, err = state.PutBrandingDomain("live.other.example", "did:plc:someone-else")
	require.NoError(t, err)
	keys, err := state.ListBrandingKeys(id)
	require.NoError(t, err)
	require.Empty(t, keys)

	require.NoError(t, state.DeleteBrandingDomain("live.other.example"))
	id, d = ResolveHost(state, "node.example", "live.other.example")
	require.Equal(t, bid, id)
	require.Nil(t, d)
}

func TestApplyReplacesAndReports(t *testing.T) {
	state := newState(t)
	ctx := context.Background()
	_, err := Apply(ctx, state, bid, Values{"siteTitle": {TextMime, []byte("A")}, "primaryColor": {TextMime, []byte("#111111")}}, nil, false, false)
	require.NoError(t, err)
	report, err := Apply(ctx, state, bid, Values{"siteTitle": {TextMime, []byte("B")}}, nil, false, false)
	require.NoError(t, err)
	require.True(t, report.Applied)
	require.Equal(t, []Change{{Key: "primaryColor", Action: "removed"}, {Key: "siteTitle", Action: "changed", Detail: "B"}}, report.Changes)
	values, err := ReadValues(state, bid)
	require.NoError(t, err)
	require.Equal(t, Values{"siteTitle": {TextMime, []byte("B")}}, values)
}
