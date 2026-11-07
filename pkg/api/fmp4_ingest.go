package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	glex "github.com/streamplace/glex/runtime"
	"stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/placestream"
)

// The Streamplace OBS plugin asks for its stream configuration with the same
// request and response shapes as OBS's Enhanced RTMP multitrack video
// configuration (GoLiveApi, see obs-studio's
// frontend/utility/models/multitrack-video.hpp), so one endpoint can configure
// both. Only the fields the node reads or sets are modeled here.
type clientConfigurationRequest struct {
	Authentication string `json:"authentication"`
	Preferences    struct {
		// AudioTracks names the OBS mixer tracks: track_id 0..5, name empty
		// when the streamer left the track unnamed.
		AudioTracks []audioTrackPreference `json:"audio_tracks"`
		// Canvases are OBS's video outputs; the plugin sends one.
		Canvases []struct {
			Framerate placestream.IngestDefs_Framerate `json:"framerate"`
		} `json:"canvases"`
	} `json:"preferences"`
}

type audioTrackPreference struct {
	TrackID int    `json:"track_id"`
	Name    string `json:"name"`
}

type clientConfiguration struct {
	Meta            clientConfigurationMeta   `json:"meta"`
	Status          clientConfigurationStatus `json:"status"`
	IngestEndpoints []ingestEndpoint          `json:"ingest_endpoints,omitempty"`
	// EncoderConfigurations lists the video encoders the plugin creates, one
	// track each; absent, the plugin keeps the encoder from OBS's own output
	// settings.
	EncoderConfigurations []videoConfiguration `json:"encoder_configurations,omitempty"`
	// AudioConfigurations lists the audio encoders the plugin creates, keyed
	// by the output they feed ("live"); absent, the plugin keeps the encoder
	// from OBS's own output settings.
	AudioConfigurations map[string][]audioConfiguration `json:"audio_configurations,omitempty"`
}

// videoConfiguration is a GoLiveApi encoder configuration, except that it
// names a codec rather than an OBS encoder type, a Streamplace extension:
// which encoders exist is the streamer's machine's business, so the plugin
// picks one (the streamer's own streaming encoder when it is that codec).
type videoConfiguration struct {
	Codec     string                           `json:"codec"`
	Width     int64                            `json:"width"`
	Height    int64                            `json:"height"`
	Framerate placestream.IngestDefs_Framerate `json:"framerate"`
	// Settings are the encoder settings the plugin applies over the
	// encoder's own; OBS's H.264 encoders share these names.
	Settings struct {
		Bitrate     int64  `json:"bitrate"`
		RateControl string `json:"rate_control"`
		KeyintSec   int    `json:"keyint_sec"`
		BFrames     int    `json:"bf"`
	} `json:"settings"`
}

type audioConfiguration struct {
	Codec    string `json:"codec"`
	TrackID  int    `json:"track_id"`
	Channels int    `json:"channels"`
	Settings struct {
		Bitrate int64 `json:"bitrate"`
	} `json:"settings"`
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

// maxStems is how many named mixer tracks become stems beside the full mix:
// OBS allows six audio encoders per output, and each track takes two.
const maxStems = 2

// defaultAudioRenditions are the codecs each audio track is encoded in when
// the streamer hasn't chosen (place.stream.ingest.audioRendition).
var defaultAudioRenditions = []*placestream.IngestAudioRendition{
	{Codec: "aac", Bitrate: 160},
	{Codec: "opus", Bitrate: 128},
}

// audioConfigurations lists the audio encoders the plugin should create: one
// per codec the streamer chose (or the defaults) for track 0 (the full mix)
// and for each mixer track the streamer named beyond the first, in track
// order, up to maxStems of them; two codecs over three tracks is OBS's limit
// of six. With no renditions and no named tracks it returns nil, and the
// plugin keeps the encoder from OBS's output settings.
func audioConfigurations(tracks []audioTrackPreference, renditions []*placestream.IngestAudioRendition) []audioConfiguration {
	var stems []int
	for _, t := range tracks {
		if t.TrackID >= 1 && t.TrackID <= 5 && t.Name != "" {
			stems = append(stems, t.TrackID)
		}
	}
	if len(stems) == 0 && len(renditions) == 0 {
		return nil
	}
	if len(renditions) == 0 {
		renditions = defaultAudioRenditions
	}
	var codecs []*placestream.IngestAudioRendition
	for _, r := range renditions {
		if !slices.ContainsFunc(codecs, func(c *placestream.IngestAudioRendition) bool { return c.Codec == r.Codec }) {
			codecs = append(codecs, r)
		}
	}
	slices.Sort(stems)
	ids := append([]int{0}, stems[:min(len(stems), maxStems)]...)
	var out []audioConfiguration
	for _, id := range ids {
		for _, r := range codecs {
			cfg := audioConfiguration{Codec: r.Codec, TrackID: id, Channels: 2}
			cfg.Settings.Bitrate = r.Bitrate
			out = append(out, cfg)
		}
	}
	return out
}

// videoConfigurations lists the video encoders the plugin should create from
// the streamer's renditions: none, and the plugin keeps OBS's own. A stream
// has one video track for now, so only the first rendition is encoded. OBS
// can only drop whole frames from its canvas, so a rate the canvas (when
// known) can't be divided into is refused with a problem for the streamer.
func videoConfigurations(renditions []*placestream.IngestVideoRendition, canvas *placestream.IngestDefs_Framerate) (out []videoConfiguration, problem string) {
	for _, r := range renditions[:min(len(renditions), 1)] {
		if canvas != nil && !dividesFramerate(*canvas, r.Framerate) {
			return nil, fmt.Sprintf("OBS runs at %s fps, which can't be encoded at the %s fps set in Streamplace Desktop. Pick a frame rate OBS's divides into, or change OBS's.", fps(*canvas), fps(r.Framerate))
		}
		cfg := videoConfiguration{Codec: r.Codec, Width: r.Width, Height: r.Height, Framerate: r.Framerate}
		cfg.Settings.Bitrate = r.Bitrate
		cfg.Settings.RateControl = "CBR"
		// Segments are a GoP each, and MUXL takes no B-frames.
		cfg.Settings.KeyintSec = 1
		cfg.Settings.BFrames = 0
		out = append(out, cfg)
	}
	return out, ""
}

// dividesFramerate reports whether rate is canvas divided by a whole number,
// the only rates OBS can encode from it.
func dividesFramerate(canvas, rate placestream.IngestDefs_Framerate) bool {
	if canvas.Numerator < 1 || canvas.Denominator < 1 {
		return true // OBS didn't say
	}
	n, d := canvas.Numerator*rate.Denominator, canvas.Denominator*rate.Numerator
	return d > 0 && n >= d && n%d == 0
}

func fps(f placestream.IngestDefs_Framerate) string {
	if f.Denominator == 1 {
		return fmt.Sprint(f.Numerator)
	}
	return fmt.Sprintf("%.2f", float64(f.Numerator)/float64(f.Denominator))
}

// localRecords returns repoDID's records of collection kept on this node,
// as T.
func localRecords[T any, PT interface {
	*T
	glex.Record
}](a *StreamplaceAPI, repoDID, collection string) ([]*T, error) {
	rows, err := a.LocalDB.ListLocalRecords(repoDID, collection)
	if err != nil {
		return nil, err
	}
	var out []*T
	for _, row := range rows {
		value := &glex.LexiconTypeDecoder{}
		if err := value.UnmarshalCBOR(row.Record); err != nil {
			return nil, fmt.Errorf("decoding %s: %w", row.URI(), err)
		}
		rec, err := glex.RecordAs[T, PT](value.Val)
		if err != nil {
			return nil, fmt.Errorf("decoding %s: %w", row.URI(), err)
		}
		out = append(out, rec)
	}
	return out, nil
}

// HandleClientConfiguration answers the plugin's configuration request. A bad
// stream key is reported in status, as GoLiveApi clients expect, so OBS shows
// the message. The encoders come from the streamer's renditions kept on this
// node (place.stream.ingest.*Rendition) and their named mixer tracks (see
// audioConfigurations); with none of either the response names no
// encoders, so the plugin keeps the encoders from OBS's output settings: one
// video and one audio track.
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
		streamer, problem := a.streamKeyStreamer(r.Context(), req.Authentication)
		if problem != "" {
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
			video, err := localRecords[placestream.IngestVideoRendition](a, streamer, "place.stream.ingest.videoRendition")
			if err != nil {
				errors.WriteHTTPInternalServerError(w, "could not read video renditions", err)
				return
			}
			audio, err := localRecords[placestream.IngestAudioRendition](a, streamer, "place.stream.ingest.audioRendition")
			if err != nil {
				errors.WriteHTTPInternalServerError(w, "could not read audio renditions", err)
				return
			}
			var canvas *placestream.IngestDefs_Framerate
			if len(req.Preferences.Canvases) > 0 {
				canvas = &req.Preferences.Canvases[0].Framerate
			}
			config.EncoderConfigurations, problem = videoConfigurations(video, canvas)
			if problem != "" {
				config = clientConfiguration{Meta: config.Meta, Status: clientConfigurationStatus{Result: "error", HTMLEnUS: problem}}
			} else if live := audioConfigurations(req.Preferences.AudioTracks, audio); live != nil {
				config.AudioConfigurations = map[string][]audioConfiguration{"live": live}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(config); err != nil {
			log.Error(ctx, "error writing client configuration", "error", err)
		}
	}
}

// streamKeyStreamer returns whose key it is, or explains to the streamer why
// key may not stream.
func (a *StreamplaceAPI) streamKeyStreamer(ctx context.Context, key string) (streamer, problem string) {
	ms, err := a.MakeMediaSigner(ctx, key)
	if err != nil {
		return "", "This stream key isn't valid. Copy a fresh one from your Streamplace dashboard."
	}
	if err := a.checkBanned(ctx, ms.Streamer()); err != nil {
		return "", "This account can't stream right now."
	}
	return ms.Streamer(), ""
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
