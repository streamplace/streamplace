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
		host := a.CLI.BroadcasterHost
		_, pub, err := a.StatefulDB.EnsurePublisherKey(ctx)
		if err != nil {
			log.Error(ctx, "could not get publisher key", "error", err)
			http.Error(w, "could not get publisher key", http.StatusInternalServerError)
			return
		}
		didJSON := atproto.DIDDoc(host, pub)
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
		did := a.CLI.AtprotoDID
		if did == "" {
			did = fmt.Sprintf("did:web:%s", a.CLI.BroadcasterHost)
		}
		_, err := fmt.Fprintf(w, "%s", did)
		if err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	}
}
