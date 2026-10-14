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
	var stationURL, firehoseURL string
	var advertiseRelay = true
	station := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/did.json" {
			t.Errorf("unexpected discovery request: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		relay := firehoseURL
		if !advertiseRelay {
			relay = ""
		}
		doc := DIDDoc("station.streamplace.team", "zQ3shRvJrnKp4SWvoxkFyZqijahHcmvJLdViKzkwQKj7cWxVm", relay)
		if err := json.NewEncoder(w).Encode(doc); err != nil {
			t.Error(err)
		}
	}))
	defer station.Close()
	stationURL = "https://station.streamplace.team"
	firehoseURL = "moqt://127.0.0.1:4443"
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
	require.Equal(t, stationURL, resolved.URL)
	require.Equal(t, firehoseURL, resolved.FirehoseURL)

	// A missing/bad advertisement must not quietly select the global firehose.
	advertiseRelay = false
	_, err = ResolveUpstreamStation(context.Background(), "", did)
	require.Error(t, err)
	advertiseRelay = true
	firehoseURL = "wss://bsky.network"
	_, err = ResolveUpstreamStation(context.Background(), "", did)
	require.Error(t, err)
}

func TestUpstreamRelayNeverFallsBack(t *testing.T) {
	atsync := &ATProtoSynchronizer{CLI: &config.CLI{
		UpstreamStation: "did:web:station.example",
		RelayHost:       "wss://bsky.network",
		HTTPAddr:        "127.0.0.1:38080",
	}}
	require.Empty(t, atsync.relayHosts())
	atsync.UpstreamStation = &UpstreamStation{FirehoseURL: "moqt://station.example"}
	require.Equal(t, []string{"moqt://station.example"}, atsync.relayHosts())
}
