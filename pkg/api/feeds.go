package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"stream.place/streamplace/pkg/log"
)

func (a *StreamplaceAPI) HandleDidJson(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		host := req.Host
		didJSON := map[string]any{
			"@context": []string{
				"https://www.w3.org/ns/did/v1",
			},
			"id": fmt.Sprintf("did:web:%s", host),
			"service": []map[string]any{
				{
					"id":              "#bsky_fg",
					"type":            "BskyFeedGenerator",
					"serviceEndpoint": fmt.Sprintf("https://%s", host),
				},
			},
		}
		w.WriteHeader(200)
		w.Header().Set("Content-Type", "application/json")
		bs, err := json.Marshal(didJSON)
		if err != nil {
			log.Error(ctx, "could not marshal did json", "error", err)
			return
		}
		w.Write(bs)
	}
}
