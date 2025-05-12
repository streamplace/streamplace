package oproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/xrpc"
	oauth "github.com/haileyok/atproto-oauth-golang"
	"github.com/haileyok/atproto-oauth-golang/helpers"
	"github.com/labstack/echo/v4"
	"github.com/lestrrat-go/jwx/v2/jwk"
)

// downstream --> upstream transition; attempt to send user to the upstream auth server
func (o *OProxy) Authorize(ctx context.Context, requestURI, clientID string) (string, error) {
	downstreamMeta := o.GetDownstreamMetadata()
	if downstreamMeta.ClientID != clientID {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("client ID mismatch: %s != %s", downstreamMeta.ClientID, clientID))
	}

	jkt, _, err := parseURN(requestURI)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to parse URN: %s", err))
	}

	session, err := o.loadOAuthSession(jkt)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to load OAuth session jkt=%s: %s", jkt, err))
	}

	if session == nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("no session found for jkt=%s", jkt))
	}

	if session.Status() != OAuthSessionStatePARCreated {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("session is not in par-created state: %s", session.Status()))
	}

	if session.DownstreamPARRequestURI != requestURI {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("request URI mismatch: %s != %s", session.DownstreamPARRequestURI, requestURI))
	}

	now := time.Now()
	session.DownstreamPARUsedAt = &now
	err = o.updateOAuthSession(jkt, session)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to update OAuth session: %s", err))
	}

	upstreamMeta := o.GetUpstreamMetadata()
	oclient, err := oauth.NewClient(oauth.ClientArgs{
		ClientJwk:   o.upstreamJWK,
		ClientId:    upstreamMeta.ClientID,
		RedirectUri: upstreamMeta.RedirectURIs[0],
	})
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to create OAuth client: %s", err))
	}

	did, err := resolveHandle(ctx, session.DID)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to resolve handle '%s': %s", session.DID, err))
	}

	service, err := resolveService(ctx, did)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to resolve service for DID '%s': %s", did, err))
	}

	authserver, err := oclient.ResolvePdsAuthServer(ctx, service)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to resolve PDS auth server for service '%s': %s", service, err))
	}

	authmeta, err := oclient.FetchAuthServerMetadata(ctx, authserver)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to fetch auth server metadata from '%s': %s", authserver, err))
	}

	k, err := helpers.GenerateKey(nil)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to generate DPoP key: %s", err))
	}

	state := makeState(jkt)

	parResp, err := oclient.SendParAuthRequest(ctx, authserver, authmeta, did, upstreamMeta.Scope, k, state)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to send PAR auth request to '%s': %s", authserver, err))
	}

	jwkJSON, err := json.Marshal(k)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to marshal DPoP key to JSON: %s", err))
	}

	u, err := url.Parse(authmeta.AuthorizationEndpoint)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to parse auth server metadata: %s", err))
	}
	u.RawQuery = fmt.Sprintf("client_id=%s&request_uri=%s", url.QueryEscape(upstreamMeta.ClientID), parResp.RequestUri)
	str := u.String()

	session.DID = did
	session.PDSUrl = service
	session.UpstreamState = parResp.State
	session.UpstreamAuthServerIssuer = authserver
	session.UpstreamPKCEVerifier = parResp.PkceVerifier
	session.UpstreamDPoPNonce = parResp.DpopAuthserverNonce
	session.UpstreamDPoPPrivateJWK = string(jwkJSON)

	err = o.updateOAuthSession(jkt, session)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to update OAuth session: %s", err))
	}

	return str, nil
}

func (o *OProxy) Return(ctx context.Context, code string, iss string, state string) (string, error) {
	upstreamMeta := o.GetUpstreamMetadata()
	oclient, err := oauth.NewClient(oauth.ClientArgs{
		ClientJwk:   o.upstreamJWK,
		ClientId:    upstreamMeta.ClientID,
		RedirectUri: upstreamMeta.RedirectURIs[0],
	})

	jkt, _, err := parseState(state)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to parse state: %s", err))
	}

	session, err := o.loadOAuthSession(jkt)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to get OAuth session: %s", err))
	}
	if session == nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("no OAuth session found for state: %s", state))
	}

	if session.Status() != OAuthSessionStateUpstream {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("session is not in upstream state: %s", session.Status()))
	}

	if session.UpstreamState != state {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("state mismatch: %s != %s", session.UpstreamState, state))
	}

	if iss != session.UpstreamAuthServerIssuer {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("issuer mismatch: %s != %s", iss, session.UpstreamAuthServerIssuer))
	}

	key, err := jwk.ParseKey([]byte(session.UpstreamDPoPPrivateJWK))
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to parse DPoP private JWK: %s", err))
	}

	itResp, err := oclient.InitialTokenRequest(ctx, code, iss, session.UpstreamPKCEVerifier, session.UpstreamDPoPNonce, key)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to request initial token: %s", err))
	}
	now := time.Now()

	if itResp.Sub != session.DID {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("sub mismatch: %s != %s", itResp.Sub, session.DID))
	}

	if itResp.Scope != upstreamMeta.Scope {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("scope mismatch: %s != %s", itResp.Scope, upstreamMeta.Scope))
	}

	downstreamCode, err := generateAuthorizationCode()
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to generate downstream code: %s", err))
	}

	expiry := now.Add(time.Second * time.Duration(itResp.ExpiresIn)).UTC()
	session.UpstreamAccessToken = itResp.AccessToken
	session.UpstreamAccessTokenExp = &expiry
	session.UpstreamRefreshToken = itResp.RefreshToken
	session.DownstreamAuthorizationCode = downstreamCode

	authArgs := &oauth.XrpcAuthedRequestArgs{
		Did:            session.DID,
		AccessToken:    session.UpstreamAccessToken,
		PdsUrl:         session.PDSUrl,
		Issuer:         session.UpstreamAuthServerIssuer,
		DpopPdsNonce:   session.UpstreamDPoPNonce,
		DpopPrivateJwk: key,
	}

	xrpcClient := &oauth.XrpcClient{
		OnDpopPdsNonceChanged: func(did, newNonce string) {},
	}

	// brief check to make sure we can actually do stuff
	var out atproto.ServerCheckAccountStatus_Output
	if err := xrpcClient.Do(ctx, authArgs, xrpc.Query, "application/json", "com.atproto.server.checkAccountStatus", nil, nil, &out); err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to check account status: %s", err))
	}

	err = o.updateOAuthSession(session.DownstreamDPoPJKT, session)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to update OAuth session: %s", err))
	}

	downstreamMeta := o.GetDownstreamMetadata()
	u, err := url.Parse(downstreamMeta.RedirectURIs[0])
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to parse downstream redirect URI: %s", err))
	}
	q := u.Query()
	q.Set("iss", fmt.Sprintf("https://%s", o.host))
	q.Set("state", session.DownstreamState)
	q.Set("code", session.DownstreamAuthorizationCode)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func (o *OProxy) GetUpstreamMetadata() *OAuthClientMetadata {
	// publicKey, err := o.upstreamJWK.PublicKey()
	// if err != nil {
	// 	panic(err)
	// }
	// jwks := jwk.NewSet()
	// err = jwks.AddKey(publicKey)
	// if err != nil {
	// 	panic(err)
	// }
	// ro := helpers.CreateJwksResponseObject(publicKey)
	meta := &OAuthClientMetadata{
		ClientID:  fmt.Sprintf("https://%s/oauth/upstream/client-metadata.json", o.host),
		JwksURI:   fmt.Sprintf("https://%s/oauth/upstream/jwks.json", o.host),
		ClientURI: fmt.Sprintf("https://%s", o.host),
		// RedirectURIs:            []string{fmt.Sprintf("https://%s/login", host)},
		Scope:                       "atproto transition:generic",
		TokenEndpointAuthMethod:     "private_key_jwt",
		ClientName:                  "Streamplace",
		ResponseTypes:               []string{"code"},
		GrantTypes:                  []string{"authorization_code", "refresh_token"},
		DPoPBoundAccessTokens:       boolPtr(true),
		TokenEndpointAuthSigningAlg: "ES256",
		RedirectURIs:                []string{fmt.Sprintf("https://%s/oauth/return", o.host)},
		// Jwks:                        ro,
	}
	return meta
}
