package oproxy

import "github.com/labstack/echo/v4"

type OProxy struct {
	createOAuthSession func(id string, session *OAuthSession) error
	updateOAuthSession func(id string, session *OAuthSession) error
	loadOAuthSession   func(id string) (*OAuthSession, error)
	e                  *echo.Echo
	host               string
	scope              string
}

type Config struct {
	CreateOAuthSession func(id string, session *OAuthSession) error
	UpdateOAuthSession func(id string, session *OAuthSession) error
	LoadOAuthSession   func(id string) (*OAuthSession, error)
	Host               string
	Scope              string
}

func New(conf *Config) *OProxy {
	e := echo.New()
	return &OProxy{
		createOAuthSession: conf.CreateOAuthSession,
		updateOAuthSession: conf.UpdateOAuthSession,
		loadOAuthSession:   conf.LoadOAuthSession,
		e:                  e,
		host:               conf.Host,
		scope:              conf.Scope,
	}
}
