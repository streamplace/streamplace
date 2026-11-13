package api

import (
	"context"
	"net/http"

	apierrors "stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
)

// HandleDesktopOBS receives OBS's Get Stream Key navigation in the system browser.
func (a *StreamplaceAPI) HandleDesktopOBS(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if a.CLI.UpstreamStation == "" {
			apierrors.WriteHTTPNotFound(w, "OBS requests are only available on single-user nodes", nil)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "none" {
			apierrors.WriteHTTPForbidden(w, "OBS requests must be opened directly", nil)
			return
		}
		a.desktopObsPending.Store(true)
		writeDesktopReturn(ctx, w, "Connect OBS to Streamplace", "Finish connecting OBS in Streamplace Desktop. You can close this window.")
	}
}

// HandleDesktopOBSRequest consumes the pending request once for Desktop's poller.
func (a *StreamplaceAPI) HandleDesktopOBSRequest(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if a.CLI.UpstreamStation == "" {
			apierrors.WriteHTTPNotFound(w, "OBS requests are only available on single-user nodes", nil)
			return
		}
		// No Content while idle, so the steady poll doesn't log as a client error.
		if !a.desktopObsPending.Swap(false) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte("{}")); err != nil {
			log.Error(ctx, "error writing OBS request response", "error", err)
		}
	}
}
