package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"github.com/labstack/echo/v4"
	"github.com/rs/cors"
	sloghttp "github.com/samber/slog-http"
	"golang.org/x/time/rate"
	"stream.place/streamplace/pkg/appbsky"

	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/js/app"
	web "stream.place/streamplace/js/web"
	"stream.place/streamplace/pkg/acme"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/signers/eip712"
	"stream.place/streamplace/pkg/director"
	apierrors "stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/linking"
	"stream.place/streamplace/pkg/localdb"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/mist/mistconfig"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/notifications"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spxrpc"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/upload"
	"stream.place/streamplace/pkg/viewlog"

	metrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	echomiddleware "github.com/slok/go-http-metrics/middleware/echo"
	httproutermiddleware "github.com/slok/go-http-metrics/middleware/httprouter"
	middlewarestd "github.com/slok/go-http-metrics/middleware/std"
)

type StreamplaceAPI struct {
	CLI           *config.CLI
	Model         model.Model
	StatefulDB    *statedb.StatefulDB
	LocalDB       localdb.LocalDB
	Updater       *Updater
	Signer        *eip712.EIP712Signer
	Mimes         map[string]string
	Notifier      notifications.Notifier
	MediaManager  *media.MediaManager
	MediaSigner   media.MediaSigner
	UploadManager *upload.Manager
	PlaybackStore blob.Store
	ViewLog       *viewlog.Writer
	XRPCServer    *spxrpc.Server
	// ACME, when set, supplies TLS certificates for every TLS listener and
	// answers HTTP-01 challenges on the redirect listener.
	ACME *acme.Manager
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

	// override tls port for http redirect server if we're using systemd file descriptors
	HTTPRedirectTLSPort *int

	rtmpSessions             map[string]*media.RTMPSession
	rtmpSessionsLock         sync.Mutex
	rtmpInternalPlaybackAddr string
}

type WebsocketTracker struct {
	connections   map[string]int
	maxConnsPerIP int
	mu            sync.RWMutex
}

func MakeStreamplaceAPI(cli *config.CLI, mod model.Model, statefulDB *statedb.StatefulDB, noter notifications.Notifier, mm *media.MediaManager, ms media.MediaSigner, bus *bus.Bus, atsync *atproto.ATProtoSynchronizer, d *director.Director, op *oatproxy.OATProxy, ldb localdb.LocalDB, um *upload.Manager, playbackStore blob.Store, viewLog *viewlog.Writer) (*StreamplaceAPI, error) {
	updater, err := PrepareUpdater(cli)
	if err != nil {
		return nil, err
	}
	a := &StreamplaceAPI{CLI: cli,
		Model:            mod,
		StatefulDB:       statefulDB,
		Updater:          updater,
		Notifier:         noter,
		MediaManager:     mm,
		MediaSigner:      ms,
		UploadManager:    um,
		PlaybackStore:    playbackStore,
		ViewLog:          viewLog,
		Aliases:          map[string]string{},
		Bus:              bus,
		ATSync:           atsync,
		Director:         d,
		connTracker:      NewWebsocketTracker(cli.RateLimitWebsocket),
		limiters:         make(map[string]*rate.Limiter),
		SignerCache:      make(map[string]media.MediaSigner),
		op:               op,
		rtmpSessions:     make(map[string]*media.RTMPSession),
		rtmpSessionsLock: sync.Mutex{},
		LocalDB:          ldb,
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
	xrpc, err := spxrpc.NewServer(ctx, a.CLI, a.Model, a.StatefulDB, a.op, mdlw, a.ATSync, a.Bus, a.LocalDB, a.MediaManager, a.UploadManager, a.PlaybackStore, a.ViewLog, a.Aliases)
	if err != nil {
		return nil, err
	}
	a.XRPCServer = xrpc.(*spxrpc.Server)
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
	addFunc(apiRouter, "DELETE", "/api/notification", a.HandleNotificationDelete(ctx))
	addFunc(apiRouter, "GET", "/api/notification/vapid-public-key", a.HandleVapidPublicKey(ctx))
	// old clients
	addFunc(router, "GET", "/app-updates", a.HandleAppUpdates(ctx))
	// new ones
	addFunc(apiRouter, "GET", "/api/manifest", a.HandleAppUpdates(ctx))
	addHandle(apiRouter, "GET", "/api/desktop-updates/:platform/:architecture/:version/:buildTime/:file", a.HandleDesktopUpdates(ctx))
	addHandle(apiRouter, "POST", "/api/webrtc/:stream", a.viewerGate(a.MistProxyHandler(ctx, "/webrtc/%s")))
	addHandle(apiRouter, "OPTIONS", "/api/webrtc/:stream", a.viewerGate(a.MistProxyHandler(ctx, "/webrtc/%s")))
	addHandle(apiRouter, "DELETE", "/api/webrtc/:stream", a.viewerGate(a.MistProxyHandler(ctx, "/webrtc/%s")))
	addFunc(apiRouter, "POST", "/api/segment", a.HandleSegment(ctx))
	addFunc(apiRouter, "GET", "/api/healthz", a.HandleHealthz(ctx))
	// they're jpegs now
	addHandle(apiRouter, "GET", "/api/playback/:user/stream.jpg", a.viewerGate(a.HandleThumbnailPlayback(ctx)))
	// this one is actually a jpeg (used previously and shouldn't remove for historical reasons)
	addHandle(apiRouter, "GET", "/api/playback/:user/stream.png", a.viewerGate(a.HandleThumbnailPlayback(ctx)))
	addHandle(apiRouter, "GET", "/api/app-return/*anything", a.HandleAppReturn(ctx))
	addHandle(apiRouter, "POST", "/api/playback/:user/webrtc", a.viewerGate(a.HandleWebRTCPlayback(ctx)))
	addHandle(apiRouter, "POST", "/api/ingest/webrtc", a.HandleWebRTCIngest(ctx))
	addHandle(apiRouter, "POST", "/api/ingest/webrtc/:key", a.HandleWebRTCIngest(ctx))
	addHandle(apiRouter, "POST", "/api/player-event", a.viewerGate(a.HandlePlayerEvent(ctx)))
	addHandle(apiRouter, "GET", "/api/chat/:repoDID", a.viewerGate(a.HandleChat(ctx)))
	addHandle(apiRouter, "GET", "/api/websocket/:repoDID", a.viewerGate(a.HandleWebsocket(ctx)))
	addHandle(apiRouter, "GET", "/api/livestream/:repoDID", a.viewerGate(a.HandleLivestream(ctx)))
	addHandle(apiRouter, "GET", "/api/bluesky/resolve/:handle", a.HandleBlueskyResolve(ctx))
	addHandle(apiRouter, "GET", "/api/view-count/:user", a.viewerGate(a.HandleViewCount(ctx)))
	addHandle(apiRouter, "GET", "/api/clip/:user/:file", a.viewerGate(a.HandleClip(ctx)))
	if a.UploadManager != nil {
		// Don't wrap in middlewarestd.Handler (go-http-metrics): its
		// responseWriterInterceptor doesn't implement Unwrap, which would
		// hide net/http.ResponseController.SetReadDeadline from tusd and
		// produce one NetworkTimeoutError warning per PATCH chunk.
		apiRouter.Handler("HEAD", "/api/upload/:id", a.UploadManager)
		apiRouter.Handler("PATCH", "/api/upload/:id", a.UploadManager)
		apiRouter.Handler("DELETE", "/api/upload/:id", a.UploadManager)
		apiRouter.Handler("OPTIONS", "/api/upload/:id", a.UploadManager)
	}
	apiRouter.NotFound = a.HandleAPI404(ctx)
	apiRouterHandler := a.RateLimitMiddleware(ctx)(apiRouter)
	xrpcHandler := a.RateLimitMiddleware(ctx)(xrpc)
	router.Handler("GET", "/api/*resource", apiRouterHandler)
	router.Handler("HEAD", "/api/*resource", apiRouterHandler)
	router.Handler("POST", "/api/*resource", apiRouterHandler)
	router.Handler("PUT", "/api/*resource", apiRouterHandler)
	router.Handler("PATCH", "/api/*resource", apiRouterHandler)
	router.Handler("DELETE", "/api/*resource", apiRouterHandler)
	router.Handler("OPTIONS", "/api/*resource", apiRouterHandler)
	router.Handler("GET", "/xrpc/*resource", xrpcHandler)
	router.Handler("POST", "/xrpc/*resource", xrpcHandler)
	router.Handler("PUT", "/xrpc/*resource", xrpcHandler)
	router.Handler("PATCH", "/xrpc/*resource", xrpcHandler)
	router.Handler("DELETE", "/xrpc/*resource", xrpcHandler)
	// i wonder if there's a better way to do this?
	router.GET("/linkbanner.png", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		if err := a.XRPCServer.HandleLinkBanner(echo.New().NewContext(r, w)); err != nil {
			log.Error(ctx, "error handling linkbanner.png", "error", err)
			w.WriteHeader(500)
		}
	})
	router.GET("/favicon.ico", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		err := a.XRPCServer.HandleFaviconICO(echo.New().NewContext(r, w))
		if err != nil {
			log.Error(ctx, "error handling favicon.ico", "error", err)
			w.WriteHeader(500)
			return
		}
	})
	router.GET("/.well-known/did.json", a.HandleDidJSON(ctx))
	router.GET("/.well-known/atproto-did", a.HandleAtprotoDID(ctx))
	router.GET("/dl/*params", a.HandleAppDownload(ctx))
	router.POST("/", a.HandleWebRTCIngest(ctx))
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
		// Always load both frontends. The NotFound dispatcher picks one per
		// request based on the sp_web_beta cookie (or the --frontend CLI
		// flag, which forces web for everyone).
		frontends, err := a.loadFrontends(ctx)
		if err != nil {
			return nil, err
		}
		linkingHandler, err := a.NotFoundLinkingHandler(ctx, frontends, a.CLI.Frontend == "web")
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
	// sloghttp.New wraps the ResponseWriter with a bodyWriter that doesn't
	// implement Unwrap, which hides net/http.ResponseController.SetReadDeadline
	// from tusd and floods the logs with one NetworkTimeoutError warning per
	// PATCH chunk. Skip it for /api/upload/* so the response writer stays
	// unwrapped on the upload path; tusd has its own per-request logging.
	preLog := handler
	logged := sloghttp.New(slog.Default())(handler)
	handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, upload.BasePath) {
			preLog.ServeHTTP(w, r)
			return
		}
		logged.ServeHTTP(w, r)
	})
	handler = a.RateLimitMiddleware(ctx)(handler)
	redirectMiddleware, err := a.RedirectMiddleware()
	if err != nil {
		return nil, err
	}
	handler = redirectMiddleware(handler)

	// this needs to be LAST so nothing else clobbers the context
	handler = a.ContextMiddleware(ctx)(handler)

	return handler, nil
}
func (a *StreamplaceAPI) ContextMiddleware(parentContext context.Context) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uuid := uuid.New().String()
			ctx := log.WithLogValues(parentContext, "requestID", uuid, "method", r.Method, "path", r.URL.Path)
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

// frontendSet holds a per-frontend linking handler so the NotFound
// dispatcher can pick which one to serve per request. The legacy RN app
// and the new Vite web app each get their own; the sp_web_beta cookie
// opts a user into the new one unless the operator forced it via
// --frontend=web (in which case forceWeb is true on pick).
type frontendSet struct {
	app http.HandlerFunc
	web http.HandlerFunc
}

func (f *frontendSet) pick(r *http.Request, forceWeb bool) http.HandlerFunc {
	if forceWeb {
		return f.web
	}
	if c, err := r.Cookie("sp_web_beta"); err == nil && c.Value == "1" {
		return f.web
	}
	return f.app
}

func (a *StreamplaceAPI) loadFrontends(ctx context.Context) (*frontendSet, error) {
	appHandler, err := a.buildLinkingHandler(ctx, app.Files)
	if err != nil {
		return nil, fmt.Errorf("loading app frontend: %w", err)
	}
	webHandler, err := a.buildLinkingHandler(ctx, web.Files)
	if err != nil {
		return nil, fmt.Errorf("loading web frontend: %w", err)
	}
	return &frontendSet{app: appHandler, web: webHandler}, nil
}

// buildLinkingHandler builds the static-file + link-card handler for a
// single frontend.
func (a *StreamplaceAPI) buildLinkingHandler(ctx context.Context, load func() (fs.FS, error)) (http.HandlerFunc, error) {
	frontendFS, err := load()
	if err != nil {
		return nil, err
	}
	index, err := frontendFS.Open("index.html")
	if err != nil {
		return nil, err
	}
	bs, err := io.ReadAll(index)
	if err != nil {
		return nil, err
	}
	linker, err := linking.NewLinker(ctx, bs, a.StatefulDB, a.CLI)
	if err != nil {
		return nil, err
	}
	return a.notFoundLinkingHandler(ctx, linker, frontendFS)
}

// NotFoundLinkingHandler dispatches to the per-frontend handler that
// matches the current request. The sp_web_beta cookie opts users into the
// new web frontend; forceWeb (driven by --frontend=web) makes it stick
// for everyone. The dev proxy (--dev-frontend-proxy) is handled upstream
// and never reaches this handler.
func (a *StreamplaceAPI) NotFoundLinkingHandler(ctx context.Context, frontends *frontendSet, forceWeb bool) (http.HandlerFunc, error) {
	return func(w http.ResponseWriter, req *http.Request) {
		frontends.pick(req, forceWeb)(w, req)
	}, nil
}

// notFoundLinkingHandler serves static files and link cards for a single
// frontend. Each frontend gets its own linker (built from that
// frontend's index.html) so OpenGraph cards match the look of whichever
// site the user is on.
func (a *StreamplaceAPI) notFoundLinkingHandler(ctx context.Context, linker *linking.Linker, frontendFS fs.FS) (http.HandlerFunc, error) {
	files := frontendFS
	fsys := AppHostingFS{http.FS(files)}

	fileHandler := a.FileHandler(ctx, http.FileServer(fsys))
	defaultHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		f := strings.TrimPrefix(req.URL.Path, "/")
		// under docs we need the index.html suffix due to astro rendering
		if strings.HasPrefix(req.URL.Path, "/docs") && strings.HasSuffix(req.URL.Path, "/") {
			f += "index.html"
		}
		_, err := fsys.Open(f)
		if err == nil {
			fileHandler.ServeHTTP(w, req)
			return
		}
		if errors.Is(err, ErrorIndex) || f == "" {
			bs, err := linker.GenerateDefaultCard(ctx, req.URL, a.CLI.SentryDSN)
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
		if !a.viewerAllowed(req) {
			// Private node, unknown visitor: the app shell and its assets with
			// the node's branding baked into the page (so the sign-in wall
			// paints branded on first render), but no stream or profile cards.
			defaultHandler.ServeHTTP(w, req)
			return
		}
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

		// VOD link cards live at /<user>/video/<tid>. Everything else with a
		// slash in it falls through to static-file / default-card handling.
		parts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
		if len(parts) == 3 && parts[1] == "video" {
			if a.writeVideoCard(ctx, w, req, linker, parts[0], parts[2]) {
				return
			}
			defaultHandler.ServeHTTP(w, req)
			return
		}

		maybeHandle := strings.TrimPrefix(req.URL.Path, "/")
		// quick check for things that aren't valid handles/dids
		if strings.ContainsAny(maybeHandle, "/_") {
			defaultHandler.ServeHTTP(w, req)
			return
		}
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
		bs, err := linker.GenerateStreamerCard(ctx, req.URL, lsv, a.CLI.SentryDSN)
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

// writeVideoCard renders an OpenGraph link card for a VOD at
// /<user>/video/<tid>, pulling the video out of the local index. It returns
// true when it wrote a response, and false (having written nothing) when the
// video can't be found or rendered, so the caller can fall back to
// static-file / default-card handling.
func (a *StreamplaceAPI) writeVideoCard(ctx context.Context, w http.ResponseWriter, req *http.Request, linker *linking.Linker, user, tid string) bool {
	repo, err := a.Model.GetRepoByHandleOrDID(user)
	if err != nil || repo == nil {
		return false
	}
	uri := fmt.Sprintf("at://%s/place.stream.video/%s", repo.DID, tid)
	vv, err := a.Model.GetVideoView(ctx, uri)
	if err != nil {
		log.Error(ctx, "error fetching video view for card", "uri", uri, "error", err)
		return false
	}
	if vv == nil {
		return false
	}
	bs, err := linker.GenerateVideoCard(ctx, req.URL, vv, a.CLI.SentryDSN)
	if err != nil {
		log.Error(ctx, "error generating video card", "uri", uri, "error", err)
		return false
	}
	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write(bs); err != nil {
		log.Error(ctx, "error writing response", "error", err)
	}
	return true
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
	var tlsPort string
	var err error
	if a.HTTPRedirectTLSPort != nil {
		tlsPort = fmt.Sprintf("%d", *a.HTTPRedirectTLSPort)
	} else {
		_, tlsPort, err = net.SplitHostPort(a.CLI.HTTPSAddr)
		if err != nil {
			return nil, err
		}
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
	Type    string `json:"type"`
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
		err = a.StatefulDB.CreateNotification(n.Token, n.RepoDID, statedb.NotificationType(n.Type))
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

// HandleNotificationDelete removes a push token (web or mobile). Used by the
// web client when a user disables notifications — the browser subscription is
// unsubscribed locally and the server row is pruned so we stop pushing to a
// dead endpoint.
func (a *StreamplaceAPI) HandleNotificationDelete(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			log.Log(ctx, "error reading notification delete", "error", err)
			w.WriteHeader(400)
			return
		}
		n := NotificationPayload{}
		if err := json.Unmarshal(payload, &n); err != nil {
			log.Log(ctx, "error unmarshalling notification delete", "error", err)
			w.WriteHeader(400)
			return
		}
		if n.Token == "" {
			w.WriteHeader(400)
			return
		}
		if err := a.StatefulDB.DeleteNotification(n.Token); err != nil {
			log.Log(ctx, "error deleting notification", "error", err)
			w.WriteHeader(400)
			return
		}
		log.Log(ctx, "successfully deleted notification", "token", n.Token)
		w.WriteHeader(200)
	}
}

// HandleVapidPublicKey returns the server's Web Push VAPID public key. The web
// client needs it to subscribe the browser's PushManager. The key is generated
// on first access (via EnsureVAPIDKeys) and stays stable thereafter.
func (a *StreamplaceAPI) HandleVapidPublicKey(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		keys, err := a.StatefulDB.EnsureVAPIDKeys(ctx)
		if err != nil {
			log.Error(ctx, "error ensuring vapid keys", "error", err)
			apierrors.WriteHTTPInternalServerError(w, "unable to get vapid public key", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		resp := struct {
			PublicKey string `json:"publicKey"`
		}{PublicKey: keys.PublicKey}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Error(ctx, "error writing vapid public key", "error", err)
		}
	}
}

func (a *StreamplaceAPI) HandleSegment(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		err := a.MediaManager.ValidateMP4(ctx, req.Body, false)
		if err != nil {
			apierrors.WriteHTTPBadRequest(w, "could not ingest segment", err)
			return
		}
		w.WriteHeader(200)
	}
}

func (a *StreamplaceAPI) HandlePlayerEvent(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, p httprouter.Params) {
		if !a.CLI.PlayerTelemetry {
			w.WriteHeader(200)
			return
		}
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
		count := a.Bus.GetViewerCount(user)
		bs, err := json.Marshal(placestream.Livestream_ViewerCount{Count: int64(count), LexiconTypeID: "place.stream.livestream#viewerCount"})
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
	Post *appbsky.FeedPost `json:"post"`
	Repo *model.Repo       `json:"repo"`
	CID  string            `json:"cid"`
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
		if livestream == nil {
			apierrors.WriteHTTPNotFound(w, "no livestream found", nil)
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

type redirectRule struct {
	re    *regexp.Regexp
	toURL *url.URL
	rawTo string
}

// RedirectMiddleware returns a middleware that handles path redirects according to CLI.Redirects
func (a *StreamplaceAPI) RedirectMiddleware() (func(http.Handler) http.Handler, error) {
	var redirectRules []redirectRule
	for from, to := range a.CLI.Redirects {
		re, err := regexp.Compile(from)
		if err != nil {
			return nil, fmt.Errorf("invalid redirect pattern: %s (regex error: %w)", from, err)
		}
		toBase, err := url.Parse(to)
		if err != nil {
			return nil, fmt.Errorf("invalid redirect destination: %s", to)
		}
		redirectRules = append(redirectRules, redirectRule{re: re, toURL: toBase, rawTo: to})
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only match redirections for GET requests
			if r.Method == http.MethodGet {
				for _, rule := range redirectRules {
					if rule.re.MatchString(r.URL.Path) {
						// Make new URL by copying base and setting url query param
						redirectURL := *rule.toURL
						q := redirectURL.Query()
						q.Set("url", r.URL.String())
						redirectURL.RawQuery = q.Encode()
						http.Redirect(w, r, redirectURL.String(), http.StatusTemporaryRedirect)
						return
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

// helper for getting a listener from a systemd file descriptor
func getListenerFromFD(fdName string) (net.Listener, error) {
	log.Debug(context.TODO(), "getting listener from fd", "fdName", fdName, "LISTEN_PID", os.Getenv("LISTEN_PID"), "LISTEN_FDNAMES", os.Getenv("LISTEN_FDNAMES"))
	if os.Getenv("LISTEN_PID") == strconv.Itoa(os.Getpid()) {
		names := strings.Split(os.Getenv("LISTEN_FDNAMES"), ":")
		for i, name := range names {
			if name == fdName {
				log.Warn(context.TODO(), "using systemd file descriptor", "fdName", fdName, "fdIndex", i+3)
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
	if a.ACME != nil {
		handler = a.ACME.HTTPChallengeHandler(handler)
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
			// tell the redirect handler we're using systemd and they should go to 443
			port443 := 443
			a.HTTPRedirectTLSPort = &port443
			log.Warn(ctx, "https server listening for https over systemd socket", "addr", ln.Addr())
		}
		if a.ACME != nil {
			s.TLSConfig = a.ACME.TLSConfig()
			s.TLSConfig.NextProtos = append([]string{"h2", "http/1.1"}, s.TLSConfig.NextProtos...)
			log.Log(ctx, "https server starting",
				"addr", ln.Addr(),
				"acmeDomains", strings.Join(a.ACME.Domains(), ","),
			)
			return s.ServeTLS(ln, "", "")
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
	// gziphandler wraps the ResponseWriter without an Unwrap method, which
	// hides net/http.ResponseController.SetReadDeadline from tusd and produces
	// one NetworkTimeoutError warning per PATCH chunk (plus, more importantly,
	// prevents tusd from extending the read deadline mid-upload so chunks
	// taking longer than the default NetworkTimeout will fail). Skip gzip for
	// /api/upload/*; compressing opaque upload bytes is pointless anyway.
	raw := handler
	gzipped := gziphandler.GzipHandler(handler)
	handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, upload.BasePath) {
			raw.ServeHTTP(w, r)
			return
		}
		gzipped.ServeHTTP(w, r)
	})
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
