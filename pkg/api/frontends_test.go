package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// This build bundles only the app frontend: no cookie selects anything
// else, and a browser still carrying the old opt-in cookie has it expired.
func TestFrontendSetPick(t *testing.T) {
	set := &frontendSet{app: tagHandler("app")}

	t.Run("no cookie: the app, and no cookie written", func(t *testing.T) {
		rec := servePick(set, httptest.NewRequest("GET", "/", nil))
		require.Equal(t, "app", rec.Body.String())
		require.Empty(t, rec.Result().Cookies())
	})

	for _, value := range []string{"1", "0", "anything"} {
		t.Run("cookie "+value+": the app, and the cookie expired", func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.AddCookie(&http.Cookie{Name: "sp_web_beta", Value: value})
			rec := servePick(set, r)
			require.Equal(t, "app", rec.Body.String())
			cookies := rec.Result().Cookies()
			require.Len(t, cookies, 1)
			require.Equal(t, "sp_web_beta", cookies[0].Name)
			require.Equal(t, "", cookies[0].Value)
			require.Negative(t, cookies[0].MaxAge)
			require.Equal(t, "/", cookies[0].Path)
		})
	}
}

func tagHandler(tag string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(tag))
	}
}

func servePick(set *frontendSet, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	set.pick(rec, r)(rec, r)
	return rec
}
