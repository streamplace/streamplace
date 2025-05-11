package oproxy

import "github.com/labstack/echo/v4"

type OProxy struct {
	saveOAuthSession func(id string, session *OAuthSession) error
	loadOAuthSession func(id string) (*OAuthSession, error)
	e                *echo.Echo
}

type OProxyConfig struct {
	SaveOAuthSession func(id string, session *OAuthSession) error
	LoadOAuthSession func(id string) (*OAuthSession, error)
}

func NewOProxy(conf *OProxyConfig) *OProxy {
	e := echo.New()
	return &OProxy{
		saveOAuthSession: conf.SaveOAuthSession,
		loadOAuthSession: conf.LoadOAuthSession,
		e:                e,
	}
}
