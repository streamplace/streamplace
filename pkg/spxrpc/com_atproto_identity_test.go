package spxrpc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestResolveIdentity(t *testing.T) {
	const did = "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa"
	for _, tc := range []struct {
		name       string
		identifier string
		plcStatus  int
		want       int
		message    string
	}{
		{name: "reserved handle", identifier: "missing.example.invalid", want: http.StatusNotFound, message: "HandleNotFound"},
		{name: "malformed identifier", identifier: "not an account", want: http.StatusBadRequest, message: "InvalidRequest"},
		{name: "unregistered DID", identifier: did, plcStatus: http.StatusNotFound, want: http.StatusNotFound, message: "DidNotFound"},
		{name: "directory outage", identifier: did, plcStatus: http.StatusServiceUnavailable, want: http.StatusInternalServerError},
		{name: "PDS outage does not hide an existing DID", identifier: did, plcStatus: http.StatusOK, want: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var pdsRequests atomic.Int32
			pds := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				pdsRequests.Add(1)
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
			defer pds.Close()
			plc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.plcStatus)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"id":          did,
					"alsoKnownAs": []string{"at://missing.example.invalid"},
					"service": []map[string]string{
						{"id": "#atproto_pds", "type": "AtprotoPersonalDataServer", "serviceEndpoint": pds.URL},
					},
				})
			}))
			defer plc.Close()
			dir := &identity.BaseDirectory{PLCURL: plc.URL, HTTPClient: *plc.Client()}
			s := &Server{identityDirectory: func() identity.Directory { return dir }}
			e := echo.New()
			e.Use(s.ErrorHandlingMiddleware())
			e.Use(s.ContextPreservingMiddleware())
			require.NoError(t, s.RegisterHandlersComatproto(e))

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/xrpc/com.atproto.identity.resolveIdentity?"+url.Values{"identifier": {tc.identifier}}.Encode(), nil))
			require.Equal(t, tc.want, rec.Code, rec.Body.String())
			var body struct {
				Did     string `json:"did"`
				Message string `json:"message"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			if tc.message != "" {
				require.Equal(t, tc.message, body.Message)
			}
			if tc.want == http.StatusOK {
				require.Equal(t, did, body.Did)
			}
			require.Zero(t, pdsRequests.Load(), "identity resolution must not depend on PDS availability")
		})
	}
}
