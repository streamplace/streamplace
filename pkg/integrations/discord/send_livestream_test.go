package discord

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	glex "github.com/streamplace/glex/runtime"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/integrations/discord/discordtypes"
	"stream.place/streamplace/pkg/placestream"
)

func TestSendLivestreamURL(t *testing.T) {
	const did = "did:plc:livestreamurl"
	// Avoid a public AppView lookup for this isolated account.
	avatarCacheMutex.Lock()
	avatarCache[did] = ""
	avatarCacheMutex.Unlock()
	t.Cleanup(func() {
		avatarCacheMutex.Lock()
		delete(avatarCache, did)
		avatarCacheMutex.Unlock()
	})

	nodeURL := "https://node.example/streamer.example"
	canonicalURL := "https://stream.place/streamer.example"
	emptyURL := ""
	for _, tc := range []struct {
		name         string
		url          *string
		canonicalURL *string
		wantURL      string
		wantTitle    string
	}{
		{"canonical takes precedence", &nodeURL, &canonicalURL, canonicalURL, "@streamer.example is LIVE on stream.place!"},
		{"canonical without node URL", nil, &canonicalURL, canonicalURL, "@streamer.example is LIVE on stream.place!"},
		{"missing canonical falls back", &nodeURL, nil, nodeURL, "@streamer.example is LIVE on node.example!"},
		{"empty canonical falls back", &nodeURL, &emptyURL, nodeURL, "@streamer.example is LIVE on node.example!"},
		{"no URL", nil, nil, "", "@streamer.example is LIVE!"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payloads := make(chan discordtypes.Payload, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload discordtypes.Payload
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				payloads <- payload
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			require.NoError(t, SendLivestream(t.Context(), &discordtypes.Webhook{URL: server.URL}, "", &placestream.Livestream_LivestreamView{
				Author: appbsky.ActorDefs_ProfileViewBasic{Did: did, Handle: "streamer.example"},
				Record: &glex.LexiconTypeDecoder{Val: &placestream.Livestream{
					Title:        "Live",
					Url:          tc.url,
					CanonicalUrl: tc.canonicalURL,
				}},
			}, nil, nil))

			payload := <-payloads
			require.Len(t, payload.Embeds, 1)
			require.Equal(t, tc.wantURL, payload.Embeds[0].URL)
			require.Equal(t, tc.wantTitle, payload.Embeds[0].Title)
		})
	}
}
