package atproto

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/xrpc"
	oauth "github.com/haileyok/atproto-oauth-golang"
	"github.com/haileyok/atproto-oauth-golang/helpers"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
)

func Login(ctx context.Context, cli *config.CLI, downstreamPAR *model.PAR, mod model.Model) (string, error) {
	meta := GetUpstreamMetadata("longos.iameli.link", "web", "")
	oclient, err := oauth.NewClient(oauth.ClientArgs{
		ClientJwk:   cli.JWK,
		ClientId:    meta.ClientID,
		RedirectUri: meta.RedirectURIs[0],
	})
	log.Log(ctx, "OAuth client information", "clientId", meta.ClientID, "redirectUri", meta.RedirectURIs[0])
	if err != nil {
		return "", fmt.Errorf("failed to create OAuth client: %w", err)
	}

	// If you already have a did or a URL, you can skip this step
	did, err := resolveHandle(ctx, downstreamPAR.LoginHint) // returns did:plc:abc123 or did:web:test.com
	if err != nil {
		return "", fmt.Errorf("failed to resolve handle '%s': %w", downstreamPAR.LoginHint, err)
	}

	// If you already have a URL, you can skip this step
	service, err := resolveService(ctx, did) // returns https://pds.haileyok.com
	if err != nil {
		return "", fmt.Errorf("failed to resolve service for DID '%s': %w", did, err)
	}

	authserver, err := oclient.ResolvePdsAuthServer(ctx, service)
	if err != nil {
		return "", fmt.Errorf("failed to resolve PDS auth server for service '%s': %w", service, err)
	}

	authmeta, err := oclient.FetchAuthServerMetadata(ctx, authserver)
	if err != nil {
		return "", fmt.Errorf("failed to fetch auth server metadata from '%s': %w", authserver, err)
	}

	k, err := helpers.GenerateKey(nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate DPoP key: %w", err)
	}

	// b, err := json.Marshal(k)
	// if err != nil {
	// 	return "", err
	// }

	parResp, err := oclient.SendParAuthRequest(ctx, authserver, authmeta, downstreamPAR.LoginHint, meta.Scope, k)
	if err != nil {
		return "", fmt.Errorf("failed to send PAR auth request to '%s': %w", authserver, err)
	}

	log.Log(ctx, "parResp", "parResp", parResp)

	jwkJSON, err := json.Marshal(k)
	if err != nil {
		return "", fmt.Errorf("failed to marshal DPoP key to JSON: %w", err)
	}

	u, err := url.Parse(authmeta.AuthorizationEndpoint)
	if err != nil {
		return "", fmt.Errorf("failed to parse auth server metadata: %w", err)
	}
	u.RawQuery = fmt.Sprintf("client_id=%s&request_uri=%s", url.QueryEscape(meta.ClientID), parResp.RequestUri)
	str := u.String()

	err = mod.CreateOAuthSession(&model.OAuthSession{
		UpstreamState:            parResp.State,
		RepoDID:                  did,
		PDSUrl:                   service,
		UpstreamAuthServerIssuer: authserver,
		UpstreamPKCEVerifier:     parResp.PkceVerifier,
		UpstreamDPoPNonce:        parResp.DpopAuthserverNonce,
		UpstreamDPoPPrivateJWK:   jwkJSON,
		DownstreamPARID:          downstreamPAR.ID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create OAuth session in database: %w", err)
	}

	return str, nil
}

var xrpcClient *oauth.XrpcClient

func GetXrpcClient(mod model.Model) *oauth.XrpcClient {
	if xrpcClient == nil {
		xrpcClient = &oauth.XrpcClient{
			OnDpopPdsNonceChanged: func(did, newNonce string) {
				// todo: update the nonce in the database... i guess we only have one session per user?
			},
		}
	}
	return xrpcClient
}

func HandleOauthReturn(ctx context.Context, cli *config.CLI, code string, iss string, state string, mod model.Model) (*model.OAuthSession, error) {
	meta := GetUpstreamMetadata("longos.iameli.link", "web", "")
	oclient, err := oauth.NewClient(oauth.ClientArgs{
		ClientJwk:   cli.JWK,
		ClientId:    meta.ClientID,
		RedirectUri: meta.RedirectURIs[0],
	})

	session, err := mod.GetOAuthSessionByUpstreamState(state)
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("no OAuth session found for state: %s", state)
	}

	if iss != session.UpstreamAuthServerIssuer {
		return nil, fmt.Errorf("issuer mismatch: %s != %s", iss, session.UpstreamAuthServerIssuer)
	}

	key, err := jwk.ParseKey(session.UpstreamDPoPPrivateJWK)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DPoP private JWK: %w", err)
	}

	itResp, err := oclient.InitialTokenRequest(ctx, code, iss, session.UpstreamPKCEVerifier, session.UpstreamDPoPNonce, key)
	if err != nil {
		return nil, fmt.Errorf("failed to request initial token: %w", err)
	}
	now := time.Now()

	if itResp.Sub != session.RepoDID {
		return nil, fmt.Errorf("sub mismatch: %s != %s", itResp.Sub, session.RepoDID)
	}

	if itResp.Scope != meta.Scope {
		return nil, fmt.Errorf("scope mismatch: %s != %s", itResp.Scope, meta.Scope)
	}

	expiry := now.Add(time.Second * time.Duration(itResp.ExpiresIn)).UTC()
	session.UpstreamAccessToken = itResp.AccessToken
	session.UpstreamAccessTokenExp = expiry
	session.UpstreamRefreshToken = itResp.RefreshToken

	log.Log(ctx, "itResp", "itResp", itResp)

	authArgs := &oauth.XrpcAuthedRequestArgs{
		Did:            session.RepoDID,
		AccessToken:    session.UpstreamAccessToken,
		PdsUrl:         session.PDSUrl,
		Issuer:         session.UpstreamAuthServerIssuer,
		DpopPdsNonce:   session.UpstreamDPoPNonce,
		DpopPrivateJwk: key,
	}

	xc := GetXrpcClient(mod)

	// brief check to make sure we can actually do stuff
	var out atproto.ServerCheckAccountStatus_Output
	if err := xc.Do(ctx, authArgs, xrpc.Query, "application/json", "com.atproto.server.checkAccountStatus", nil, nil, &out); err != nil {
		return nil, fmt.Errorf("failed to check account status: %w", err)
	}

	err = mod.UpdateOAuthSession(session)
	if err != nil {
		return nil, fmt.Errorf("failed to update OAuth session: %w", err)
	}

	return session, nil
}
