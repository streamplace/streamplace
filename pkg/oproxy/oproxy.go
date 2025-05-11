package oproxy

import (
	"log/slog"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwk"
)

type OProxy struct {
	createOAuthSession func(id string, session *OAuthSession) error
	updateOAuthSession func(id string, session *OAuthSession) error
	loadOAuthSession   func(id string) (*OAuthSession, error)
	e                  *echo.Echo
	host               string
	scope              string
	upstreamJWK        jwk.Key
	downstreamJWK      jwk.Key
	slog               *slog.Logger
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
	return &OProxy{
		createOAuthSession: conf.CreateOAuthSession,
		updateOAuthSession: conf.UpdateOAuthSession,
		loadOAuthSession:   conf.LoadOAuthSession,
		e:                  e,
		host:               conf.Host,
		scope:              conf.Scope,
		upstreamJWK:        conf.UpstreamJWK,
		downstreamJWK:      conf.DownstreamJWK,
	}
}
