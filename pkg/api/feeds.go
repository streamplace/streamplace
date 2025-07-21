package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/log"
)

func (a *StreamplaceAPI) HandleDidJSON(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		host := a.CLI.PublicHost
		didJSON := atproto.DIDDoc(host)
		w.WriteHeader(200)
		w.Header().Set("Content-Type", "application/json")
		bs, err := json.Marshal(didJSON)
		if err != nil {
			log.Error(ctx, "could not marshal did json", "error", err)
			return
		}
		if _, err := w.Write(bs); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	}
}

func (a *StreamplaceAPI) HandleAtprotoDID(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		host := a.CLI.PublicHost
		_, err := fmt.Fprintf(w, "did:web:%s", host)
		if err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	}
}
