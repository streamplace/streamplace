package api

import (
	"context"
	"encoding/json"
	"net/http"

	"stream.place/streamplace/pkg/errors"
)

func (a *StreamplaceAPI) HandlePush(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if a.CLI.UpstreamStation == "" {
			errors.WriteHTTPNotFound(w, "push status is only available on single-user nodes", nil)
			return
		}
		if a.PushStatus == nil {
			errors.WriteHTTPNotImplemented(w, "push status is unavailable", nil)
			return
		}
		data, err := json.Marshal(a.PushStatus())
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "could not encode push status", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(data)
	}
}
