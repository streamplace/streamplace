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
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
	sloghttp "github.com/samber/slog-http"

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
	"stream.place/streamplace/pkg/renditions"
	"stream.place/streamplace/pkg/spmetrics"
	"stream.place/streamplace/pkg/spxrpc"
	"stream.place/streamplace/pkg/streamplace"
)

type StreamplaceAPI struct {
	CLI              *config.CLI
	Model            model.Model
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
}

func MakeStreamplaceAPI(cli *config.CLI, mod model.Model, signer *eip712.EIP712Signer, noter notifications.FirebaseNotifier, mm *media.MediaManager, ms media.MediaSigner, bus *bus.Bus, atsync *atproto.ATProtoSynchronizer, d *director.Director) (*StreamplaceAPI, error) {
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
		Bus:              bus,
		ATSync:           atsync,
		Director:         d,
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
	if !errors.Is(err1, os.ErrNotExist) {
		return nil, err1
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
	xrpc, err := spxrpc.NewServer(a.CLI, a.Model)
	if err != nil {
		return nil, err
	}
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
	apiRouter.Handler("POST", "/api/segment", a.HandleSegment(ctx))
	apiRouter.HandlerFunc("GET", "/api/healthz", a.HandleHealthz(ctx))
	apiRouter.GET("/api/playback/:user/hls/*file", a.HandleHLSPlayback(ctx))
	apiRouter.GET("/api/playback/:user/stream.mp4", a.HandleMP4Playback(ctx))
	apiRouter.GET("/api/playback/:user/stream.webm", a.HandleMKVPlayback(ctx))
	// they're, uh, not jpegs. but we used this once and i don't wanna break backwards compatibility
	apiRouter.GET("/api/playback/:user/stream.jpg", a.HandleThumbnailPlayback(ctx))
	// this one is not a lie
	apiRouter.GET("/api/playback/:user/stream.png", a.HandleThumbnailPlayback(ctx))
	apiRouter.GET("/api/app-return/*anything", a.HandleAppReturn(ctx))
	apiRouter.POST("/api/playback/:user/webrtc", a.HandleWebRTCPlayback(ctx))
	apiRouter.POST("/api/ingest/webrtc", a.HandleWebRTCIngest(ctx))
	apiRouter.POST("/api/ingest/webrtc/:key", a.HandleWebRTCIngest(ctx))
	apiRouter.POST("/api/player-event", a.HandlePlayerEvent(ctx))
	apiRouter.GET("/api/chat/:repoDID", a.HandleChat(ctx))
	apiRouter.GET("/api/websocket/:repoDID", a.HandleWebsocket(ctx))
	apiRouter.GET("/api/livestream/:repoDID", a.HandleLivestream(ctx))
	apiRouter.GET("/api/segment/recent", a.HandleRecentSegments(ctx))
	apiRouter.GET("/api/segment/recent/:repoDID", a.HandleUserRecentSegments(ctx))
	apiRouter.GET("/api/bluesky/resolve/:handle", a.HandleBlueskyResolve(ctx))
	for _, platform := range atproto.AllowedPlatforms {
		apiRouter.GET(fmt.Sprintf("/api/atproto-oauth/%s", platform), a.HandleATProtoOAuth(ctx, platform))
	}
	apiRouter.GET("/api/live-users", a.HandleLiveUsers(ctx))
	apiRouter.GET("/api/view-count/:user", a.HandleViewCount(ctx))
	apiRouter.NotFound = a.HandleAPI404(ctx)
	router.Handler("GET", "/api/*resource", apiRouter)
	router.Handler("POST", "/api/*resource", apiRouter)
	router.Handler("PUT", "/api/*resource", apiRouter)
	router.Handler("PATCH", "/api/*resource", apiRouter)
	router.Handler("DELETE", "/api/*resource", apiRouter)
	router.Handler("GET", "/xrpc/*resource", xrpc)
	router.Handler("POST", "/xrpc/*resource", xrpc)
	router.Handler("PUT", "/xrpc/*resource", xrpc)
	router.Handler("PATCH", "/xrpc/*resource", xrpc)
	router.Handler("DELETE", "/xrpc/*resource", xrpc)
	router.GET("/.well-known/did.json", a.HandleDidJson(ctx))
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
		router.NotFound = linkingHandler
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
		_, err := fs.Open(f)
		if err == nil {
			fileHandler.ServeHTTP(w, req)
			return
		} else if errors.Is(err, ErrorIndex) || f == "" {
			bs, err := linker.GenerateDefaultCard(ctx, req.URL)
			if err != nil {
				log.Error(ctx, "error generating default card", "error", err)
			}
			w.Header().Set("Content-Type", "text/html")
			w.Write(bs)
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
		w.Write(bs)
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
		w.Write(bs)
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
		w.Write(bs)
	}
}

type LiveUsersResponse struct {
	model.Segment
	Viewers int `json:"viewers"`
}

func (a *StreamplaceAPI) HandleLiveUsers(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		repos, err := a.Model.MostRecentSegments()
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not get live users", err)
			return
		}
		liveUsers := []LiveUsersResponse{}
		for _, repo := range repos {
			viewers := spmetrics.GetViewCount(repo.RepoDID)
			liveUsers = append(liveUsers, LiveUsersResponse{
				Segment: repo,
				Viewers: viewers,
			})
		}
		bs, err := json.Marshal(liveUsers)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not marshal live users", err)
			return
		}
		w.Write(bs)
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
		w.Write(bs)
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
		w.Write(bs)
	}
}

func (a *StreamplaceAPI) HandleATProtoOAuth(ctx context.Context, platform string) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		host, _, err := net.SplitHostPort(req.Host)
		if err != nil {
			host = req.Host
		}
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
		w.Write(bs)
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
		w.Write(bs)
	}
}

// todo: does this mean a whole message has to fit within the buffer?
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (a *StreamplaceAPI) HandleWebsocket(ctx context.Context) httprouter.Handle {
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
		conn, err := upgrader.Upgrade(w, req, nil)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not upgrade to websocket", err)
			return
		}
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		defer conn.Close()
		initialBurst := make(chan any, 200)
		go func() {

			ch := a.Bus.Subscribe(repoDID)
			defer a.Bus.Unsubscribe(repoDID, ch)
			// Create a ticker that fires every 3 seconds
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()

			send := func(msg any) {
				bs, err := json.Marshal(msg)
				if err != nil {
					log.Error(ctx, "could not marshal message", "error", err)
					return
				}
				log.Debug(ctx, "sending message", "message", string(bs))
				err = conn.WriteMessage(websocket.TextMessage, bs)
				if err != nil {
					log.Error(ctx, "could not write message", "error", err)
					return
				}
			}

			for {
				select {
				case msg := <-ch:
					send(msg)
				case msg := <-initialBurst:
					send(msg)
				case <-ticker.C:
					count := spmetrics.GetViewCount(repoDID)
					bs, err := json.Marshal(streamplace.Livestream_ViewerCount{Count: int64(count), LexiconTypeID: "place.stream.livestream#viewerCount"})
					if err != nil {
						log.Error(ctx, "could not marshal view count", "error", err)
						continue
					}
					err = conn.WriteMessage(websocket.TextMessage, bs)
					if err != nil {
						log.Error(ctx, "could not write ping message", "error", err)
						return
					}
				case <-ctx.Done():
					log.Debug(ctx, "context done, stopping websocket sender")
					return
				}
			}
		}()

		go func() {
			seg, err := a.Model.LatestSegmentForUser(repoDID)
			if err != nil {
				log.Error(ctx, "could not get replies", "error", err)
				return
			}
			spSeg, err := seg.ToStreamplaceSegment()
			if err != nil {
				log.Error(ctx, "could not convert segment to streamplace segment", "error", err)
				return
			}
			initialBurst <- spSeg
			if a.CLI.LivepeerGatewayURL != "" {
				renditions, err := renditions.GenerateRenditions(spSeg)
				if err != nil {
					log.Error(ctx, "could not generate renditions", "error", err)
					return
				}
				outRs := streamplace.Defs_Renditions{
					LexiconTypeID: "place.stream.defs#renditions",
				}
				outRs.Renditions = []*streamplace.Defs_Rendition{}
				for _, r := range renditions {
					outRs.Renditions = append(outRs.Renditions, &streamplace.Defs_Rendition{
						LexiconTypeID: "place.stream.defs#rendition",
						Name:          r.Name,
					})
				}
				initialBurst <- outRs
			}
		}()

		go func() {
			ls, err := a.Model.GetLatestLivestreamForRepo(repoDID)
			if err != nil {
				log.Error(ctx, "could not get latest livestream", "error", err)
				return
			}
			lsv, err := ls.ToLivestreamView()
			if err != nil {
				log.Error(ctx, "could not marshal livestream", "error", err)
				return
			}
			initialBurst <- lsv
		}()

		go func() {
			count := spmetrics.GetViewCount(repoDID)
			initialBurst <- streamplace.Livestream_ViewerCount{Count: int64(count), LexiconTypeID: "place.stream.livestream#viewerCount"}
		}()

		go func() {
			messages, err := a.Model.MostRecentChatMessages(repoDID)
			if err != nil {
				log.Error(ctx, "could not get chat messages", "error", err)
				return
			}
			for _, message := range messages {
				initialBurst <- message
			}
		}()

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				log.Error(ctx, "error reading message", "error", err)
				break
			}
			log.Log(ctx, "received message", "messageType", messageType, "message", string(message))
		}
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
