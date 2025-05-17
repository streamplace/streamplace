package api

import (
	"context"
	_ "embed"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	apierrors "stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
)

//go:embed app-return.html
var appReturnHTML string

func (a *StreamplaceAPI) HandleAppReturn(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		if a.CLI.AppBundleID == "" {
			apierrors.WriteHTTPNotImplemented(w, "server has no --app-bundle-id set", nil)
			return
		}
		html := strings.Replace(appReturnHTML, "APP_BUNDLE_ID_REPLACE_ME", a.CLI.AppBundleID, 1)
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte(html)); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	}
}
