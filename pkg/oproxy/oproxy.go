package oproxy

type OProxy struct {
	SaveOAuthSession func(id string, session *OAuthSession) error
	LoadOAuthSession func(id string) (*OAuthSession, error)
}
