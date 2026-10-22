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
	for _, plcHost := range []string{e2ePLCHost, "plc.pds.example.test"} {
		t.Run(plcHost, func(t *testing.T) {
			h, err := newE2EHTTPS("pds.example.test", "station.example.test", plcHost, 0)
			require.NoError(t, err)
			t.Cleanup(h.Close)

			caPEM, err := os.ReadFile(h.caPath)
			require.NoError(t, err)
			roots := x509.NewCertPool()
			require.True(t, roots.AppendCertsFromPEM(caPEM), "harness CA could not be loaded")
			backend := func(body string) string {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = io.WriteString(w, body+" "+r.Host+" "+r.URL.Path)
				}))
				t.Cleanup(srv.Close)
				return strings.TrimPrefix(srv.URL, "http://")
			}
			pdsAddr, plcAddr, nodeAddr := backend("pds"), backend("plc"), backend("node")
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			h.Serve(ctx, pdsAddr, plcAddr, nodeAddr)

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
					// The alias deliberately sits below the PDS hostname: exact
					// PLC routing must take precedence over wildcard PDS routing.
					for host, want := range map[string]string{
						h.pdsHost:              "pds",
						"account." + h.pdsHost: "pds",
						h.stationHost:          "node",
						e2ePLCHost:             "plc",
						plcHost:                "plc",
					} {
						resp, err := client.Get("https://" + host + "/did:plc:test")
						require.NoError(t, err, "GET %s", host)
						body, err := io.ReadAll(resp.Body)
						resp.Body.Close()
						require.NoError(t, err)
						require.Equal(t, want+" "+host+" /did:plc:test", string(body), "GET %s", host)
					}
				})
			}
		})
	}
}
