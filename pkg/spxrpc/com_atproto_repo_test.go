package spxrpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/config"
)

// A former user's DID still points at a PDS hostname now serving Streamplace.
// Exercise the configured identity directory and the real HTTP proxy.
func TestRepoProxyLoop(t *testing.T) {
	for _, method := range []string{"getRecord", "listRecords", "describeRepo"} {
		t.Run(method, func(t *testing.T) {
			dir := identity.NewMockDirectory()
			s := &Server{
				cli:               &config.CLI{ServerHost: "node.example", BroadcasterHost: "node.example"},
				identityDirectory: func() identity.Directory { return &dir },
			}
			e := echo.New()
			e.Use(s.ContextPreservingMiddleware())
			require.NoError(t, s.RegisterHandlersComatproto(e))

			const did = "did:plc:ho26ynsdw2ey7l56xx4owjzd"
			var requests atomic.Int32
			upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Bound the broken implementation so a regression never exhausts
				// sockets or leaves recursive requests running after the test.
				if requests.Add(1) > 4 {
					w.WriteHeader(http.StatusLoopDetected)
					return
				}
				e.ServeHTTP(w, r)
			}))
			defer upstream.Close()
			service := upstream.URL
			dir.Insert(identity.Identity{
				DID:    syntax.DID(did),
				Handle: syntax.HandleInvalid,
				Services: map[string]identity.ServiceEndpoint{
					"atproto_pds": {Type: "AtprotoPersonalDataServer", URL: service},
				},
			})
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
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.ServeHTTP(w, r)
	}))
	defer upstream.Close()
	service := upstream.URL
	cli, router := newSyncTestNode(t, "node.example", "node.example")
	e = router
	did := cli.ServerDID()
	commitTestRecord(t, cli, "place.stream.live.viewerCount", "streamer")

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

func TestRepoProxyConfiguredDirectory(t *testing.T) {
	const did = "did:plc:ho26ynsdw2ey7l56xx4owjzd"
	const recordURI = "at://" + did + "/app.bsky.actor.profile/self"
	var service string
	var identityRequests, recordRequests atomic.Int32
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/" + did:
			identityRequests.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": did,
				"service": []map[string]string{
					{"id": "#atproto_pds", "type": "AtprotoPersonalDataServer", "serviceEndpoint": service},
					{"id": "#streamplace", "type": "Streamplace", "serviceEndpoint": "https://station.example"},
				},
			})
		case "/xrpc/com.atproto.repo.getRecord":
			recordRequests.Add(1)
			require.Equal(t, did, r.URL.Query().Get("repo"))
			require.Equal(t, "app.bsky.actor.profile", r.URL.Query().Get("collection"))
			require.Equal(t, "self", r.URL.Query().Get("rkey"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"uri": recordURI,
				"value": map[string]string{
					"$type": "app.bsky.actor.profile", "displayName": "Configured directory",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	service = upstream.URL
	repoTestClients(t, upstream)
	dir := &identity.BaseDirectory{PLCURL: upstream.URL, HTTPClient: *upstream.Client()}
	s := &Server{
		cli:               &config.CLI{ServerHost: "station.example", BroadcasterHost: "node.example"},
		identityDirectory: func() identity.Directory { return dir },
	}

	record, err := s.handleComAtprotoRepoGetRecord(context.Background(), "", "app.bsky.actor.profile", did, "self")
	require.NoError(t, err)
	require.Equal(t, recordURI, record.Uri)
	require.EqualValues(t, 1, identityRequests.Load(), "DID resolution must use the configured PLC directory")
	require.EqualValues(t, 1, recordRequests.Load(), "the account PDS, not the station service, must serve the record")
}

func TestResolveRepoServiceConfiguredIdentity(t *testing.T) {
	const did = "did:plc:ho26ynsdw2ey7l56xx4owjzd"
	const handle = "account.example"
	const pds = "https://pds.example"
	dir := identity.NewMockDirectory()
	dir.Insert(identity.Identity{
		DID:    syntax.DID(did),
		Handle: syntax.Handle(handle),
		Services: map[string]identity.ServiceEndpoint{
			"atproto_pds": {Type: "AtprotoPersonalDataServer", URL: pds},
		},
	})
	s := &Server{
		cli:               &config.CLI{ServerHost: "station.example", BroadcasterHost: "node.example"},
		identityDirectory: func() identity.Directory { return &dir },
	}
	for _, repo := range []string{did, handle} {
		t.Run(repo, func(t *testing.T) {
			resolvedDID, service, resolvedHandle, err := s.resolveRepoService(context.Background(), repo)
			require.NoError(t, err)
			require.Equal(t, did, resolvedDID)
			require.Equal(t, pds, service)
			require.Equal(t, handle, resolvedHandle)
			local, service, err := s.isLocalPDS(context.Background(), repo)
			require.NoError(t, err)
			require.False(t, local, "the station must not treat an upstream account's PDS as a local repository")
			require.Equal(t, pds, service)
		})
	}
}

func TestIsLocalPDSOwnDIDsWithoutLookup(t *testing.T) {
	for _, serverHost := range []string{"", "node.example", "station.example"} {
		t.Run(serverHost, func(t *testing.T) {
			s := &Server{
				cli: &config.CLI{ServerHost: serverHost, BroadcasterHost: "node.example"},
				identityDirectory: func() identity.Directory {
					t.Fatal("own repository routing must not consult the identity directory")
					return nil
				},
			}
			for _, did := range []string{s.cli.ServerDID(), s.cli.BroadcasterDID()} {
				local, service, err := s.isLocalPDS(context.Background(), did)
				require.NoError(t, err)
				require.True(t, local)
				require.Empty(t, service)
			}
		})
	}
}

func repoTestClients(t *testing.T, upstream *httptest.Server) {
	t.Helper()
	originalClient := aqhttp.Client
	aqhttp.Client = *upstream.Client()
	t.Cleanup(func() {
		aqhttp.Client = originalClient
	})
}
