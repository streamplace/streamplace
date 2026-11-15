package spaces

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/stretchr/testify/require"
)

// delegating stands in for the user's OAuth session.
type delegating struct{}

func (delegating) Do(_ context.Context, _, _, method string, params map[string]any, _, out any) error {
	if method != "com.atproto.space.getDelegationToken" || params["space"] != PersonalSpaceURI("did:plc:example") {
		return rpcError("InvalidRequest")
	}
	return json.Unmarshal([]byte(`{"token":"delegation"}`), out)
}

func TestFollow(t *testing.T) {
	const did = "did:plc:example"
	space := PersonalSpaceURI(did)
	expires := time.Now().Add(10 * time.Minute).Truncate(time.Second)
	// A PDS answers with a JWT; only its expiry matters here.
	credential := func(claims string) string {
		return "e30." + base64.RawURLEncoding.EncodeToString([]byte(claims)) + ".sig"
	}
	answer := credential(fmt.Sprintf(`{"iss":%q,"exp":%d}`, did, expires.Unix()))
	pds := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/xrpc/com.atproto.space.authorizeSpaceSync", r.URL.Path)
		var body struct {
			Space           string `json:"space"`
			CredentialKey   string `json:"credentialKey"`
			DelegationToken string `json:"delegationToken"`
			Notify          struct {
				Endpoint string `json:"endpoint"`
				Token    string `json:"token"`
			} `json:"notify"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, space, body.Space)
		require.Equal(t, "delegation", body.DelegationToken)
		key, err := atcrypto.ParsePublicDIDKey(body.CredentialKey)
		require.NoError(t, err)
		require.IsType(t, &atcrypto.PublicKeyP256{}, key)
		require.Equal(t, "https://node.example", body.Notify.Endpoint)
		// The PDS sends this token back, so it must be the one the node accepts.
		require.Equal(t, NotifyToken(space), body.Notify.Token)
		_, _ = w.Write([]byte(`{"credential":"` + answer + `"}`))
	}))
	defer pds.Close()
	client := &xrpc.Client{Host: pds.URL, Client: pds.Client()}
	until, err := Follow(t.Context(), delegating{}, client, did, "https://node.example")
	require.NoError(t, err)
	require.True(t, until.Equal(expires))

	// Without an expiry the node cannot know when to ask again.
	answer = credential(`{}`)
	_, err = Follow(t.Context(), delegating{}, client, did, "https://node.example")
	require.Error(t, err)

	// Tokens differ by space, so one space's cannot wake another's follower.
	require.NotEqual(t, NotifyToken(space), NotifyToken(PersonalSpaceURI("did:plc:other")))
}
