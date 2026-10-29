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

func TestInvalidHandleWebhook(t *testing.T) {
	const did = "did:plc:invalidhandlewebhook"
	// No public AppView lookup is needed for this isolated account.
	avatarCacheMutex.Lock()
	avatarCache[did] = ""
	avatarCacheMutex.Unlock()
	t.Cleanup(func() {
		avatarCacheMutex.Lock()
		delete(avatarCache, did)
		avatarCacheMutex.Unlock()
	})
	payloads := make(chan discordtypes.Payload, 2)
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
	webhook := &discordtypes.Webhook{URL: server.URL}
	author := appbsky.ActorDefs_ProfileViewBasic{Did: did, Handle: "handle.invalid"}
	require.NoError(t, SendChat(t.Context(), webhook, did, &placestream.ChatDefs_MessageView{
		Author: author,
		Record: &glex.LexiconTypeDecoder{Val: &placestream.ChatMessage{Text: "Hello"}},
	}))
	require.Equal(t, did, (<-payloads).Username)
	require.NoError(t, SendLivestream(t.Context(), webhook, "", &placestream.Livestream_LivestreamView{
		Author: author,
		Record: &glex.LexiconTypeDecoder{Val: &placestream.Livestream{Title: "Live"}},
	}, nil, nil))
	payload := <-payloads
	require.Equal(t, did, payload.Username)
	require.Equal(t, did+" is LIVE!", payload.Embeds[0].Title)
}
