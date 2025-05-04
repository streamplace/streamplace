package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/streamplace"
)

func (a *StreamplaceAPI) HandleAccountLogin(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		bs, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer req.Body.Close()
		var input streamplace.AccountLogin_Input
		if err := json.Unmarshal(bs, &input); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		output, err := atproto.Login(ctx, a.CLI, &input)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		bs, err = json.Marshal(output)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		w.Write(bs)
	}
}
