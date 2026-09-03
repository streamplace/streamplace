package api

import (
	"net/http"

	"github.com/julienschmidt/httprouter"

	"stream.place/streamplace/pkg/access"
	apierrors "stream.place/streamplace/pkg/errors"
)

// viewerAllowed reports whether a request on a non-XRPC route comes from an
// account holding the viewer role. These routes carry no Authorization
// header (browsers hit them from <img>, WHEP fetches and websockets), so the
// identity comes from the viewer cookie the XRPC gate minted. On an open
// node everything passes.
func (a *StreamplaceAPI) viewerAllowed(r *http.Request) bool {
	ac := a.CLI.Access
	if ac == nil || ac.Modes()[access.RoleViewer] == access.ModeOpen {
		return true
	}
	did, ok := ac.ViewerFromCookie(r)
	if !ok {
		return false
	}
	return ac.Allowed(r.Context(), did, access.RoleViewer)
}

func (a *StreamplaceAPI) writeViewerDenied(w http.ResponseWriter) {
	apierrors.WriteHTTPForbidden(w, "ViewerRequired: this node is private", nil)
}

// viewerGate wraps an httprouter handle with the viewer check.
func (a *StreamplaceAPI) viewerGate(next httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		if !a.viewerAllowed(r) {
			a.writeViewerDenied(w)
			return
		}
		next(w, r, p)
	}
}

// viewerGateFunc is viewerGate for plain http.HandlerFuncs.
func (a *StreamplaceAPI) viewerGateFunc(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.viewerAllowed(r) {
			a.writeViewerDenied(w)
			return
		}
		next(w, r)
	}
}
