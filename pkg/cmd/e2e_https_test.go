package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestE2EHTTPSPLCHostname(t *testing.T) {
	for _, tc := range []struct {
		plcHost        string
		stationCluster bool
	}{
		{plcHost: e2ePLCHost},
		{plcHost: "plc.pds.example.test", stationCluster: true},   // Takes precedence over wildcard PDS routing.
		{plcHost: "directory.example.test", stationCluster: true}, // Requires its own certificate SAN.
	} {
		plcHost := tc.plcHost
		t.Run(plcHost, func(t *testing.T) {
			h, err := newE2EHTTPS("pds.example.test", "station.example.test", plcHost, 0, tc.stationCluster)
			require.NoError(t, err)
			t.Cleanup(h.Close)

			caPEM, err := os.ReadFile(h.caPath)
			require.NoError(t, err)
			roots := x509.NewCertPool()
			require.True(t, roots.AppendCertsFromPEM(caPEM), "harness CA could not be loaded")
			backend := func(body string) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					_, _ = io.WriteString(w, body)
				}))
				t.Cleanup(srv.Close)
				return strings.TrimPrefix(srv.URL, "http://")
			}
			pdsAddr, plcAddr, nodeAddr := backend("pds"), backend("plc"), backend("node")
			var edgeAddr string
			wantEdge := "pds"
			if tc.stationCluster {
				wantEdge = "edge"
				edgeAddr = backend(wantEdge)
			}
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			h.Serve(ctx, pdsAddr, plcAddr, nodeAddr, edgeAddr)

			proxyURL, err := url.Parse(h.ProxyURL())
			require.NoError(t, err)
			for _, proxied := range []bool{false, true} {
				name := "direct"
				if proxied {
					name = "proxy"
				}
				t.Run(name, func(t *testing.T) {
					transport := &http.Transport{
						TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
					}
					if proxied {
						transport.Proxy = http.ProxyURL(proxyURL)
					} else {
						// Native clients reach the TLS listener directly via public
						// loopback DNS; use its ephemeral test port without DNS.
						transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
							return (&net.Dialer{}).DialContext(ctx, network, h.frontLn.Addr().String())
						}
					}
					t.Cleanup(transport.CloseIdleConnections)
					client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
					for host, want := range map[string]string{
						h.pdsHost:                   "pds",
						"account." + h.pdsHost:      "pds",
						"station-edge." + h.pdsHost: wantEdge,
						h.stationHost:               "node",
						e2ePLCHost:                  "plc",
						plcHost:                     "plc",
					} {
						resp, err := client.Get("https://" + host + "/did:plc:test")
						require.NoError(t, err, "GET %s", host)
						body, err := io.ReadAll(resp.Body)
						resp.Body.Close()
						require.NoError(t, err)
						require.Equal(t, want, string(body), "GET %s selected the wrong backend", host)
					}
				})
			}
		})
	}
}

func TestE2EHTTPSStationEdgePLCCollision(t *testing.T) {
	const pdsHost = "pds.example.test"
	edgeHost := "station-edge." + pdsHost
	h, err := newE2EHTTPS(pdsHost, "station.example.test", edgeHost, 0, true)
	if h != nil {
		t.Cleanup(h.Close)
	}
	require.ErrorContains(t, err, edgeHost+" conflicts")
}
