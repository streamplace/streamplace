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

// requestHost is the public host the client used: the edge proxy's
// X-Forwarded-Host when present, the Host header otherwise.
func requestHost(req *http.Request) string {
	if h := req.Header.Get("X-Forwarded-Host"); h != "" {
		return h
	}
	return req.Host
}

func (a *StreamplaceAPI) HandleDidJSON(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		// Route to the correct virtual PDS based on Host header.
		// Single-node (ServerHost == BroadcasterHost): always server PDS.
		// Multi-node: server PDS on ServerHost, broadcaster PDS on BroadcasterHost.
		host := a.CLI.ServerHost
		pubKey := atproto.ServerPubMultibase
		if a.CLI.ServerHost != a.CLI.BroadcasterHost && requestHost(req) != a.CLI.ServerHost {
			host = a.CLI.BroadcasterHost
			pubKey = atproto.LexiconPubMultibase
		}
		didJSON := atproto.DIDDoc(host, pubKey)
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
		// An explicit override takes precedence over host-based DID routing.
		did := a.CLI.AtprotoDID
		if did == "" {
			did = a.CLI.ServerDID()
			if a.CLI.ServerHost != a.CLI.BroadcasterHost && requestHost(req) != a.CLI.ServerHost {
				did = a.CLI.BroadcasterDID()
			}
		}
		_, err := fmt.Fprintf(w, "%s", did)
		if err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	}
}
