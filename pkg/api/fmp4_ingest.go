package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
)

// The Streamplace OBS plugin asks for its stream configuration with the same
// request and response shapes as OBS's Enhanced RTMP multitrack video
// configuration (GoLiveApi, see obs-studio's
// frontend/utility/models/multitrack-video.hpp), so one endpoint can configure
// both. Only the fields the node reads or sets are modeled here.
type clientConfigurationRequest struct {
	Authentication string `json:"authentication"`
}

type clientConfiguration struct {
	Meta            clientConfigurationMeta   `json:"meta"`
	Status          clientConfigurationStatus `json:"status"`
	IngestEndpoints []ingestEndpoint          `json:"ingest_endpoints,omitempty"`
}

type clientConfigurationMeta struct {
	Service       string `json:"service"`
	SchemaVersion string `json:"schema_version"`
	ConfigID      string `json:"config_id"`
}

type clientConfigurationStatus struct {
	Result   string `json:"result"`
	HTMLEnUS string `json:"html_en_us,omitempty"`
}

type ingestEndpoint struct {
	Protocol    string `json:"protocol"`
	URLTemplate string `json:"url_template"`
}

// HandleClientConfiguration answers the plugin's configuration request. A bad
// stream key is reported in status, as GoLiveApi clients expect, so OBS shows
// the message. The response names no encoders, so the plugin keeps the
// encoders from OBS's output settings: one video and one audio track, which is
// what the node's segment pipeline handles today.
func (a *StreamplaceAPI) HandleClientConfiguration(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req clientConfigurationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errors.WriteHTTPBadRequest(w, "invalid configuration request", err)
			return
		}
		config := clientConfiguration{
			Meta: clientConfigurationMeta{
				Service:       "Streamplace",
				SchemaVersion: "2025-01-25",
				ConfigID:      uuid.NewString(),
			},
			Status: clientConfigurationStatus{Result: "success"},
		}
		if problem := a.streamKeyProblem(r.Context(), req.Authentication); problem != "" {
			config.Status = clientConfigurationStatus{Result: "error", HTMLEnUS: problem}
		} else {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			config.IngestEndpoints = []ingestEndpoint{{
				Protocol:    "FMP4",
				URLTemplate: fmt.Sprintf("%s://%s/api/ingest/fmp4/{stream_key}", scheme, r.Host),
			}}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(config); err != nil {
			log.Error(ctx, "error writing client configuration", "error", err)
		}
	}
}

// streamKeyProblem explains to the streamer why key may not stream, or returns
// "" if it may.
func (a *StreamplaceAPI) streamKeyProblem(ctx context.Context, key string) string {
	ms, err := a.MakeMediaSigner(ctx, key)
	if err != nil {
		return "This stream key isn't valid. Copy a fresh one from your Streamplace dashboard."
	}
	if err := a.checkBanned(ctx, ms.Streamer()); err != nil {
		return "This account can't stream right now."
	}
	return ""
}

// HandleFMP4Ingest accepts the OBS plugin's push: a chunked POST of fragmented
// MP4 that is signed as-is (media.IngestTransportFMP4Direct).
func (a *StreamplaceAPI) HandleFMP4Ingest(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		mediaSigner, err := a.MakeMediaSigner(r.Context(), p.ByName("key"))
		if err != nil {
			errors.WriteHTTPUnauthorized(w, "invalid authorization key", err)
			return
		}
		a.ingestFMP4Push(w, r, r.Body, mediaSigner, media.IngestTransportFMP4Direct)
	}
}

// ingestFMP4Push runs an authenticated fragmented-MP4 push to completion: in a
// detached worker that takes over the connection (--isolated-ingest), or in
// process, reading body.
func (a *StreamplaceAPI) ingestFMP4Push(w http.ResponseWriter, httpReq *http.Request, body io.Reader, mediaSigner media.MediaSigner, transport string) {
	reqCtx := log.WithLogValues(httpReq.Context(), "streamer", mediaSigner.Streamer())

	if err := a.checkBanned(reqCtx, mediaSigner.Streamer()); err != nil {
		errors.WriteHTTPUnauthorized(w, err.Error(), err)
		return
	}

	if !a.CLI.IsolatedIngest {
		if err := a.MediaManager.MP4Ingest(reqCtx, body, mediaSigner, transport); err != nil {
			log.Log(reqCtx, "stream error", "error", err)
			errors.WriteHTTPInternalServerError(w, "stream error", err)
			return
		}
		log.Log(reqCtx, "stream success")
		return
	}

	// Zero-downtime path: hijack the authed push connection and hand it to a
	// DETACHED worker that owns the connection (so it survives a main
	// restart) and serves signed segments back over its socket.
	hj, ok := w.(http.Hijacker)
	if !ok {
		// The isolated path needs a hijackable HTTP/1.1 connection (which the
		// real push clients — the `streamplace live` CLI, the OBS plugin, and
		// tests pushing over localhost — always are; the Mist ingest itself
		// arrives via MistPullIngest). We don't support a non-hijack fallback:
		// it couldn't receive mid-stream manifest updates and would stay stuck
		// pre-live, so refuse. Such a client can use WHIP instead.
		log.Error(reqCtx, "isolated ingest requires a hijackable HTTP/1.1 connection; refusing push")
		errors.WriteHTTPInternalServerError(w, "isolated ingest requires a hijackable HTTP/1.1 connection; use WHIP", fmt.Errorf("connection is not hijackable"))
		return
	}
	conn, bufrw, err := hj.Hijack()
	if err != nil {
		log.Error(reqCtx, "ingest hijack failed", "error", err)
		return
	}
	var prebuf []byte
	if n := bufrw.Reader.Buffered(); n > 0 {
		prebuf = make([]byte, n)
		_, _ = io.ReadFull(bufrw.Reader, prebuf)
	}
	chunked := len(httpReq.TransferEncoding) > 0 && httpReq.TransferEncoding[0] == "chunked"
	if err := a.MediaManager.MP4IngestDetached(reqCtx, conn, prebuf, chunked, mediaSigner, transport); err != nil {
		log.Log(reqCtx, "isolated stream ended", "error", err)
	}
	// The connection is hijacked; the HTTP response is the worker's now.
}
