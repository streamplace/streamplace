package atproto

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveStationIdentity(t *testing.T) {
	const did = "did:plc:ewvi7nxzyoun6zhxrhs64oiz"
	for _, tc := range []struct {
		name, playback, pds, wantURL, wantPDS string
		dev                                   bool
	}{
		{name: "distinct services", playback: "https://play.example/", pds: "https://repo.example", wantURL: "https://play.example", wantPDS: "https://repo.example"},
		{name: "PDS only", playback: "https://play.example/path", pds: "https://repo.example", wantPDS: "https://repo.example"},
		{name: "playback only", playback: "https://play.example", pds: "https://person:secret@repo.example", wantURL: "https://play.example"},
		{name: "no usable service", playback: "https://play.example?", pds: "https://repo.example#"},
		{name: "loopback without development opt-in", playback: "http://127.0.0.1:1234", pds: "http://[::1]:9876"},
		{name: "numeric development loopback", playback: "http://127.0.0.1:1234", pds: "http://[::1]:9876", dev: true, wantURL: "http://127.0.0.1:1234", wantPDS: "http://[::1]:9876"},
		{name: "development does not permit other HTTP hosts", playback: "http://localhost:1234", pds: "http://192.168.0.1:9876", dev: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := "false"
			if tc.dev {
				value = "true"
			}
			t.Setenv("SP_TRUST_PRIVATE_NETWORK", value)
			plc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/"+did {
					http.NotFound(w, r)
					return
				}
				if err := json.NewEncoder(w).Encode(map[string]any{
					"id": did,
					"service": []map[string]string{
						{"id": "did:web:foreign.example#streamplace", "type": "StreamplaceServer", "serviceEndpoint": "https://foreign.example"},
						{"id": "#streamplace", "type": "StreamplaceServer", "serviceEndpoint": tc.playback},
						{"id": did + "#atproto_pds", "type": "AtprotoPersonalDataServer", "serviceEndpoint": tc.pds},
					},
				}); err != nil {
					t.Error(err)
				}
			}))
			t.Cleanup(plc.Close)

			got, err := ResolveStationIdentity(t.Context(), plc.URL, did)
			if tc.wantURL == "" && tc.wantPDS == "" {
				require.Error(t, err)
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, &StationIdentity{DID: did, PDS: tc.wantPDS, URL: tc.wantURL}, got)
		})
	}
}
