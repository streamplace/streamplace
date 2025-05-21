package oproxy

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwk"
)

type OProxy struct {
	createOAuthSession  func(id string, session *OAuthSession) error
	updateOAuthSession  func(id string, session *OAuthSession) error
	userGetOAuthSession func(id string) (*OAuthSession, error)
	Echo                *echo.Echo
	host                string
	scope               string
	upstreamJWK         jwk.Key
	downstreamJWK       jwk.Key
	slog                *slog.Logger
	clientMetadata      *OAuthClientMetadata
}

type Config struct {
	CreateOAuthSession func(id string, session *OAuthSession) error
	UpdateOAuthSession func(id string, session *OAuthSession) error
	GetOAuthSession    func(id string) (*OAuthSession, error)
	Host               string
	Scope              string
	UpstreamJWK        jwk.Key
	DownstreamJWK      jwk.Key
	Slog               *slog.Logger
	ClientMetadata     *OAuthClientMetadata
}

func New(conf *Config) *OProxy {
	e := echo.New()
	mySlog := conf.Slog
	if mySlog == nil {
		mySlog = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	o := &OProxy{
		createOAuthSession:  conf.CreateOAuthSession,
		updateOAuthSession:  conf.UpdateOAuthSession,
		userGetOAuthSession: conf.GetOAuthSession,
		Echo:                e,
		host:                conf.Host,
		scope:               conf.Scope,
		upstreamJWK:         conf.UpstreamJWK,
		downstreamJWK:       conf.DownstreamJWK,
		slog:                mySlog,
		clientMetadata:      conf.ClientMetadata,
	}
	o.Echo.GET("/.well-known/oauth-authorization-server", o.HandleOAuthAuthorizationServer)
	o.Echo.GET("/.well-known/oauth-protected-resource", o.HandleOAuthProtectedResource)
	o.Echo.GET("/xrpc/com.atproto.identity.resolveHandle", HandleComAtprotoIdentityResolveHandle)
	o.Echo.POST("/oauth/par", o.HandleOAuthPAR)
	o.Echo.GET("/oauth/authorize", o.HandleOAuthAuthorize)
	o.Echo.GET("/oauth/return", o.HandleOAuthReturn)
	o.Echo.POST("/oauth/token", o.DPoPNonceMiddleware(o.HandleOAuthToken))
	o.Echo.POST("/oauth/revoke", o.DPoPNonceMiddleware(o.HandleOAuthRevoke))
	o.Echo.GET("/oauth/upstream/client-metadata.json", o.HandleClientMetadataUpstream)
	o.Echo.GET("/oauth/upstream/jwks.json", o.HandleJwksUpstream)
	o.Echo.GET("/oauth/downstream/client-metadata.json", o.HandleClientMetadataDownstream)
	o.Echo.Any("/xrpc/*", o.OAuthMiddleware(o.HandleWildcard))
	o.Echo.Use(o.ErrorHandlingMiddleware)
	return o
}

func (o *OProxy) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // todo: ehhhhhhhhhhhh
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,DPoP")
		w.Header().Set("Access-Control-Allow-Methods", "*")
		w.Header().Set("Access-Control-Expose-Headers", "DPoP-Nonce")
		o.Echo.ServeHTTP(w, r)
	})
}
