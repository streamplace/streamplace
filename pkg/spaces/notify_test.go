package spaces

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

// The PDS here checks the registration the way proposal 0016 has a space host
// check any credentialed request: the signature covers the credential and its
// audience, and verifies under the key the credential was issued to.
func TestRegisterNotify(t *testing.T) {
	const did = "did:plc:example"
	expires := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	var kid string
	pds := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, PersonalSpaceURI(did), body["space"])
		switch r.URL.Path {
		case "/xrpc/com.atproto.space.getSpaceCredential":
			require.Equal(t, "delegation", body["delegationToken"])
			kid = body["credentialKid"]
			_, _ = w.Write([]byte(`{"credential":"credential.jwt"}`))
		case "/xrpc/com.atproto.space.registerNotify":
			require.Equal(t, "did:web:node.example#streamplace", body["service"])
			require.Equal(t, "Atproto-Space-Credential credential.jwt", r.Header.Get("Authorization"))
			require.Equal(t, did, r.Header.Get("Atproto-Space-Audience"))
			params, ok := strings.CutPrefix(r.Header.Get("Signature-Input"), "atproto-space-credential=")
			require.True(t, ok)
			require.Equal(t, `("authorization" "atproto-space-audience");tag="atproto-space-credential"`, params)
			encoded, ok := strings.CutPrefix(r.Header.Get("Signature"), "atproto-space-credential=:")
			require.True(t, ok)
			sig, err := base64.StdEncoding.DecodeString(strings.TrimSuffix(encoded, ":"))
			require.NoError(t, err)
			key, err := atcrypto.ParsePublicDIDKey(kid)
			require.NoError(t, err)
			require.IsType(t, &atcrypto.PublicKeyP256{}, key)
			base := "\"authorization\": Atproto-Space-Credential credential.jwt\n" +
				"\"atproto-space-audience\": " + did + "\n" +
				"\"@signature-params\": " + params
			require.NoError(t, key.HashAndVerify([]byte(base), sig))
			_, _ = w.Write([]byte(`{"expiresAt":"` + expires.Format(time.RFC3339) + `"}`))
		default:
			t.Errorf("unexpected call %s", r.URL.Path)
		}
	}))
	defer pds.Close()

	until, err := RegisterNotify(t.Context(), delegating{}, xrpc.Client{Host: pds.URL, Client: pds.Client()}, did, "did:web:node.example#streamplace")
	require.NoError(t, err)
	require.True(t, until.Equal(expires))
}
