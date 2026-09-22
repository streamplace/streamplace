package api

import (
	"context"
	_ "embed"
	"html"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"stream.place/streamplace/pkg/log"
)

//go:embed dev-frontend-proxy.html
var devFrontendProxyHTML string

// devFrontendProxyStatusPath is handled locally by the node and never proxied,
// so the loading page has something to poll while the dev server is down.
const devFrontendProxyStatusPath = "/__dev_frontend_proxy/status"

// upstreamPortUp reports whether the dev server's TCP port accepts connections.
// That is exactly the condition whose failure makes httputil.ReverseProxy answer
// with a bare 502, so it is what the loading page polls for.
func upstreamPortUp(u *url.URL) bool {
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(u.Hostname(), port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// HandleDevFrontendProxyStatus answers 200 once the proxied dev server is
// accepting connections and 503 until then.
func (a *StreamplaceAPI) HandleDevFrontendProxyStatus(ctx context.Context, u *url.URL) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		w.Header().Set("Cache-Control", "no-store")
		if upstreamPortUp(u) {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

// renderDevFrontendProxyPage bakes the proxy destination into the loading page
// once at startup.
func renderDevFrontendProxyPage(upstream string) []byte {
	return []byte(strings.Replace(devFrontendProxyHTML, "UPSTREAM_REPLACE_ME", html.EscapeString(upstream), 1))
}

// isDocumentNavigation reports whether a request is a browser page load that
// should get the loading page. Assets, fetches, and XRPC calls keep a plain
// error so tooling fails loudly instead of trying to parse HTML.
func isDocumentNavigation(r *http.Request) bool {
	return r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/html")
}

// devFrontendProxyErrorHandler replaces the reverse proxy's default 502 with a
// page that polls until the dev server is up and then reloads. Without it, a
// slow `pnpm app start` leaves the browser on a flat error page until the
// developer manually retries.
func devFrontendProxyErrorHandler(ctx context.Context, page []byte) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		if !isDocumentNavigation(r) {
			http.Error(w, "frontend dev server unavailable", http.StatusServiceUnavailable)
			return
		}
		log.Debug(ctx, "frontend dev server unreachable, serving loading page", "path", r.URL.Path, "error", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusServiceUnavailable)
		if _, werr := w.Write(page); werr != nil {
			log.Error(ctx, "error writing dev frontend proxy loading page", "error", werr)
		}
	}
}
