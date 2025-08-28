package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	rtpprof "runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/pion/webrtc/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	sloghttp "github.com/samber/slog-http"
	"stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/mist/mistconfig"
	"stream.place/streamplace/pkg/mist/misttriggers"
	"stream.place/streamplace/pkg/model"
	notificationpkg "stream.place/streamplace/pkg/notifications"
	"stream.place/streamplace/pkg/rtcrec"
	v0 "stream.place/streamplace/pkg/schema/v0"
)

func (a *StreamplaceAPI) ServeInternalHTTP(ctx context.Context) error {
	handler, err := a.InternalHandler(ctx)
	if err != nil {
		return err
	}
	return a.ServerWithShutdown(ctx, handler, func(s *http.Server) error {
		s.Addr = a.CLI.HTTPInternalAddr
		log.Log(ctx, "http server starting", "addr", s.Addr)
		return s.ListenAndServe()
	})
}

// lightweight way to authenticate push requests to ourself
var mkvRE *regexp.Regexp

func init() {
	mkvRE = regexp.MustCompile(`^\d+\.mkv$`)
}

func (a *StreamplaceAPI) InternalHandler(ctx context.Context) (http.Handler, error) {
	router := httprouter.New()
	broker := misttriggers.NewTriggerBroker()

	broker.OnPushRewrite(func(ctx context.Context, payload *misttriggers.PushRewritePayload) (string, error) {
		log.Log(ctx, "got push out start", "streamName", payload.StreamName, "url", payload.URL.String())
		// Extract the last part of the URL path
		urlPath := payload.URL.Path
		parts := strings.Split(urlPath, "/")
		lastPart := ""
		if len(parts) > 0 {
			lastPart = parts[len(parts)-1]
		}
		mediaSigner, err := a.MakeMediaSigner(ctx, lastPart)
		if err != nil {
			return "", err
		}

		ms := time.Now().UnixMilli()
		out := fmt.Sprintf("%s+%s_%d", mistconfig.StreamName, mediaSigner.Streamer(), ms)
		a.SignerCacheMu.Lock()
		a.SignerCache[mediaSigner.Streamer()] = mediaSigner
		a.SignerCacheMu.Unlock()
		log.Log(ctx, "added key to cache", "mist-stream", out, "streamer", mediaSigner.Streamer())

		return out, nil
	})
	triggerCollection := misttriggers.NewMistCallbackHandlersCollection(a.CLI, broker)
	router.POST("/mist-trigger", triggerCollection.Trigger())
	router.HandlerFunc("GET", "/healthz", a.HandleHealthz(ctx))

	// Add pprof handlers
	router.HandlerFunc("GET", "/debug/pprof/", pprof.Index)
	router.HandlerFunc("GET", "/debug/pprof/cmdline", pprof.Cmdline)
	router.HandlerFunc("GET", "/debug/pprof/profile", pprof.Profile)
	router.HandlerFunc("GET", "/debug/pprof/symbol", pprof.Symbol)
	router.HandlerFunc("GET", "/debug/pprof/trace", pprof.Trace)
	router.Handler("GET", "/debug/pprof/goroutine", pprof.Handler("goroutine"))
	router.Handler("GET", "/debug/pprof/heap", pprof.Handler("heap"))
	router.Handler("GET", "/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	router.Handler("GET", "/debug/pprof/block", pprof.Handler("block"))
	router.Handler("GET", "/debug/pprof/allocs", pprof.Handler("allocs"))
	router.Handler("GET", "/debug/pprof/mutex", pprof.Handler("mutex"))

	router.POST("/gc", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		runtime.GC()
		w.WriteHeader(204)
	})

	router.Handler("GET", "/metrics", promhttp.Handler())

	router.GET("/playback/:user/:rendition/concat", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		user := p.ByName("user")
		if user == "" {
			errors.WriteHTTPBadRequest(w, "user required", nil)
			return
		}
		rendition := p.ByName("rendition")
		if rendition == "" {
			errors.WriteHTTPBadRequest(w, "rendition required", nil)
			return
		}
		user, err := a.NormalizeUser(ctx, user)
		if err != nil {
			errors.WriteHTTPBadRequest(w, "invalid user", err)
			return
		}
		w.Header().Set("content-type", "text/plain")
		fmt.Fprintf(w, "ffconcat version 1.0\n")
		// intermittent reports that you need two here to make things work properly? shouldn't matter.
		for i := 0; i < 2; i += 1 {
			fmt.Fprintf(w, "file '%s/playback/%s/%s/latest.mp4'\n", a.CLI.OwnInternalURL(), user, rendition)
		}
	})

	router.GET("/playback/:user/:rendition/latest.mp4", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		user := p.ByName("user")
		if user == "" {
			errors.WriteHTTPBadRequest(w, "user required", nil)
			return
		}
		user, err := a.NormalizeUser(ctx, user)
		if err != nil {
			errors.WriteHTTPBadRequest(w, "invalid user", err)
			return
		}
		rendition := p.ByName("rendition")
		if rendition == "" {
			errors.WriteHTTPBadRequest(w, "rendition required", nil)
			return
		}
		segChan := a.Bus.SubscribeSegment(ctx, user, rendition)
		defer a.Bus.UnsubscribeSegment(ctx, user, rendition, segChan)
		seg := <-segChan.C
		base := filepath.Base(seg.Filepath)
		w.Header().Set("Location", fmt.Sprintf("%s/playback/%s/%s/segment/%s\n", a.CLI.OwnInternalURL(), user, rendition, base))
		w.WriteHeader(301)
	})

	router.GET("/playback/:user/:rendition/segment/:file", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		user := p.ByName("user")
		if user == "" {
			errors.WriteHTTPBadRequest(w, "user required", nil)
			return
		}
		user, err := a.NormalizeUser(ctx, user)
		if err != nil {
			errors.WriteHTTPBadRequest(w, "invalid user", err)
			return
		}
		file := p.ByName("file")
		if file == "" {
			errors.WriteHTTPBadRequest(w, "file required", nil)
			return
		}
		fullpath, err := a.CLI.SegmentFilePath(user, file)
		if err != nil {
			errors.WriteHTTPBadRequest(w, "badly formatted request", err)
			return
		}
		http.ServeFile(w, r, fullpath)
	})

	router.HEAD("/playback/:user/:rendition/stream.mkv", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		user := p.ByName("user")
		if user == "" {
			errors.WriteHTTPBadRequest(w, "user required", nil)
			return
		}
		w.Header().Set("Content-Type", "video/x-matroska")
		w.Header().Set("Transfer-Encoding", "chunked")
		w.WriteHeader(200)
	})

	router.POST("/http-pipe/:uuid", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		uu := p.ByName("uuid")
		if uu == "" {
			errors.WriteHTTPBadRequest(w, "uuid required", nil)
			return
		}
		pr := a.MediaManager.GetHTTPPipeWriter(uu)
		if pr == nil {
			errors.WriteHTTPNotFound(w, "http-pipe not found", nil)
			return
		}
		if _, err := io.Copy(pr, r.Body); err != nil {
			errors.WriteHTTPInternalServerError(w, "failed to copy response", nil)
		}
	})

	// self-destruct code, useful for dumping goroutines on windows
	router.POST("/abort", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		if err := rtpprof.Lookup("goroutine").WriteTo(os.Stderr, 2); err != nil {
			log.Log(ctx, "error writing rtpprof", "error", err)
		}
		log.Log(ctx, "got POST /abort, self-destructing")
		os.Exit(1)
	})

	handleIncomingStream := func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		key := p.ByName("key")
		log.Log(ctx, "stream start")

		var mediaSigner media.MediaSigner
		var ok bool
		var err error
		parts := strings.Split(key, "_")

		if len(parts) == 2 {
			a.SignerCacheMu.Lock()
			mediaSigner, ok = a.SignerCache[parts[0]]
			a.SignerCacheMu.Unlock()
			if !ok {
				log.Error(ctx, "couldn't find key in cache", "part", parts[0], "key", key)
				errors.WriteHTTPUnauthorized(w, "invalid authorization key", nil)
				return
			}
		} else {
			mediaSigner, err = a.MakeMediaSigner(ctx, key)
			if err != nil {
				errors.WriteHTTPUnauthorized(w, "invalid authorization key", err)
				return
			}
		}

		err = a.MediaManager.MKVIngest(ctx, r.Body, mediaSigner)

		if err != nil {
			log.Log(ctx, "stream error", "error", err)
			errors.WriteHTTPInternalServerError(w, "stream error", err)
			return
		}
		log.Log(ctx, "stream success", "url", r.URL.String())
	}

	// route to accept an incoming mkv stream from OBS, segment it, and push the segments back to this HTTP handler
	router.POST("/live/:key", handleIncomingStream)
	router.PUT("/live/:key", handleIncomingStream)

	router.GET("/player-report/:id", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		id := p.ByName("id")
		if id == "" {
			errors.WriteHTTPBadRequest(w, "id required", nil)
			return
		}
		events, err := a.Model.PlayerReport(id)
		if err != nil {
			errors.WriteHTTPBadRequest(w, err.Error(), err)
			return
		}
		bs, err := json.Marshal(events)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to marhsal json", err)
			return
		}
		if _, err := w.Write(bs); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	})

	router.GET("/segment/:id", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		id := p.ByName("id")
		if id == "" {
			errors.WriteHTTPBadRequest(w, "id required", nil)
			return
		}
		segment, err := a.Model.GetSegment(id)
		if err != nil {
			errors.WriteHTTPBadRequest(w, err.Error(), err)
			return
		}
		if segment == nil {
			errors.WriteHTTPNotFound(w, "segment not found", nil)
			return
		}
		spSeg, err := segment.ToStreamplaceSegment()
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to convert segment to streamplace segment", err)
			return
		}
		bs, err := json.Marshal(spSeg)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to marhsal json", err)
			return
		}
		if _, err := w.Write(bs); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	})

	router.DELETE("/player-events", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		err := a.Model.ClearPlayerEvents()
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to delete player events", err)
			return
		}
		w.WriteHeader(204)
	})

	router.GET("/settings", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		id := a.Signer.Hex()

		ident, err := a.Model.GetIdentity(id)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to get settings", err)
			return
		}

		bs, err := json.Marshal(ident)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to marshal json", err)
			return
		}
		if _, err := w.Write(bs); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	})

	router.GET("/followers/:user", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		user := p.ByName("user")
		if user == "" {
			errors.WriteHTTPBadRequest(w, "user required", nil)
			return
		}

		followers, err := a.Model.GetUserFollowers(ctx, user)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to get followers", err)
			return
		}
		bs, err := json.Marshal(followers)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to marshal json", err)
			return
		}
		if _, err := w.Write(bs); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	})

	router.GET("/following/:user", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		user := p.ByName("user")
		if user == "" {
			errors.WriteHTTPBadRequest(w, "user required", nil)
			return
		}

		followers, err := a.Model.GetUserFollowing(ctx, user)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to get followers", err)
			return
		}
		bs, err := json.Marshal(followers)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to marshal json", err)
			return
		}
		if _, err := w.Write(bs); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	})

	router.GET("/notifications", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		notifications, err := a.Model.ListNotifications()
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to get notifications", err)
			return
		}
		bs, err := json.Marshal(notifications)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to marshal json", err)
			return
		}
		if _, err := w.Write(bs); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	})

	router.GET("/chat-posts", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		posts, err := a.Model.ListFeedPosts()
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to get chat posts", err)
			return
		}
		bs, err := json.Marshal(posts)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to marshal json", err)
			return
		}
		if _, err := w.Write(bs); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	})

	router.GET("/chat/:cid", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		cid := p.ByName("cid")
		if cid == "" {
			errors.WriteHTTPBadRequest(w, "cid required", nil)
			return
		}
		msg, err := a.Model.GetChatMessage(cid)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to get chat posts", err)
			return
		}
		spmsg, err := msg.ToStreamplaceMessageView()
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to convert chat message to streamplace message view", err)
			return
		}
		bs, err := json.Marshal(spmsg)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to marshal json", err)
			return
		}
		if _, err := w.Write(bs); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	})

	router.GET("/oauth-sessions", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		sessions, err := a.Model.ListOAuthSessions()
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to get oauth sessions", err)
			return
		}
		bs, err := json.Marshal(sessions)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to marshal oauth sessions", err)
			return
		}
		if _, err := w.Write(bs); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	})

	router.POST("/notification-blast", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		var payload notificationpkg.NotificationBlast
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			errors.WriteHTTPBadRequest(w, "invalid request body", err)
			return
		}
		notifications, err := a.Model.ListNotifications()
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to get notifications", err)
			return
		}
		if a.FirebaseNotifier == nil {
			errors.WriteHTTPInternalServerError(w, "firebase notifier not initialized", nil)
			return
		}
		tokens := []string{}
		for _, not := range notifications {
			tokens = append(tokens, not.Token)
		}
		err = a.FirebaseNotifier.Blast(ctx, tokens, &payload)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to blast notifications", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	router.PUT("/settings/:id", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "PUT")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		id := p.ByName("id")
		if id == "" {
			errors.WriteHTTPBadRequest(w, "id required", nil)
			return
		}

		var ident model.Identity
		if err := json.NewDecoder(r.Body).Decode(&ident); err != nil {
			errors.WriteHTTPBadRequest(w, "invalid request body", err)
			return
		}
		ident.ID = id

		if err := a.Model.UpdateIdentity(&ident); err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to update settings", err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})

	router.POST("/replay/:streamKey", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		key := p.ByName("streamKey")
		if key == "" {
			errors.WriteHTTPBadRequest(w, "streamKey required", nil)
			return
		}
		mediaSigner, err := a.MakeMediaSigner(ctx, key)
		if err != nil {
			errors.WriteHTTPUnauthorized(w, "invalid authorization key", err)
			return
		}
		pc, err := rtcrec.NewReplayPeerConnection(ctx, r.Body)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to create replay peer connection", err)
			return
		}
		answer, err := a.MediaManager.WebRTCIngest(ctx, &webrtc.SessionDescription{SDP: "placeholder"}, mediaSigner, pc, make(chan struct{}))
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to ingest web rtc", err)
			return
		}
		w.WriteHeader(200)
		if _, err := w.Write([]byte(answer.SDP)); err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to write response", err)
			log.Error(ctx, "error writing response", "error", err)
		}
	})

	router.GET("/clip/:did/clip.mp4", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		did := p.ByName("did")
		if did == "" {
			errors.WriteHTTPBadRequest(w, "did required", nil)
			return
		}
		user, err := a.NormalizeUser(ctx, did)
		if err != nil {
			errors.WriteHTTPBadRequest(w, "invalid user", err)
			return
		}
		secsStr := r.URL.Query().Get("secs")
		secs := 60 // Default to 60 seconds
		if secsStr != "" {
			parsedSecs, err := strconv.Atoi(secsStr)
			if err != nil {
				errors.WriteHTTPBadRequest(w, "invalid secs parameter", err)
				return
			}
			secs = parsedSecs
		}
		after := time.Now().Add(-time.Duration(secs) * time.Second)
		w.Header().Set("Content-Type", "video/mp4")
		err = media.ClipUser(ctx, a.Model, a.CLI, user, w, nil, &after)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to clip user", err)
			return
		}
	})

	handler := sloghttp.Recovery(router)
	if log.Level(4) {
		handler = sloghttp.New(slog.Default())(handler)
	}
	return handler, nil
}

func (a *StreamplaceAPI) keyToUser(ctx context.Context, key string) (string, error) {
	payload, err := base64.URLEncoding.DecodeString(key)
	if err != nil {
		return "", err
	}
	signed, err := a.Signer.Verify(payload)
	if err != nil {
		return "", err
	}
	_, ok := signed.Data().(*v0.StreamKey)
	if !ok {
		return "", fmt.Errorf("got signed data but it wasn't a stream key")
	}
	return strings.ToLower(signed.Signer()), nil
}
