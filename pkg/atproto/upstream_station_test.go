package atproto

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/config"
)

func TestResolveUpstreamStation(t *testing.T) {
	stationEndpoint := "https://station.streamplace.team"
	station := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/did.json" {
			t.Errorf("unexpected discovery request: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		doc := DIDDoc("station.streamplace.team", "zQ3shRvJrnKp4SWvoxkFyZqijahHcmvJLdViKzkwQKj7cWxVm")
		for _, svc := range doc["service"].([]map[string]any) {
			if svc["id"] == "#"+StreamplaceServiceID {
				svc["serviceEndpoint"] = stationEndpoint
			}
		}
		if err := json.NewEncoder(w).Encode(doc); err != nil {
			t.Error(err)
		}
	}))
	defer station.Close()
	original := aqhttp.Client
	client := station.Client()
	transport := client.Transport.(*http.Transport).Clone()
	target, err := url.Parse(station.URL)
	require.NoError(t, err)
	transport.TLSClientConfig.ServerName = target.Hostname()
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, target.Host)
	}
	client.Transport = transport
	aqhttp.Client = *client
	t.Cleanup(func() { aqhttp.Client = original })
	did := "did:web:station.streamplace.team"

	resolved, err := ResolveUpstreamStation(context.Background(), "", did)
	require.NoError(t, err)
	require.Equal(t, "https://station.streamplace.team", resolved.URL)

	// OAuth is delegated to this origin, so anything but a bare HTTPS origin
	// stops startup.
	for _, endpoint := range []string{"http://station.streamplace.team", "https://station.streamplace.team/oauth", ""} {
		stationEndpoint = endpoint
		_, err = ResolveUpstreamStation(context.Background(), "", did)
		require.Error(t, err, endpoint)
	}
}

// With an upstream station the node's own PDS firehose is left out: its
// users' records live on their own PDSes, reached through the MoQ relay.
func TestUpstreamRelayHosts(t *testing.T) {
	cli := &config.CLI{RelayHost: "moqt://streamplace.network", HTTPAddr: "127.0.0.1:38080"}
	atsync := &ATProtoSynchronizer{CLI: cli}
	require.Equal(t, []string{"moqt://streamplace.network", "ws://127.0.0.1:38080"}, atsync.relayHosts())
	cli.UpstreamStation = "did:web:station.example"
	require.Equal(t, []string{"moqt://streamplace.network"}, atsync.relayHosts())
}
