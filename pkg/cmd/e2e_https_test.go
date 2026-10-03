package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/aqhttp"
)

func TestE2EHighPortTrustedHTTPS(t *testing.T) {
	// ProxyFromEnvironment caches its first environment snapshot process-wide.
	// Isolate it so the test works after any other HTTP tests and cannot change
	// their proxy settings.
	const marker = "STREAMPLACE_E2E_PROXY_TEST_CHILD"
	if os.Getenv(marker) != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestE2EHighPortTrustedHTTPS$")
		cmd.Env = append(os.Environ(), marker+"=1")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
		return
	}
	backend := func(text string) *httptest.Server {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, text)
		}))
		t.Cleanup(s.Close)
		return s
	}
	pds, plc, node := backend("pds"), backend("plc"), backend("node")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h, err := newE2EHTTPS("pds.invalid", "station.invalid", 0)
	require.NoError(t, err)
	defer h.Close()
	h.Serve(ctx, strings.TrimPrefix(pds.URL, "http://"), strings.TrimPrefix(plc.URL, "http://"), strings.TrimPrefix(node.URL, "http://"))
	t.Setenv("HTTPS_PROXY", h.ProxyURL())
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")
	ca, err := os.ReadFile(h.caPath)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(ca))
	transport := aqhttp.NewTrustedTransport()
	base := transport.Base.(*http.Transport)
	base.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	defer base.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	for host, want := range map[string]string{"pds.invalid": "pds", "account.pds.invalid": "pds", "plc.directory": "plc", "station.invalid": "node"} {
		resp, err := client.Get("https://" + host + "/xrpc/com.atproto.sync.getLatestCommit")
		require.NoError(t, err, host)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, resp.Body.Close())
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, want, string(body), host)
	}
}
