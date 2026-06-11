package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// frontendSet.pick decides which per-frontend handler serves a given
// request. Cookie wins unless the operator forced web via --frontend=web.
func TestFrontendSetPick(t *testing.T) {
	t.Run("no cookie, not forced, returns app", func(t *testing.T) {
		got := serve(&frontendSet{app: appHandler("a"), web: appHandler("w")}, nil, false)
		require.Equal(t, "a", got)
	})

	t.Run("cookie = 1, returns web", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		r.AddCookie(&http.Cookie{Name: "sp_web_beta", Value: "1"})
		got := serveRaw(&frontendSet{app: appHandler("a"), web: appHandler("w")}, r, false)
		require.Equal(t, "w", got)
	})

	t.Run("cookie = 0, returns app", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		r.AddCookie(&http.Cookie{Name: "sp_web_beta", Value: "0"})
		got := serveRaw(&frontendSet{app: appHandler("a"), web: appHandler("w")}, r, false)
		require.Equal(t, "a", got)
	})

	t.Run("no cookie, forceWeb is true, returns web", func(t *testing.T) {
		got := serve(&frontendSet{app: appHandler("a"), web: appHandler("w")}, nil, true)
		require.Equal(t, "w", got)
	})

	t.Run("cookie = 0, forceWeb is true, cookie ignored", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		r.AddCookie(&http.Cookie{Name: "sp_web_beta", Value: "0"})
		got := serveRaw(&frontendSet{app: appHandler("a"), web: appHandler("w")}, r, true)
		require.Equal(t, "w", got)
	})
}

// appHandler returns a handler that writes the given tag so tests can
// assert which frontend was picked.
func appHandler(tag string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(tag))
	}
}

// serve runs pick with a fresh request. Useful for cases that don't need
// a cookie.
func serve(set *frontendSet, _ *http.Cookie, forceWeb bool) string {
	return serveRaw(set, httptest.NewRequest("GET", "/", nil), forceWeb)
}

// serveRaw runs pick with the given request and reads back the body.
func serveRaw(set *frontendSet, r *http.Request, forceWeb bool) string {
	rec := httptest.NewRecorder()
	set.pick(r, forceWeb)(rec, r)
	return rec.Body.String()
}
