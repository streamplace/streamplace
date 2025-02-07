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
	"slices"
	"strings"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
	sloghttp "github.com/samber/slog-http"

	"stream.place/streamplace/js/app"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/signers/eip712"
	apierrors "stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/media"
	"stream.place/streamplace/pkg/mist/mistconfig"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/notifications"
	v0 "stream.place/streamplace/pkg/schema/v0"
)

type StreamplaceAPI struct {
	CLI              *config.CLI
	Model            model.Model
	Updater          *Updater
	Signer           *eip712.EIP712Signer
	Mimes            map[string]string
	FirebaseNotifier notifications.FirebaseNotifier
	MediaManager     *media.MediaManager
	MediaSigner      *media.MediaSigner
	// not thread-safe yet
	Aliases map[string]string
}

func MakeStreamplaceAPI(cli *config.CLI, mod model.Model, signer *eip712.EIP712Signer, noter notifications.FirebaseNotifier, mm *media.MediaManager, ms *media.MediaSigner) (*StreamplaceAPI, error) {
	updater, err := PrepareUpdater(cli)
	if err != nil {
		return nil, err
	}
	a := &StreamplaceAPI{CLI: cli,
		Model:            mod,
		Updater:          updater,
		Signer:           signer,
		FirebaseNotifier: noter,
		MediaManager:     mm,
		MediaSigner:      ms,
		Aliases:          map[string]string{},
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

func (fs AppHostingFS) Open(name string) (http.File, error) {
	file, err1 := fs.FileSystem.Open(name)
	if err1 == nil {
		return file, nil
	}
	if !errors.Is(err1, os.ErrNotExist) {
		return nil, err1
	}
	file, err2 := fs.FileSystem.Open(fmt.Sprintf(name + ".html"))
	if err2 == nil {
		return file, nil
	}
	if !errors.Is(err2, os.ErrNotExist) {
		return nil, err2
	}

	return fs.FileSystem.Open("index.html")
}

func (a *StreamplaceAPI) Handler(ctx context.Context) (http.Handler, error) {
	router := httprouter.New()
	apiRouter := httprouter.New()
	apiRouter.HandlerFunc("POST", "/api/notification", a.HandleNotification(ctx))
	// old clients
	router.HandlerFunc("GET", "/app-updates", a.HandleAppUpdates(ctx))
	// new ones
	apiRouter.HandlerFunc("GET", "/api/manifest", a.HandleAppUpdates(ctx))
	apiRouter.GET("/api/desktop-updates/:platform/:architecture/:version/:buildTime/:file", a.HandleDesktopUpdates(ctx))
	apiRouter.POST("/api/webrtc/:stream", a.MistProxyHandler(ctx, "/webrtc/%s"))
	apiRouter.OPTIONS("/api/webrtc/:stream", a.MistProxyHandler(ctx, "/webrtc/%s"))
	apiRouter.DELETE("/api/webrtc/:stream", a.MistProxyHandler(ctx, "/webrtc/%s"))
	apiRouter.GET("/api/hls/:stream/*resource", a.MistProxyHandler(ctx, "/hls/%s"))
	apiRouter.Handler("POST", "/api/segment", a.HandleSegment(ctx))
	apiRouter.HandlerFunc("GET", "/api/healthz", a.HandleHealthz(ctx))
	apiRouter.GET("/api/playback/:user/stream.mp4", a.HandleMP4Playback(ctx))
	apiRouter.GET("/api/playback/:user/stream.webm", a.HandleMKVPlayback(ctx))
	apiRouter.GET("/api/playback/:user/hls/:file", a.HandleHLSPlayback(ctx))
	apiRouter.GET("/api/playback/:user/stream.jpg", a.HandleThumbnailPlayback(ctx))
	apiRouter.GET("/api/app-return/*anything", a.HandleAppReturn(ctx))
	apiRouter.POST("/api/playback/:user/webrtc", a.HandleWebRTCPlayback(ctx))
	apiRouter.POST("/api/ingest/webrtc", a.HandleWebRTCIngest(ctx))
	apiRouter.POST("/api/player-event", a.HandlePlayerEvent(ctx))
	apiRouter.GET("/api/segment/recent", a.HandleRecentSegments(ctx))
	apiRouter.GET("/api/identity", a.HandleIdentityGET(ctx))
	apiRouter.PUT("/api/identity/:id", a.HandleIdentityPUT(ctx))
	apiRouter.GET("/api/bluesky/resolve/:handle", a.HandleBlueskyResolve(ctx))
	apiRouter.GET("/api/atproto-oauth/:platform", a.HandleATProtoOAuth(ctx))
	apiRouter.GET("/api/live-users", a.HandleLiveUsers(ctx))
	apiRouter.NotFound = a.HandleAPI404(ctx)
	router.Handler("GET", "/api/*resource", apiRouter)
	router.Handler("POST", "/api/*resource", apiRouter)
	router.Handler("PUT", "/api/*resource", apiRouter)
	router.Handler("PATCH", "/api/*resource", apiRouter)
	router.Handler("DELETE", "/api/*resource", apiRouter)
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
		files, err := app.Files()
		if err != nil {
			return nil, err
		}
		router.NotFound = a.FileHandler(ctx, http.FileServer(AppHostingFS{http.FS(files)}))
	}
	// needed because the WebRTC handler issues 405s from / otherwise
	router.GET("/", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		router.NotFound.ServeHTTP(w, r)
	})
	handler := sloghttp.Recovery(router)
	handler = cors.AllowAll().Handler(handler)
	handler = sloghttp.New(slog.Default())(handler)

	return handler, nil
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

		fullstream := fmt.Sprintf("%s+%s", mistconfig.STREAM_NAME, stream)
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
		io.Copy(w, resp.Body)
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
	_, tlsPort, err := net.SplitHostPort(a.CLI.HttpsAddr)
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

func (a *StreamplaceAPI) HandleIdentityPUT(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		id := params.ByName("id")
		if id == "" {
			apierrors.WriteHTTPBadRequest(w, "id required", nil)
			return
		}
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			apierrors.WriteHTTPBadRequest(w, "error reading body", err)
			return
		}
		signed, err := a.Signer.Verify(payload)
		if err != nil {
			apierrors.WriteHTTPBadRequest(w, "could not verify signature on payload", err)
			return
		}
		ident, ok := signed.Data().(*v0.Identity)
		if !ok {
			log.Log(ctx, "got signed payload but it wasn't a golive")
			apierrors.WriteHTTPBadRequest(w, "not a golive", nil)
			return
		}
		if signed.Signer() != a.CLI.AdminAccount {
			log.Log(ctx, "wrong user tried to golive", "signer", signed.Signer(), "admin", a.CLI.AdminAccount)
			apierrors.WriteHTTPForbidden(w, "admins only for now", nil)
			return
		}
		log.Log(ctx, "got signed & verified payload", "payload", signed)
		if err := a.Model.UpdateIdentity(&model.Identity{
			ID:     id,
			Handle: ident.Handle,
			DID:    ident.DID,
		}); err != nil {
			apierrors.WriteHTTPInternalServerError(w, "unable to update settings", err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
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
		err = a.Model.CreateNotification(n.Token, n.RepoDID)
		if err != nil {
			log.Log(ctx, "error creating notification", "error", err)
			w.WriteHeader(400)
			return
		}
		log.Log(ctx, "successfully created notification", "token", n.Token)
		w.WriteHeader(200)
		if n.RepoDID != "" {
			go func() {
				_, err := atproto.SyncBlueskyRepo(ctx, n.RepoDID, a.Model)
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
		w.Write(bs)
	}
}

func (a *StreamplaceAPI) HandleLiveUsers(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		repos, err := a.Model.GetLiveUsers()
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not get live users", err)
			return
		}
		bs, err := json.Marshal(repos)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not marshal live users", err)
			return
		}
		w.Write(bs)
	}
}

func (a *StreamplaceAPI) HandleIdentityGET(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		id := a.MediaSigner.Pub.String()

		ident, err := a.Model.GetIdentity(id)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "unable to get settings", err)
			return
		}

		bs, err := json.Marshal(ident)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "unable to marshal json", err)
			return
		}
		w.Write(bs)
	}
}

func (a *StreamplaceAPI) HandleBlueskyResolve(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		log.Log(ctx, "got bluesky notification", "params", params)
		key, err := atproto.SyncBlueskyRepo(ctx, params.ByName("handle"), a.Model)
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
		w.Write(bs)
	}
}

func (a *StreamplaceAPI) HandleATProtoOAuth(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		host, _, err := net.SplitHostPort(req.Host)
		if err != nil {
			host = req.Host
		}
		platform := params.ByName("platform")
		if !slices.Contains(atproto.AllowedPlatforms, platform) {
			apierrors.WriteHTTPBadRequest(w, "unsupported platform", nil)
			return
		}

		meta := atproto.GetMetadata(host, platform, a.CLI.AppBundleID)
		bs, err := json.Marshal(meta)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not marshal metadata", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(bs)
	}
}

func (a *StreamplaceAPI) ServeHTTP(ctx context.Context) error {
	handler, err := a.Handler(ctx)
	if err != nil {
		return err
	}
	return a.ServerWithShutdown(ctx, handler, func(s *http.Server) error {
		s.Addr = a.CLI.HttpAddr
		log.Log(ctx, "http server starting", "addr", s.Addr)
		return s.ListenAndServe()
	})
}

func (a *StreamplaceAPI) ServeHTTPRedirect(ctx context.Context) error {
	handler, err := a.RedirectHandler(ctx)
	if err != nil {
		return err
	}
	return a.ServerWithShutdown(ctx, handler, func(s *http.Server) error {
		s.Addr = a.CLI.HttpAddr
		log.Log(ctx, "http tls redirecct server starting", "addr", s.Addr)
		return s.ListenAndServe()
	})
}

func (a *StreamplaceAPI) ServeHTTPS(ctx context.Context) error {
	handler, err := a.Handler(ctx)
	if err != nil {
		return err
	}
	return a.ServerWithShutdown(ctx, handler, func(s *http.Server) error {
		s.Addr = a.CLI.HttpsAddr
		log.Log(ctx, "https server starting",
			"addr", s.Addr,
			"certPath", a.CLI.TLSCertPath,
			"keyPath", a.CLI.TLSKeyPath,
		)
		return s.ListenAndServeTLS(a.CLI.TLSCertPath, a.CLI.TLSKeyPath)
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
