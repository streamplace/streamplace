package oproxy

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwk"
)

type OProxy struct {
	createOAuthSession   func(id string, session *OAuthSession) error
	updateOAuthSession   func(id string, session *OAuthSession) error
	userLoadOAuthSession func(id string) (*OAuthSession, error)
	e                    *echo.Echo
	host                 string
	scope                string
	upstreamJWK          jwk.Key
	downstreamJWK        jwk.Key
	slog                 *slog.Logger
}

type Config struct {
	CreateOAuthSession func(id string, session *OAuthSession) error
	UpdateOAuthSession func(id string, session *OAuthSession) error
	LoadOAuthSession   func(id string) (*OAuthSession, error)
	Host               string
	Scope              string
	UpstreamJWK        jwk.Key
	DownstreamJWK      jwk.Key
	Slog               *slog.Logger
}

func New(conf *Config) *OProxy {
	e := echo.New()
	mySlog := conf.Slog
	if mySlog == nil {
		mySlog = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	o := &OProxy{
		createOAuthSession:   conf.CreateOAuthSession,
		updateOAuthSession:   conf.UpdateOAuthSession,
		userLoadOAuthSession: conf.LoadOAuthSession,
		e:                    e,
		host:                 conf.Host,
		scope:                conf.Scope,
		upstreamJWK:          conf.UpstreamJWK,
		downstreamJWK:        conf.DownstreamJWK,
		slog:                 mySlog,
	}
	o.e.GET("/.well-known/oauth-authorization-server", o.HandleOAuthAuthorizationServer)
	o.e.GET("/.well-known/oauth-protected-resource", o.HandleOAuthProtectedResource)
	o.e.POST("/oauth/par", o.HandleOAuthPAR)
	o.e.GET("/oauth/authorize", o.HandleOAuthAuthorize)
	o.e.GET("/oauth/return", o.HandleOAuthReturn)
	o.e.POST("/oauth/token", o.DPoPNonceMiddleware(o.HandleOAuthToken))
	o.e.POST("/oauth/revoke", o.DPoPNonceMiddleware(o.HandleOAuthRevoke))
	o.e.GET("/oauth/upstream/client-metadata.json", o.HandleClientMetadataUpstream)
	o.e.GET("/oauth/upstream/jwks.json", o.HandleJwksUpstream)
	o.e.GET("/oauth/downstream/client-metadata.json", o.HandleClientMetadataDownstream)
	o.e.Use(o.ErrorHandlingMiddleware)
	return o
}

func (o *OProxy) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // todo: ehhhhhhhhhhhh
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,DPoP")
		w.Header().Set("Access-Control-Allow-Methods", "*")
		w.Header().Set("Access-Control-Expose-Headers", "DPoP-Nonce")
		o.e.ServeHTTP(w, r)
	})
}
