package statedb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/bluesky-social/indigo/xrpc"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
)

// An account the node has credentials for gets a client from a session
// created with the app password at the account's PDS, reused across calls;
// an account it has no credentials for and no stored session for gets none.
func TestUserXrpcClientFromCredentials(t *testing.T) {
	var logins atomic.Int32
	var seen comatproto.ServerCreateSession_Input
	pds := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.createSession":
			logins.Add(1)
			require.NoError(t, json.NewDecoder(r.Body).Decode(&seen))
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"did": "did:plc:streamer", "handle": "streamer.test", "accessJwt": "access", "refreshJwt": "refresh"})
		case "/xrpc/com.atproto.repo.getRecord":
			require.Equal(t, "Bearer access", r.Header.Get("Authorization"), "the session's token signs the call")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"uri": "at://did:plc:streamer/x/y", "cid": "bafy", "value": map[string]any{"$type": "x"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer pds.Close()

	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	require.NoError(t, mod.UpdateRepo(&model.Repo{DID: "did:plc:streamer", Handle: "streamer.test", PDS: pds.URL}))
	cli := &config.CLI{DBURL: ":memory:", DevAccountCreds: map[string]string{"did:plc:streamer": "xxxx-xxxx-xxxx-xxxx"}}
	state, err := MakeDB(t.Context(), cli, nil, mod)
	require.NoError(t, err)

	require.True(t, state.HasUserSession("did:plc:streamer"))
	require.False(t, state.HasUserSession("did:plc:nobody"))

	ctx := context.Background()
	client, err := state.UserXrpcClient(ctx, "did:plc:streamer")
	require.NoError(t, err)
	require.Equal(t, "did:plc:streamer", seen.Identifier, "logs in as the DID")
	require.Equal(t, "xxxx-xxxx-xxxx-xxxx", seen.Password)
	var out map[string]any
	require.NoError(t, client.Do(ctx, xrpc.Query, "application/json", "com.atproto.repo.getRecord", map[string]any{"repo": "did:plc:streamer", "collection": "x", "rkey": "y"}, nil, &out))

	again, err := state.UserXrpcClient(ctx, "did:plc:streamer")
	require.NoError(t, err)
	require.Equal(t, int32(1), logins.Load(), "the session is reused, not re-created per call")
	require.NotNil(t, again)

	state.ForgetPasswordSession("did:plc:streamer")
	_, err = state.UserXrpcClient(ctx, "did:plc:streamer")
	require.NoError(t, err)
	require.Equal(t, int32(2), logins.Load(), "forgotten, it logs in again")

	_, err = state.UserXrpcClient(ctx, "did:plc:nobody")
	require.Error(t, err, "no credentials and no stored session")
}
