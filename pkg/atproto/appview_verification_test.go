package atproto

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAppViewConfig(t *testing.T) {
	c := parseAppViewConfig("https://api.example.com/", "", "")
	require.Equal(t, "https://api.example.com", c.URL)
	require.Equal(t, "did:web:api.example.com", c.Issuer())
	require.Equal(t, "wsocialVerified", c.Field)
	require.True(t, c.matches("wid"), "* means any non-empty value")
	require.False(t, c.matches(""))
	require.False(t, c.matches(nil))

	c = parseAppViewConfig("https://api.example.com", "wsocialAccountType", "human, organization")
	require.True(t, c.matches("human"))
	require.False(t, c.matches("bot"))

	require.Equal(t, appViewConfig{}, parseAppViewConfig("", "", ""))
	require.Equal(t, appViewConfig{}, parseAppViewConfig("not a url", "", ""))
}

func TestCheckAppView(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/xrpc/app.bsky.actor.getProfiles", r.URL.Path)
		actors := r.URL.Query()["actors"]
		var profiles []map[string]any
		for _, a := range actors {
			p := map[string]any{"did": a, "handle": a + ".example"}
			if a == "did:plc:yes" {
				p["wsocialVerified"] = "wid"
			}
			profiles = append(profiles, p)
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"profiles": profiles}))
	}))
	defer srv.Close()
	c := parseAppViewConfig(srv.URL, "", "")
	atsync := &ATProtoSynchronizer{}
	res, err := atsync.checkAppView(context.Background(), c, []string{"did:plc:yes", "did:plc:no"})
	require.NoError(t, err)
	require.True(t, res["did:plc:yes"])
	require.False(t, res["did:plc:no"])
}
