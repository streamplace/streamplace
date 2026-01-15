package analytics

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"stream.place/streamplace/pkg/log"
)

func HandleRealtimeStats(client Client) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if client == nil {
			log.Debug(req.Context(), "analytics not configured, skipping realtime stats")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(503)
			err := json.NewEncoder(w).Encode(map[string]string{"error": "analytics not configured"})
			if err != nil {
				log.Error(req.Context(), "Could not send error message to client from HandleRealtimeStats")
			}
			return
		}

		windowMinutes := req.URL.Query().Get("window")
		if windowMinutes == "" {
			windowMinutes = "5"
		}
		window, _ := strconv.ParseUint(windowMinutes, 10, 32)

		r := &RealtimeStatsRequest{
			WindowMinutes: uint32(window),
		}

		if streamerDid := req.URL.Query().Get("streamer"); streamerDid != "" {
			r.StreamerDid = &streamerDid
		}

		resp, err := client.GetRealtimeStats(req.Context(), r)
		if err != nil {
			log.Log(req.Context(), "failed to fetch realtime stats", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			if encErr := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}); encErr != nil {
				log.Error(req.Context(), "failed to encode error response", "error", encErr)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Error(req.Context(), "failed to encode realtime stats response", "error", err)
		}
	}
}

func HandleStreamerStats(client Client) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		if client == nil {
			log.Debug(req.Context(), "analytics not configured, skipping streamer stats")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(503)
			err := json.NewEncoder(w).Encode(map[string]string{"error": "analytics not configured"})
			if err != nil {
				log.Error(req.Context(), "Could not send error message to client from HandleStreamerStats")
			}
			return
		}

		streamerDid := params.ByName("did")

		r := &StreamerStatsRequest{
			StreamerDid: streamerDid,
		}

		if start := req.URL.Query().Get("start"); start != "" {
			startMs, _ := strconv.ParseInt(start, 10, 64)
			r.StartTimeMs = &startMs
		}

		if end := req.URL.Query().Get("end"); end != "" {
			endMs, _ := strconv.ParseInt(end, 10, 64)
			r.EndTimeMs = &endMs
		}

		resp, err := client.GetStreamerStats(req.Context(), r)
		if err != nil {
			log.Log(req.Context(), "failed to fetch streamer stats", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			if encErr := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}); encErr != nil {
				log.Error(req.Context(), "failed to encode error response", "error", encErr)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Error(req.Context(), "failed to encode streamer stats response", "error", err)
		}
	}
}

func HandleViewerHistory(client Client) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		if client == nil {
			log.Debug(req.Context(), "analytics not configured, skipping viewer history")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(503)
			if err := json.NewEncoder(w).Encode(map[string]string{"error": "analytics not configured"}); err != nil {
				log.Error(req.Context(), "failed to encode error response", "error", err)
			}
			return
		}

		did := params.ByName("did")

		limit := req.URL.Query().Get("limit")
		if limit == "" {
			limit = "50"
		}
		limitVal, _ := strconv.ParseUint(limit, 10, 32)

		r := &ViewerHistoryRequest{
			Did:   did,
			Limit: uint32(limitVal),
		}

		if start := req.URL.Query().Get("start"); start != "" {
			startMs, _ := strconv.ParseInt(start, 10, 64)
			r.StartTimeMs = &startMs
		}

		if end := req.URL.Query().Get("end"); end != "" {
			endMs, _ := strconv.ParseInt(end, 10, 64)
			r.EndTimeMs = &endMs
		}

		resp, err := client.GetViewerHistory(req.Context(), r)
		if err != nil {
			log.Log(req.Context(), "failed to fetch viewer history", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			if encErr := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}); encErr != nil {
				log.Error(req.Context(), "failed to encode error response", "error", encErr)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Error(req.Context(), "failed to encode viewer history response", "error", err)
		}
	}
}
