package api

import (
	"context"
	"encoding/json"
	"net/http"

	apierrors "stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
)

// VersionManifest is the node's build manifest: which build of Streamplace
// this process is serving. The frontend bundles are embedded in the binary,
// so this one document also identifies the app and web bundles, and the
// widgets they serve (see /overlay/<name>): a page that polls it can tell
// that a new deployment has replaced the one it booted against.
type VersionManifest struct {
	// Version is `git describe` for the commit the binary was built from.
	Version string `json:"version"`
	// BuildTime is that commit's timestamp, RFC3339.
	BuildTime string `json:"buildTime"`
	// UUID is a stable per-build identifier derived from BuildTime.
	UUID string `json:"uuid"`
}

// HandleVersion serves the node's build manifest. Widgets poll it and reload
// when it changes, so an operator's long-lived OBS browser source picks up a
// deploy without being restarted.
func (a *StreamplaceAPI) HandleVersion(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		build := a.CLI.Build
		if build == nil {
			apierrors.WriteHTTPInternalServerError(w, "no build information", nil)
			return
		}
		bs, err := json.Marshal(VersionManifest{
			Version:   build.Version,
			BuildTime: build.BuildTimeStr(),
			UUID:      build.UUID,
		})
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not marshal version manifest", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// The whole point is noticing a change, so a cached response would
		// pin every widget to the version it booted with.
		w.Header().Set("Cache-Control", "no-store")
		if _, err := w.Write(bs); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	}
}
