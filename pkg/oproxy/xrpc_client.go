package oproxy

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bluesky-social/indigo/xrpc"
	oauth "github.com/haileyok/atproto-oauth-golang"
	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwk"
)

var xrpcClient *oauth.XrpcClient

type XrpcClient struct {
	client   *oauth.XrpcClient
	authArgs *oauth.XrpcAuthedRequestArgs
}

func (o *OProxy) GetXrpcClient(session *OAuthSession) (*XrpcClient, error) {
	key, err := jwk.ParseKey([]byte(session.UpstreamDPoPPrivateJWK))
	if err != nil {
		return nil, fmt.Errorf("failed to parse DPoP private JWK: %w", err)
	}
	authArgs := &oauth.XrpcAuthedRequestArgs{
		Did:            session.DID,
		AccessToken:    session.UpstreamAccessToken,
		PdsUrl:         session.PDSUrl,
		Issuer:         session.UpstreamAuthServerIssuer,
		DpopPdsNonce:   session.UpstreamDPoPNonce,
		DpopPrivateJwk: key,
	}

	xrpcClient := &oauth.XrpcClient{
		OnDpopPdsNonceChanged: func(did, newNonce string) {
			sess, err := o.getOAuthSession(session.DownstreamDPoPJKT)
			if err != nil {
				o.slog.Error("failed to get OAuth session in OnDpopPdsNonceChanged", "error", err)
				return
			}
			sess.UpstreamDPoPNonce = newNonce
			err = o.updateOAuthSession(session.DownstreamDPoPJKT, sess)
			if err != nil {
				o.slog.Error("failed to update OAuth session in OnDpopPdsNonceChanged", "error", err)
			}
			o.slog.Info("updated OAuth session in OnDpopPdsNonceChanged", "session", sess)
		},
	}
	return &XrpcClient{client: xrpcClient, authArgs: authArgs}, nil
}

func (c *XrpcClient) Do(ctx context.Context, kind xrpc.XRPCRequestType, inpenc, method string, params map[string]any, bodyobj any, out any) error {
	err := c.client.Do(ctx, c.authArgs, kind, inpenc, method, params, bodyobj, out)
	if err == nil {
		return nil
	}
	xErr, ok := err.(*xrpc.Error)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return xErr
}
