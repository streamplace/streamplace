package spxrpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"stream.place/streamplace/pkg/branding"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/statedb"
)

// A custom domain is served its owner's brand; every other hostname the
// node's own.
func TestCustomDomainReadsFollowHost(t *testing.T) {
	ctx := context.Background()
	sdb, err := statedb.MakeDB(ctx, &config.CLI{DBURL: ":memory:"}, nil, nil)
	require.NoError(t, err)
	s := &Server{cli: &config.CLI{BroadcasterHost: "node.example", AdminDIDs: []string{"did:plc:admin"}}, statefulDB: sdb}

	_, err = sdb.PutBrandingDomain("live.custom.example", "did:plc:owner")
	require.NoError(t, err)
	nodeIcon := []byte{0x89, 'P', 'N', 'G', 'n'}
	domainIcon := []byte{0x89, 'P', 'N', 'G', 'd'}
	require.NoError(t, sdb.PutBrandingBlob("did:web:node.example", "favicon", "image/png", nodeIcon, nil, nil))
	require.NoError(t, sdb.PutBrandingBlob("did:web:live.custom.example", "favicon", "image/png", domainIcon, nil, nil))
	require.NoError(t, sdb.PutBrandingBlob("did:web:live.custom.example", "siteTitle", branding.TextMime, []byte("Custom"), nil, nil))
	require.NoError(t, sdb.PutBrandingBlob("did:web:live.custom.example", "appName", branding.TextMime, []byte("Custom App"), nil, nil))

	favicon := func(host string) []byte {
		req := httptest.NewRequest(http.MethodGet, "/favicon.png", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		require.NoError(t, s.HandleFaviconPNG(echo.New().NewContext(req, rec)))
		return rec.Body.Bytes()
	}
	require.Equal(t, nodeIcon, favicon("node.example"))
	require.Equal(t, domainIcon, favicon("live.custom.example:8443"))
	require.Equal(t, nodeIcon, favicon("unregistered.example"))

	onHost := func(host string) context.Context { return branding.WithRequestHost(ctx, host) }

	out, err := s.handlePlaceStreamBroadcastGetBroadcaster(onHost("live.custom.example"))
	require.NoError(t, err)
	require.Equal(t, "did:web:node.example", out.Broadcaster, "the broadcaster is the node's either way")
	require.Equal(t, "did:web:live.custom.example", *out.Brand)
	require.Equal(t, []string{"did:plc:owner"}, out.BrandAdmins)
	out, err = s.handlePlaceStreamBroadcastGetBroadcaster(onHost("node.example"))
	require.NoError(t, err)
	require.Equal(t, "did:web:node.example", *out.Brand)
	require.Equal(t, []string{"did:plc:admin"}, out.BrandAdmins)

	// No broadcaster param: the brand of the host asked on. Build-time keys
	// stay out of what the running app fetches.
	br, err := s.HandlePlaceStreamBrandingGetBrandingDirect(onHost("live.custom.example"), "")
	require.NoError(t, err)
	keys := map[string]string{}
	for _, a := range br.Assets {
		if a.Data != nil {
			keys[a.Key] = *a.Data
		} else {
			keys[a.Key] = ""
		}
	}
	require.Equal(t, "Custom", keys["siteTitle"])
	require.Contains(t, keys, "favicon")
	require.NotContains(t, keys, "appName")
}
