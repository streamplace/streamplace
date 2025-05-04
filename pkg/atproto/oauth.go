package atproto

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/xrpc"
	oauth "github.com/haileyok/atproto-oauth-golang"
	"github.com/haileyok/atproto-oauth-golang/helpers"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/streamplace"
)

func Login(ctx context.Context, cli *config.CLI, input *streamplace.AccountLogin_Input, mod model.Model) (*streamplace.AccountDefs_LoginResponse, error) {
	meta := GetMetadata("longos.iameli.link", "web", "")
	oclient, err := oauth.NewClient(oauth.ClientArgs{
		ClientJwk:   cli.JWK,
		ClientId:    meta.ClientID,
		RedirectUri: meta.RedirectURIs[0],
	})
	log.Log(ctx, "OAuth client information", "clientId", meta.ClientID, "redirectUri", meta.RedirectURIs[0])
	if err != nil {
		return nil, fmt.Errorf("failed to create OAuth client: %w", err)
	}

	// If you already have a did or a URL, you can skip this step
	did, err := resolveHandle(ctx, input.HandleOrDID) // returns did:plc:abc123 or did:web:test.com
	if err != nil {
		return nil, fmt.Errorf("failed to resolve handle '%s': %w", input.HandleOrDID, err)
	}

	// If you already have a URL, you can skip this step
	service, err := resolveService(ctx, did) // returns https://pds.haileyok.com
	if err != nil {
		return nil, fmt.Errorf("failed to resolve service for DID '%s': %w", did, err)
	}

	authserver, err := oclient.ResolvePdsAuthServer(ctx, service)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve PDS auth server for service '%s': %w", service, err)
	}

	authmeta, err := oclient.FetchAuthServerMetadata(ctx, authserver)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch auth server metadata from '%s': %w", authserver, err)
	}

	k, err := helpers.GenerateKey(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate DPoP key: %w", err)
	}

	// b, err := json.Marshal(k)
	// if err != nil {
	// 	return nil, err
	// }

	parResp, err := oclient.SendParAuthRequest(ctx, authserver, authmeta, input.HandleOrDID, meta.Scope, k)
	if err != nil {
		return nil, fmt.Errorf("failed to send PAR auth request to '%s': %w", authserver, err)
	}

	log.Log(ctx, "parResp", "parResp", parResp)

	jwkJSON, err := json.Marshal(k)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal DPoP key to JSON: %w", err)
	}

	u, err := url.Parse(authmeta.AuthorizationEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse auth server metadata: %w", err)
	}
	u.RawQuery = fmt.Sprintf("client_id=%s&request_uri=%s", url.QueryEscape(meta.ClientID), parResp.RequestUri)
	str := u.String()

	err = mod.CreateOAuthSession(&model.OAuthSession{
		State:            parResp.State,
		RepoDID:          did,
		PDSUrl:           service,
		AuthServerIssuer: authserver,
		PKCEVerifier:     parResp.PkceVerifier,
		DPoPNonce:        parResp.DpopAuthserverNonce,
		DPoPPrivateJWK:   jwkJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create OAuth session in database: %w", err)
	}

	return &streamplace.AccountDefs_LoginResponse{
		RedirectUrl: str,
	}, nil
}

var xrpcClient *oauth.XrpcClient

func getXrpcClient(mod model.Model) *oauth.XrpcClient {
	if xrpcClient == nil {
		xrpcClient = &oauth.XrpcClient{
			OnDpopPdsNonceChanged: func(did, newNonce string) {
				// todo: update the nonce in the database... i guess we only have one session per user?
			},
		}
	}
	return xrpcClient
}

func HandleOauthReturn(ctx context.Context, cli *config.CLI, code string, iss string, state string, mod model.Model) error {
	meta := GetMetadata("longos.iameli.link", "web", "")
	oclient, err := oauth.NewClient(oauth.ClientArgs{
		ClientJwk:   cli.JWK,
		ClientId:    meta.ClientID,
		RedirectUri: meta.RedirectURIs[0],
	})

	session, err := mod.GetOAuthSessionByState(state)
	if err != nil {
		return fmt.Errorf("failed to get OAuth session: %w", err)
	}
	if session == nil {
		return fmt.Errorf("no OAuth session found for state: %s", state)
	}

	if iss != session.AuthServerIssuer {
		return fmt.Errorf("issuer mismatch: %s != %s", iss, session.AuthServerIssuer)
	}

	key, err := jwk.ParseKey(session.DPoPPrivateJWK)
	if err != nil {
		return fmt.Errorf("failed to parse DPoP private JWK: %w", err)
	}

	itResp, err := oclient.InitialTokenRequest(ctx, code, iss, session.PKCEVerifier, session.DPoPNonce, key)
	if err != nil {
		return fmt.Errorf("failed to request initial token: %w", err)
	}
	now := time.Now()

	if itResp.Sub != session.RepoDID {
		return fmt.Errorf("sub mismatch: %s != %s", itResp.Sub, session.RepoDID)
	}

	if itResp.Scope != meta.Scope {
		return fmt.Errorf("scope mismatch: %s != %s", itResp.Scope, meta.Scope)
	}

	expiry := now.Add(time.Second * time.Duration(itResp.ExpiresIn)).UTC()
	session.AccessToken = itResp.AccessToken
	session.AccessTokenExp = expiry
	session.RefreshToken = itResp.RefreshToken
	err = mod.UpdateOAuthSession(session)
	if err != nil {
		return fmt.Errorf("failed to update OAuth session: %w", err)
	}

	log.Log(ctx, "itResp", "itResp", itResp)

	authArgs := &oauth.XrpcAuthedRequestArgs{
		Did:            session.RepoDID,
		AccessToken:    session.AccessToken,
		PdsUrl:         session.PDSUrl,
		Issuer:         session.AuthServerIssuer,
		DpopPdsNonce:   session.DPoPNonce,
		DpopPrivateJwk: key,
	}

	post := bsky.FeedPost{
		Text:      "hello from atproto golang oauth client",
		CreatedAt: syntax.DatetimeNow().String(),
	}

	input := atproto.RepoCreateRecord_Input{
		Collection: "app.bsky.feed.post",
		Repo:       authArgs.Did,
		Record:     &util.LexiconTypeDecoder{Val: &post},
	}

	xc := getXrpcClient(mod)

	var out atproto.RepoCreateRecord_Output
	if err := xc.Do(ctx, authArgs, xrpc.Procedure, "application/json", "com.atproto.repo.createRecord", nil, input, &out); err != nil {
		return err
	}

	log.Log(ctx, "out", "out", out)

	return nil
}
