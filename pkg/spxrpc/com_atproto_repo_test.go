package spxrpc

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/config"
)

// A former user's DID still points at a PDS hostname now serving Streamplace.
// Exercise DID resolution and the real HTTP proxy, not a mocked upstream call.
func TestRepoProxyLoop(t *testing.T) {
	for _, method := range []string{"getRecord", "listRecords", "describeRepo"} {
		t.Run(method, func(t *testing.T) {
			s := &Server{cli: &config.CLI{ServerHost: "node.example", BroadcasterHost: "node.example"}}
			e := echo.New()
			e.Use(s.ContextPreservingMiddleware())
			require.NoError(t, s.RegisterHandlersComatproto(e))

			var did, service string
			var requests atomic.Int32
			upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/"+did {
					_ = json.NewEncoder(w).Encode(map[string]any{
						"id": did,
						"service": []map[string]string{{
							"id": "#atproto_pds", "type": "AtprotoPersonalDataServer", "serviceEndpoint": service,
						}},
					})
					return
				}
				// Bound the broken implementation so a regression never exhausts
				// sockets or leaves recursive requests running after the test.
				if requests.Add(1) > 4 {
					w.WriteHeader(http.StatusLoopDetected)
					return
				}
				e.ServeHTTP(w, r)
			}))
			defer upstream.Close()
			service = upstream.URL
			did = "did:plc:ho26ynsdw2ey7l56xx4owjzd"
			repoTestClients(t, upstream)
			params := url.Values{"repo": {did}, "collection": {"app.bsky.actor.profile"}, "rkey": {"self"}}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, service+"/xrpc/com.atproto.repo."+method+"?"+params.Encode(), nil)
			require.NoError(t, err)
			resp, err := upstream.Client().Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, http.StatusLoopDetected, resp.StatusCode)
			require.EqualValues(t, 2, requests.Load(), "the forwarded request must not forward again")
		})
	}
}

// A proxied read of a node's actual repo must still be served locally. Rejecting
// every marked request at ingress would break reads between Streamplace nodes.
func TestRepoProxyLocalRecord(t *testing.T) {
	var e *echo.Echo
	var did, service string
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/did.json" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": did,
				"service": []map[string]string{{
					"id": "#atproto_pds", "type": "AtprotoPersonalDataServer", "serviceEndpoint": service,
				}},
			})
			return
		}
		e.ServeHTTP(w, r)
	}))
	defer upstream.Close()
	service = upstream.URL
	cli, router := newSyncTestNode(t, "node.example", "node.example")
	e = router
	did = cli.ServerDID()
	commitTestRecord(t, cli, "place.stream.live.viewerCount", "streamer")

	repoTestClients(t, upstream)
	params := url.Values{"repo": {did}, "collection": {"place.stream.live.viewerCount"}, "rkey": {"streamer"}}
	req, err := http.NewRequest(http.MethodGet, service+"/xrpc/com.atproto.repo.getRecord?"+params.Encode(), nil)
	require.NoError(t, err)
	req.Header.Set("X-Streamplace-Repo-Proxy", "true")
	resp, err := upstream.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var record struct {
		URI   string `json:"uri"`
		Value struct {
			Count int `json:"count"`
		} `json:"value"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&record))
	require.Equal(t, "at://"+did+"/place.stream.live.viewerCount/streamer", record.URI)
	require.Equal(t, 7, record.Value.Count)
}

func repoTestClients(t *testing.T, upstream *httptest.Server) {
	t.Helper()
	originalClient, originalDefault := aqhttp.Client, http.DefaultClient
	aqhttp.Client = *upstream.Client()
	// oatproxy's service resolver currently uses http.DefaultClient rather
	// than its client argument. Route its real HTTPS lookups to this fixture.
	transport := upstream.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.ServerName = "127.0.0.1"
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, upstream.Listener.Addr().String())
	}
	http.DefaultClient = &http.Client{Transport: transport, Timeout: 5 * time.Second}
	t.Cleanup(func() {
		aqhttp.Client, http.DefaultClient = originalClient, originalDefault
		transport.CloseIdleConnections()
	})
}
