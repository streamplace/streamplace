package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
	sloghttp "github.com/samber/slog-http"
	"golang.org/x/time/rate"

	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/js/app"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/signers/eip712"
	"stream.place/streamplace/pkg/director"
	apierrors "stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/linking"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/mist/mistconfig"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/notifications"
	"stream.place/streamplace/pkg/spmetrics"
	"stream.place/streamplace/pkg/spxrpc"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/streamplace"

	metrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	echomiddleware "github.com/slok/go-http-metrics/middleware/echo"
	httproutermiddleware "github.com/slok/go-http-metrics/middleware/httprouter"
	middlewarestd "github.com/slok/go-http-metrics/middleware/std"
)

type StreamplaceAPI struct {
	CLI              *config.CLI
	Model            model.Model
	StatefulDB       *statedb.StatefulDB
	Updater          *Updater
	Signer           *eip712.EIP712Signer
	Mimes            map[string]string
	FirebaseNotifier notifications.FirebaseNotifier
	MediaManager     *media.MediaManager
	MediaSigner      media.MediaSigner
	// not thread-safe yet
	Aliases  map[string]string
	Bus      *bus.Bus
	ATSync   *atproto.ATProtoSynchronizer
	Director *director.Director

	connTracker *WebsocketTracker

	limiters      map[string]*rate.Limiter
	limitersMu    sync.Mutex
	SignerCache   map[string]media.MediaSigner
	SignerCacheMu sync.Mutex
	op            *oatproxy.OATProxy
}

type WebsocketTracker struct {
	connections   map[string]int
	maxConnsPerIP int
	mu            sync.RWMutex
}

func MakeStreamplaceAPI(cli *config.CLI, mod model.Model, statefulDB *statedb.StatefulDB, signer *eip712.EIP712Signer, noter notifications.FirebaseNotifier, mm *media.MediaManager, ms media.MediaSigner, bus *bus.Bus, atsync *atproto.ATProtoSynchronizer, d *director.Director, op *oatproxy.OATProxy) (*StreamplaceAPI, error) {
	updater, err := PrepareUpdater(cli)
	if err != nil {
		return nil, err
	}
	a := &StreamplaceAPI{CLI: cli,
		Model:            mod,
		StatefulDB:       statefulDB,
		Updater:          updater,
		Signer:           signer,
		FirebaseNotifier: noter,
		MediaManager:     mm,
		MediaSigner:      ms,
		Aliases:          map[string]string{},
		Bus:              bus,
		ATSync:           atsync,
		Director:         d,
		connTracker:      NewWebsocketTracker(cli.RateLimitWebsocket),
		limiters:         make(map[string]*rate.Limiter),
		SignerCache:      make(map[string]media.MediaSigner),
		op:               op,
	}
	a.Mimes, err = updater.GetMimes()
	if err != nil {
		return nil, err
	}
	return a, nil
}

type AppHostingFS struct {
	http.FileSystem
}

var ErrorIndex = errors.New("not found, use index.html")

func (fs AppHostingFS) Open(name string) (http.File, error) {
	file, err1 := fs.FileSystem.Open(name)
	if err1 == nil {
		return file, nil
	}
	return nil, ErrorIndex
}

// api/playback/iame.li/webrtc?rendition=source
// api/playback/iame.li/stream.mp4?rendition=source
// api/playback/iame.li/stream.webm?rendition=source
// api/playback/iame.li/hls/index.m3u8
// api/playback/iame.li/hls/source/stream.m3u8
// api/playback/iame.li/hls/source/000000000000.ts

func (a *StreamplaceAPI) Handler(ctx context.Context) (http.Handler, error) {

	mdlw := middleware.New(middleware.Config{
		Recorder: metrics.NewRecorder(metrics.Config{}),
	})
	var xrpc http.Handler
	xrpc, err := spxrpc.NewServer(ctx, a.CLI, a.Model, a.StatefulDB, a.op, mdlw, a.ATSync)
	if err != nil {
		return nil, err
	}
	router := httprouter.New()

	// Create our middleware factory with the default settings.

	a.op.Echo.Use(echomiddleware.Handler("", mdlw))

	// r.GET("/test/:id", httproutermiddleware.Handler("/test/:id", h1, mdlw))

	addHandle := func(router *httprouter.Router, method, path string, handler httprouter.Handle) {
		router.Handle(method, path, httproutermiddleware.Handler(path, handler, mdlw))
	}
	addFunc := func(router *httprouter.Router, method, path string, handler http.HandlerFunc) {
		router.Handler(method, path, middlewarestd.Handler(path, mdlw, handler))
	}

	router.Handler("GET", "/oauth/*anything", a.op.Handler())
	router.Handler("POST", "/oauth/*anything", a.op.Handler())
	router.Handler("GET", "/.well-known/oauth-authorization-server", a.op.Handler())
	router.Handler("GET", "/.well-known/oauth-protected-resource", a.op.Handler())
	router.Handler("GET", "/.well-known/apple-app-site-association", a.HandleAppleAppSiteAssociation(ctx))
	router.Handler("GET", "/.well-known/assetlinks.json", a.HandleAndroidAssetLinks(ctx))
	apiRouter := httprouter.New()
	addFunc(apiRouter, "POST", "/api/notification", a.HandleNotification(ctx))
	// old clients
	addFunc(router, "GET", "/app-updates", a.HandleAppUpdates(ctx))
	// new ones
	addFunc(apiRouter, "GET", "/api/manifest", a.HandleAppUpdates(ctx))
	addHandle(apiRouter, "GET", "/api/desktop-updates/:platform/:architecture/:version/:buildTime/:file", a.HandleDesktopUpdates(ctx))
	addHandle(apiRouter, "POST", "/api/webrtc/:stream", a.MistProxyHandler(ctx, "/webrtc/%s"))
	addHandle(apiRouter, "OPTIONS", "/api/webrtc/:stream", a.MistProxyHandler(ctx, "/webrtc/%s"))
	addHandle(apiRouter, "DELETE", "/api/webrtc/:stream", a.MistProxyHandler(ctx, "/webrtc/%s"))
	addFunc(apiRouter, "POST", "/api/segment", a.HandleSegment(ctx))
	addFunc(apiRouter, "GET", "/api/healthz", a.HandleHealthz(ctx))
	addHandle(apiRouter, "GET", "/api/playback/:user/hls/*file", a.HandleHLSPlayback(ctx))
	// they're jpegs now
	addHandle(apiRouter, "GET", "/api/playback/:user/stream.jpg", a.HandleThumbnailPlayback(ctx))
	// this one is actually a jpeg (used previously and shouldn't remove for historical reasons)
	addHandle(apiRouter, "GET", "/api/playback/:user/stream.png", a.HandleThumbnailPlayback(ctx))
	addHandle(apiRouter, "GET", "/api/app-return/*anything", a.HandleAppReturn(ctx))
	addHandle(apiRouter, "POST", "/api/playback/:user/webrtc", a.HandleWebRTCPlayback(ctx))
	addHandle(apiRouter, "POST", "/api/ingest/webrtc", a.HandleWebRTCIngest(ctx))
	addHandle(apiRouter, "POST", "/api/ingest/webrtc/:key", a.HandleWebRTCIngest(ctx))
	addHandle(apiRouter, "POST", "/api/player-event", a.HandlePlayerEvent(ctx))
	addHandle(apiRouter, "GET", "/api/chat/:repoDID", a.HandleChat(ctx))
	addHandle(apiRouter, "GET", "/api/websocket/:repoDID", a.HandleWebsocket(ctx))
	addHandle(apiRouter, "GET", "/api/livestream/:repoDID", a.HandleLivestream(ctx))
	addHandle(apiRouter, "GET", "/api/segment/recent", a.HandleRecentSegments(ctx))
	addHandle(apiRouter, "GET", "/api/segment/recent/:repoDID", a.HandleUserRecentSegments(ctx))
	addHandle(apiRouter, "GET", "/api/bluesky/resolve/:handle", a.HandleBlueskyResolve(ctx))
	addHandle(apiRouter, "GET", "/api/view-count/:user", a.HandleViewCount(ctx))
	addHandle(apiRouter, "GET", "/api/clip/:user/:file", a.HandleClip(ctx))
	apiRouter.NotFound = a.HandleAPI404(ctx)
	apiRouterHandler := a.RateLimitMiddleware(ctx)(apiRouter)
	xrpcHandler := a.RateLimitMiddleware(ctx)(xrpc)
	router.Handler("GET", "/api/*resource", apiRouterHandler)
	router.Handler("POST", "/api/*resource", apiRouterHandler)
	router.Handler("PUT", "/api/*resource", apiRouterHandler)
	router.Handler("PATCH", "/api/*resource", apiRouterHandler)
	router.Handler("DELETE", "/api/*resource", apiRouterHandler)
	router.Handler("GET", "/xrpc/*resource", xrpcHandler)
	router.Handler("POST", "/xrpc/*resource", xrpcHandler)
	router.Handler("PUT", "/xrpc/*resource", xrpcHandler)
	router.Handler("PATCH", "/xrpc/*resource", xrpcHandler)
	router.Handler("DELETE", "/xrpc/*resource", xrpcHandler)
	router.GET("/.well-known/did.json", a.HandleDidJSON(ctx))
	router.GET("/.well-known/atproto-did", a.HandleAtprotoDID(ctx))
	router.GET("/dl/*params", a.HandleAppDownload(ctx))
	router.POST("/", a.HandleWebRTCIngest(ctx))
	for _, redirect := range a.CLI.Redirects {
		parts := strings.Split(redirect, ":")
		if len(parts) != 2 {
			log.Error(ctx, "invalid redirect", "redirect", redirect)
			return nil, fmt.Errorf("invalid redirect: %s", redirect)
		}
		router.Handle("GET", parts[0], func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
			http.Redirect(w, r, parts[1], http.StatusTemporaryRedirect)
		})
	}
	if a.CLI.FrontendProxy != "" {
		u, err := url.Parse(a.CLI.FrontendProxy)
		if err != nil {
			return nil, err
		}
		log.Warn(ctx, "using frontend proxy instead of bundled frontend", "destination", a.CLI.FrontendProxy)
		router.NotFound = &httputil.ReverseProxy{
			Rewrite: func(r *httputil.ProxyRequest) {
				// workaround for Expo disliking serving requests from 127.0.0.1 instead of localhost
				// we need to use 127.0.0.1 because the atproto oauth client requires it
				r.Out.Header.Set("Origin", u.String())
				r.SetURL(u)
			},
		}
	} else {
		files, err := app.Files()
		if err != nil {
			return nil, err
		}
		index, err := files.Open("index.html")
		if err != nil {
			return nil, err
		}
		bs, err := io.ReadAll(index)
		if err != nil {
			return nil, err
		}
		linker, err := linking.NewLinker(ctx, bs)
		if err != nil {
			return nil, err
		}
		linkingHandler, err := a.NotFoundLinkingHandler(ctx, linker)
		if err != nil {
			return nil, err
		}
		router.NotFound = middlewarestd.Handler("/*static", mdlw, linkingHandler)
	}
	// needed because the WebRTC handler issues 405s from / otherwise
	router.GET("/", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		router.NotFound.ServeHTTP(w, r)
	})
	handler := sloghttp.Recovery(router)
	handler = cors.AllowAll().Handler(handler)
	handler = sloghttp.New(slog.Default())(handler)
	handler = a.RateLimitMiddleware(ctx)(handler)

	// this needs to be LAST so nothing else clobbers the context
	handler = a.ContextMiddleware(ctx)(handler)

	return handler, nil
}
func (a *StreamplaceAPI) ContextMiddleware(ctx context.Context) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uuid := uuid.New().String()
			ctx = log.WithLogValues(ctx, "requestID", uuid, "method", r.Method, "path", r.URL.Path)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}
func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		// we'll handle CORS ourselves, thanks
		if strings.HasPrefix(k, "Access-Control") {
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// handler that takes care of static files and otherwise returns the index.html with the correct link card data
func (a *StreamplaceAPI) NotFoundLinkingHandler(ctx context.Context, linker *linking.Linker) (http.HandlerFunc, error) {
	files, err := app.Files()
	if err != nil {
		return nil, err
	}
	fs := AppHostingFS{http.FS(files)}

	fileHandler := a.FileHandler(ctx, http.FileServer(fs))
	defaultHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		f := strings.TrimPrefix(req.URL.Path, "/")
		// under docs we need the index.html suffix due to astro rendering
		if strings.HasPrefix(req.URL.Path, "/docs") && strings.HasSuffix(req.URL.Path, "/") {
			f += "index.html"
		}
		_, err := fs.Open(f)
		if err == nil {
			fileHandler.ServeHTTP(w, req)
			return
		}
		if errors.Is(err, ErrorIndex) || f == "" {
			bs, err := linker.GenerateDefaultCard(ctx, req.URL)
			if err != nil {
				log.Error(ctx, "error generating default card", "error", err)
			}
			w.Header().Set("Content-Type", "text/html")
			if _, err := w.Write(bs); err != nil {
				log.Error(ctx, "error writing response", "error", err)
			}
		} else {
			log.Warn(ctx, "error opening file", "error", err)
			apierrors.WriteHTTPInternalServerError(w, "file not found", err)
		}
	})
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		proto := "http"
		if req.TLS != nil {
			proto = "https"
		}
		fwProto := req.Header.Get("x-forwarded-proto")
		if fwProto != "" {
			proto = fwProto
		}
		req.URL.Host = req.Host
		req.URL.Scheme = proto
		maybeHandle := strings.TrimPrefix(req.URL.Path, "/")
		repo, err := a.Model.GetRepoByHandleOrDID(maybeHandle)
		if err != nil || repo == nil {
			log.Error(ctx, "no repo found", "maybeHandle", maybeHandle)
			defaultHandler.ServeHTTP(w, req)
			return
		}
		ls, err := a.Model.GetLatestLivestreamForRepo(repo.DID)
		if err != nil || ls == nil {
			log.Error(ctx, "no livestream found", "repoDID", repo.DID)
			defaultHandler.ServeHTTP(w, req)
			return
		}
		lsv, err := ls.ToLivestreamView()
		if err != nil || lsv == nil {
			log.Error(ctx, "no livestream view found", "repoDID", repo.DID)
			defaultHandler.ServeHTTP(w, req)
			return
		}
		bs, err := linker.GenerateStreamerCard(ctx, req.URL, lsv)
		if err != nil {
			log.Error(ctx, "error generating html", "error", err)
			defaultHandler.ServeHTTP(w, req)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write(bs); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	}), nil
}

func (a *StreamplaceAPI) MistProxyHandler(ctx context.Context, tmpl string) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		if !a.CLI.HasMist() {
			apierrors.WriteHTTPNotImplemented(w, "Playback only on the Linux version for now", nil)
			return
		}
		stream := params.ByName("stream")
		if stream == "" {
			apierrors.WriteHTTPBadRequest(w, "missing stream in request", nil)
			return
		}

		fullstream := fmt.Sprintf("%s+%s", mistconfig.StreamName, stream)
		prefix := fmt.Sprintf(tmpl, fullstream)
		resource := params.ByName("resource")

		// path := strings.TrimPrefix(req.URL.EscapedPath(), "/api")

		client := &http.Client{}
		req.URL = &url.URL{
			Scheme:   "http",
			Host:     fmt.Sprintf("127.0.0.1:%d", a.CLI.MistHTTPPort),
			Path:     fmt.Sprintf("%s%s", prefix, resource),
			RawQuery: req.URL.RawQuery,
		}

		//http: Request.RequestURI can't be set in client requests.
		//http://golang.org/src/pkg/net/http/client.go
		req.RequestURI = ""

		resp, err := client.Do(req)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "error connecting to mist", err)
			return
		}
		defer resp.Body.Close()

		copyHeader(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		if _, err := io.Copy(w, resp.Body); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	}
}

func (a *StreamplaceAPI) FileHandler(ctx context.Context, fs http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		noslash := req.URL.Path[1:]
		ct, ok := a.Mimes[noslash]
		if ok {
			w.Header().Set("content-type", ct)
		}
		fs.ServeHTTP(w, req)
	}
}

func (a *StreamplaceAPI) RedirectHandler(ctx context.Context) (http.Handler, error) {
	_, tlsPort, err := net.SplitHostPort(a.CLI.HTTPSAddr)
	if err != nil {
		return nil, err
	}
	handleRedirect := func(w http.ResponseWriter, req *http.Request) {
		host, _, err := net.SplitHostPort(req.Host)
		if err != nil {
			host = req.Host
		}
		u := req.URL
		if tlsPort == "443" {
			u.Host = host
		} else {
			u.Host = net.JoinHostPort(host, tlsPort)
		}
		u.Scheme = "https"
		http.Redirect(w, req, u.String(), http.StatusTemporaryRedirect)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleRedirect)
	return mux, nil
}

type NotificationPayload struct {
	Token   string `json:"token"`
	RepoDID string `json:"repoDID"`
}

func (a *StreamplaceAPI) HandleAPI404(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(404)
	}
}

func (a *StreamplaceAPI) HandleNotification(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			log.Log(ctx, "error reading notification create", "error", err)
			w.WriteHeader(400)
			return
		}
		n := NotificationPayload{}
		err = json.Unmarshal(payload, &n)
		if err != nil {
			log.Log(ctx, "error unmarshalling notification create", "error", err)
			w.WriteHeader(400)
			return
		}
		err = a.StatefulDB.CreateNotification(n.Token, n.RepoDID)
		if err != nil {
			log.Log(ctx, "error creating notification", "error", err)
			w.WriteHeader(400)
			return
		}
		log.Log(ctx, "successfully created notification", "token", n.Token)
		w.WriteHeader(200)
		if n.RepoDID != "" {
			go func() {
				_, err := a.ATSync.SyncBlueskyRepo(ctx, n.RepoDID, a.Model)
				if err != nil {
					log.Error(ctx, "error syncing bluesky repo after notification creation", "error", err)
				}
			}()
		}
	}
}

func (a *StreamplaceAPI) HandleSegment(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		err := a.MediaManager.ValidateMP4(ctx, req.Body)
		if err != nil {
			apierrors.WriteHTTPBadRequest(w, "could not ingest segment", err)
			return
		}
		w.WriteHeader(200)
	}
}

func (a *StreamplaceAPI) HandlePlayerEvent(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, p httprouter.Params) {
		var event model.PlayerEventAPI
		if err := json.NewDecoder(req.Body).Decode(&event); err != nil {
			apierrors.WriteHTTPBadRequest(w, "could not decode JSON body", err)
			return
		}
		err := a.Model.CreatePlayerEvent(event)
		if err != nil {
			apierrors.WriteHTTPBadRequest(w, "could not create player event", err)
			return
		}
		w.WriteHeader(201)
	}
}

func (a *StreamplaceAPI) HandleRecentSegments(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		segs, err := a.Model.MostRecentSegments()
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not get segments", err)
			return
		}
		bs, err := json.Marshal(segs)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not marshal segments", err)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		if _, err := w.Write(bs); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	}
}

func (a *StreamplaceAPI) HandleUserRecentSegments(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		user := params.ByName("repoDID")
		if user == "" {
			apierrors.WriteHTTPBadRequest(w, "user required", nil)
			return
		}
		user, err := a.NormalizeUser(ctx, user)
		if err != nil {
			apierrors.WriteHTTPNotFound(w, "user not found", err)
			return
		}
		seg, err := a.Model.LatestSegmentForUser(user)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not get segments", err)
			return
		}
		streamplaceSeg, err := seg.ToStreamplaceSegment()
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not convert segment to streamplace segment", err)
			return
		}
		bs, err := json.Marshal(streamplaceSeg)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not marshal segments", err)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		if _, err := w.Write(bs); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	}
}

func (a *StreamplaceAPI) HandleViewCount(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		user := params.ByName("user")
		if user == "" {
			apierrors.WriteHTTPBadRequest(w, "user required", nil)
			return
		}
		user, err := a.NormalizeUser(ctx, user)
		if err != nil {
			apierrors.WriteHTTPNotFound(w, "user not found", err)
			return
		}
		count := spmetrics.GetViewCount(user)
		bs, err := json.Marshal(streamplace.Livestream_ViewerCount{Count: int64(count), LexiconTypeID: "place.stream.livestream#viewerCount"})
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not marshal view count", err)
			return
		}
		if _, err := w.Write(bs); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	}
}

func (a *StreamplaceAPI) HandleBlueskyResolve(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		log.Log(ctx, "got bluesky notification", "params", params)
		key, err := a.ATSync.SyncBlueskyRepo(ctx, params.ByName("handle"), a.Model)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not resolve streamplace key", err)
			return
		}
		signingKeys, err := a.Model.GetSigningKeysForRepo(key.DID)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not get signing keys", err)
			return
		}
		bs, err := json.Marshal(signingKeys)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not marshal signing keys", err)
			return
		}
		if _, err := w.Write(bs); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	}
}

type ChatResponse struct {
	Post *bsky.FeedPost `json:"post"`
	Repo *model.Repo    `json:"repo"`
	CID  string         `json:"cid"`
}

func (a *StreamplaceAPI) HandleChat(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		user := params.ByName("repoDID")
		if user == "" {
			apierrors.WriteHTTPBadRequest(w, "user required", nil)
			return
		}
		repoDID, err := a.NormalizeUser(ctx, user)
		if err != nil {
			apierrors.WriteHTTPNotFound(w, "user not found", err)
			return
		}
		replies, err := a.Model.GetReplies(repoDID)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not get replies", err)
			return
		}
		bs, err := json.Marshal(replies)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not marshal replies", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(bs); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	}
}

func (a *StreamplaceAPI) HandleLivestream(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		user := params.ByName("repoDID")
		if user == "" {
			apierrors.WriteHTTPBadRequest(w, "user required", nil)
			return
		}
		repoDID, err := a.NormalizeUser(ctx, user)
		if err != nil {
			apierrors.WriteHTTPNotFound(w, "user not found", err)
			return
		}
		livestream, err := a.Model.GetLatestLivestreamForRepo(repoDID)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not get livestream", err)
			return
		}

		doc, err := livestream.ToLivestreamView()
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not marshal livestream", err)
			return
		}

		if livestream == nil {
			apierrors.WriteHTTPNotFound(w, "no livestream found", nil)
			return
		}

		bs, err := json.Marshal(doc)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not marshal livestream", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(bs); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	}
}

func (a *StreamplaceAPI) RateLimitMiddleware(ctx context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ip, _, err := net.SplitHostPort(req.RemoteAddr)
			if err != nil {
				ip = req.RemoteAddr
			}

			if a.CLI.RateLimitPerSecond > 0 {
				limiter := a.getLimiter(ip)

				if !limiter.Allow() {
					log.Warn(ctx, "rate limit exceeded", "ip", ip, "path", req.URL.Path)
					apierrors.WriteHTTPTooManyRequests(w, "rate limit exceeded")
					return
				}
			}

			next.ServeHTTP(w, req)
		})
	}
}

// helper for getting a listener from a systemd file descriptor
func getListenerFromFD(fdName string) (net.Listener, error) {
	log.Log(context.TODO(), "getting listener from fd", "fdName", fdName, "LISTEN_PID", os.Getenv("LISTEN_PID"), "LISTEN_FDNAMES", os.Getenv("LISTEN_FDNAMES"))
	if os.Getenv("LISTEN_PID") == strconv.Itoa(os.Getpid()) {
		names := strings.Split(os.Getenv("LISTEN_FDNAMES"), ":")
		for i, name := range names {
			if name == fdName {
				f1 := os.NewFile(uintptr(i+3), fdName)
				return net.FileListener(f1)
			}
		}
	}
	return nil, nil
}

func (a *StreamplaceAPI) ServeHTTP(ctx context.Context) error {
	handler, err := a.Handler(ctx)
	if err != nil {
		return err
	}
	return a.ServerWithShutdown(ctx, handler, func(s *http.Server) error {
		ln, err := getListenerFromFD("http")
		if err != nil {
			return err
		}
		if ln == nil {
			ln, err = net.Listen("tcp", a.CLI.HTTPAddr)
			if err != nil {
				return err
			}
		} else {
			log.Warn(ctx, "api server listening for http over systemd socket", "addr", ln.Addr())
		}
		log.Log(ctx, "http server starting", "addr", ln.Addr())
		return s.Serve(ln)
	})
}

func (a *StreamplaceAPI) ServeHTTPRedirect(ctx context.Context) error {
	handler, err := a.RedirectHandler(ctx)
	if err != nil {
		return err
	}
	return a.ServerWithShutdown(ctx, handler, func(s *http.Server) error {
		ln, err := getListenerFromFD("http")
		if err != nil {
			return err
		}
		if ln == nil {
			ln, err = net.Listen("tcp", a.CLI.HTTPAddr)
			if err != nil {
				return err
			}
		} else {
			log.Warn(ctx, "http tls redirect server listening for http over systemd socket", "addr", ln.Addr())
		}
		log.Log(ctx, "http tls redirect server starting", "addr", ln.Addr())
		return s.Serve(ln)
	})
}

func (a *StreamplaceAPI) ServeHTTPS(ctx context.Context) error {
	handler, err := a.Handler(ctx)
	if err != nil {
		return err
	}
	return a.ServerWithShutdown(ctx, handler, func(s *http.Server) error {
		ln, err := getListenerFromFD("https")
		if err != nil {
			return err
		}
		if ln == nil {
			ln, err = net.Listen("tcp", a.CLI.HTTPSAddr)
			if err != nil {
				return err
			}
		} else {
			log.Warn(ctx, "https server listening for https over systemd socket", "addr", ln.Addr())
		}
		log.Log(ctx, "https server starting",
			"addr", ln.Addr(),
			"certPath", a.CLI.TLSCertPath,
			"keyPath", a.CLI.TLSKeyPath,
		)
		return s.ServeTLS(ln, a.CLI.TLSCertPath, a.CLI.TLSKeyPath)
	})
}

func (a *StreamplaceAPI) ServerWithShutdown(ctx context.Context, handler http.Handler, serve func(*http.Server) error) error {
	ctx, cancel := context.WithCancel(ctx)
	handler = gziphandler.GzipHandler(handler)
	server := http.Server{Handler: handler}
	var serveErr error
	go func() {
		serveErr = serve(&server)
		cancel()
	}()
	<-ctx.Done()
	if serveErr != nil {
		return fmt.Errorf("error in http server: %w", serveErr)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}

func (a *StreamplaceAPI) HandleHealthz(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(200)
	}
}

func (a *StreamplaceAPI) getLimiter(ip string) *rate.Limiter {
	a.limitersMu.Lock()
	defer a.limitersMu.Unlock()

	limiter, exists := a.limiters[ip]
	if !exists {
		limiter = rate.NewLimiter(rate.Limit(a.CLI.RateLimitPerSecond), a.CLI.RateLimitBurst)
		a.limiters[ip] = limiter
	}

	return limiter
}

func (a *StreamplaceAPI) HandleClip(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		user := params.ByName("user")
		file := params.ByName("file")
		if user == "" || file == "" {
			apierrors.WriteHTTPBadRequest(w, "user and file required", nil)
			return
		}
		user, err := a.NormalizeUser(ctx, user)
		if err != nil {
			apierrors.WriteHTTPNotFound(w, "user not found", err)
			return
		}
		fPath := []string{user, "clips", file}
		exists, err := a.CLI.DataFileExists(fPath)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not check if file exists", err)
			return
		}
		if !exists {
			apierrors.WriteHTTPNotFound(w, "file not found", nil)
			return
		}
		fd, err := os.Open(a.CLI.DataFilePath(fPath))
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not open file", err)
			return
		}
		defer fd.Close()
		w.Header().Set("Content-Type", "video/mp4")
		if _, err := io.Copy(w, fd); err != nil {
			log.Error(ctx, "error writing response", "error", err)
		}
	}
}

func NewWebsocketTracker(maxConns int) *WebsocketTracker {
	return &WebsocketTracker{
		connections:   make(map[string]int),
		maxConnsPerIP: maxConns,
	}
}

func (t *WebsocketTracker) AddConnection(ip string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	count := t.connections[ip]

	if count >= t.maxConnsPerIP {
		return false
	}

	t.connections[ip] = count + 1
	return true
}

func (t *WebsocketTracker) RemoveConnection(ip string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	count := t.connections[ip]
	if count > 0 {
		t.connections[ip] = count - 1
	}

	if t.connections[ip] == 0 {
		delete(t.connections, ip)
	}
}
