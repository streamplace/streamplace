package oproxy

import (
	"context"
	"net/http"

	"github.com/bluesky-social/indigo/xrpc"
	oauth "github.com/haileyok/atproto-oauth-golang"
	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
)

var xrpcClient *oauth.XrpcClient

type XrpcClient struct {
	client   *oauth.XrpcClient
	authArgs *oauth.XrpcAuthedRequestArgs
}

func GetXrpcClient(ctx context.Context, mod model.Model, session *model.OAuthSession) *XrpcClient {
	key, err := jwk.ParseKey(session.UpstreamDPoPPrivateJWK)
	if err != nil {
		log.Error(ctx, "failed to parse DPoP private JWK", "error", err)
		panic(err)
	}
	authArgs := &oauth.XrpcAuthedRequestArgs{
		Did:            session.RepoDID,
		AccessToken:    session.UpstreamAccessToken,
		PdsUrl:         session.PDSUrl,
		Issuer:         session.UpstreamAuthServerIssuer,
		DpopPdsNonce:   session.UpstreamDPoPNonce,
		DpopPrivateJwk: key,
	}

	xrpcClient := &oauth.XrpcClient{
		OnDpopPdsNonceChanged: func(did, newNonce string) {
			sess, err := mod.GetOAuthSession(session.ID)
			if err != nil {
				log.Error(ctx, "failed to get OAuth session", "error", err)
				return
			}
			sess.UpstreamDPoPNonce = newNonce
			err = mod.UpdateOAuthSession(sess)
			if err != nil {
				log.Error(ctx, "failed to update OAuth session", "error", err)
			}
		},
	}
	return &XrpcClient{client: xrpcClient, authArgs: authArgs}
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
	return echo.NewHTTPError(xErr.StatusCode, xErr.Error())
}
