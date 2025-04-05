package api

import (
	"bufio"
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
	"github.com/prometheus/client_golang/prometheus/promhttp"
	sloghttp "github.com/samber/slog-http"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/mist/mistconfig"
	"stream.place/streamplace/pkg/mist/misttriggers"
	"stream.place/streamplace/pkg/model"
	notificationpkg "stream.place/streamplace/pkg/notifications"
	"stream.place/streamplace/pkg/renditions"
	v0 "stream.place/streamplace/pkg/schema/v0"
)

func (a *StreamplaceAPI) ServeInternalHTTP(ctx context.Context) error {
	handler, err := a.InternalHandler(ctx)
	if err != nil {
		return err
	}
	return a.ServerWithShutdown(ctx, handler, func(s *http.Server) error {
		s.Addr = a.CLI.HttpInternalAddr
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
	broker.OnPushOutStart(func(ctx context.Context, payload *misttriggers.PushOutStartPayload) (string, error) {
		return payload.URL, nil
	})
	broker.OnPushRewrite(func(ctx context.Context, payload *misttriggers.PushRewritePayload) (string, error) {
		log.Log(ctx, "got push out start", "streamName", payload.StreamName, "url", payload.URL.String())

		ms := time.Now().UnixMilli()
		out := fmt.Sprintf("%s+%s_%d", mistconfig.STREAM_NAME, payload.StreamName, ms)

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
		seg := <-a.MediaManager.SubscribeSegment(ctx, user, rendition)
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

	router.GET("/playback/:user/:rendition/stream.mkv", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
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
		w.Header().Set("Content-Type", "video/x-matroska")
		w.WriteHeader(200)
		err = a.MediaManager.SegmentToMKVPlusOpus(ctx, user, rendition, w)
		if err != nil {
			log.Log(ctx, "stream.mkv error", "error", err)
		}
	})

	router.GET("/playback/:user/:rendition/stream.mp4", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
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
		var delayMS int64 = 1000
		userDelay := r.URL.Query().Get("delayms")
		if userDelay != "" {
			var err error
			delayMS, err = strconv.ParseInt(userDelay, 10, 64)
			if err != nil {
				errors.WriteHTTPBadRequest(w, "error parsing delay", err)
				return
			}
			if delayMS > 10000 {
				errors.WriteHTTPBadRequest(w, "delay too large, maximum 10000", nil)
				return
			}
		}
		w.Header().Set("Content-Type", "video/mp4")
		w.WriteHeader(200)
		g, ctx := errgroup.WithContext(ctx)
		pr, pw := io.Pipe()
		bufw := bufio.NewWriter(pw)
		g.Go(func() error {
			return a.MediaManager.SegmentToMP4(ctx, user, rendition, bufw)
		})
		g.Go(func() error {
			time.Sleep(time.Duration(delayMS) * time.Millisecond)
			_, err := io.Copy(w, pr)
			return err
		})
		g.Wait()
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
		io.Copy(pr, r.Body)
	})

	// self-destruct code, useful for dumping goroutines on windows
	router.POST("/abort", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		rtpprof.Lookup("goroutine").WriteTo(os.Stderr, 2)
		log.Log(ctx, "got POST /abort, self-destructing")
		os.Exit(1)
	})

	handleIncomingStream := func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		log.Log(ctx, "stream start")
		err := a.MediaManager.IngestStream(ctx, r.Body, a.MediaSigner)

		if err != nil {
			log.Log(ctx, "stream error", "error", err)
			errors.WriteHTTPInternalServerError(w, "stream error", err)
			return
		}
		log.Log(ctx, "stream success", "url", r.URL.String())
	}

	// route to accept an incoming mkv stream from OBS, segment it, and push the segments back to this HTTP handler
	router.POST("/stream/:key", handleIncomingStream)
	router.PUT("/stream/:key", handleIncomingStream)

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
		w.Write(bs)
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
		w.Write(bs)
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
		w.Write(bs)
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
		w.Write(bs)
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
		w.Write(bs)
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
		w.Write(bs)
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
		w.Write(bs)
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

	router.POST("/livepeer-auth-webhook-url", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		var payload struct {
			URL string `json:"url"`
		}
		// urls look like http://127.0.0.1:9999/live/did:plc:dkh4rwafdcda4ko7lewe43ml-uucbv40mdkcfat50/47.mp4
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			errors.WriteHTTPBadRequest(w, "invalid request body (could not decode)", err)
			return
		}
		parts := strings.Split(payload.URL, "/")
		if len(parts) < 5 {
			errors.WriteHTTPBadRequest(w, "invalid request body (too few parts)", nil)
			return
		}
		didSession := parts[4]
		idParts := strings.Split(didSession, "-")
		if len(idParts) != 2 {
			errors.WriteHTTPBadRequest(w, "invalid request body (invalid did session)", nil)
			return
		}
		did := idParts[0]
		// sessionID := idParts[1]
		seg, err := a.Model.LatestSegmentForUser(did)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to get latest segment", err)
			return
		}
		spseg, err := seg.ToStreamplaceSegment()
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to convert segment to streamplace segment", err)
			return
		}
		renditions, err := renditions.GenerateRenditions(spseg)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to generate renditions", err)
			return
		}
		out := map[string]any{
			"manifestID": didSession,
			"profiles":   renditions.ToLivepeerProfiles(),
		}
		bs, err := json.Marshal(out)
		if err != nil {
			errors.WriteHTTPInternalServerError(w, "unable to marshal json", err)
			return
		}
		w.Write(bs)
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
