package atproto

import (
	"context"
	"fmt"
	"net/url"

	oauth "github.com/haileyok/atproto-oauth-golang"
	"github.com/haileyok/atproto-oauth-golang/helpers"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
)

func Login(ctx context.Context, cli *config.CLI) (*string, error) {
	meta := GetMetadata("longos.iameli.link", "web", "")
	oclient, err := oauth.NewClient(oauth.ClientArgs{
		ClientJwk:   cli.JWK,
		ClientId:    meta.ClientID,
		RedirectUri: meta.RedirectURIs[0],
	})
	log.Log(ctx, "OAuth client information", "clientId", meta.ClientID, "redirectUri", meta.RedirectURIs[0])
	if err != nil {
		return nil, err
	}
	userInput := "scumb.ag"

	// If you already have a did or a URL, you can skip this step
	// did, err := resolveHandle(ctx, userInput) // returns did:plc:abc123 or did:web:test.com
	// if err != nil {
	// 	return nil, err
	// }
	// did := "did:plc:dkh4rwafdcda4ko7lewe43ml"

	// If you already have a URL, you can skip this step
	// service, err := resolveService(ctx, did) // returns https://pds.haileyok.com
	// if err != nil {
	// 	return nil, err
	// }

	service := "https://milkcap.us-west.host.bsky.network"

	authserver, err := oclient.ResolvePdsAuthServer(ctx, service)
	if err != nil {
		return nil, err
	}

	authmeta, err := oclient.FetchAuthServerMetadata(ctx, authserver)
	if err != nil {
		return nil, err
	}

	k, err := helpers.GenerateKey(nil)
	if err != nil {
		return nil, err
	}

	// b, err := json.Marshal(k)
	// if err != nil {
	// 	return nil, err
	// }

	parResp, err := oclient.SendParAuthRequest(ctx, authserver, authmeta, userInput, meta.Scope, k)
	if err != nil {
		return nil, err
	}

	log.Log(ctx, "parResp", "parResp", parResp)

	u, _ := url.Parse(authmeta.AuthorizationEndpoint)
	u.RawQuery = fmt.Sprintf("client_id=%s&request_uri=%s", url.QueryEscape(meta.ClientID), parResp.RequestUri)
	str := u.String()

	// https://longos.iameli.link/login?state=30866d14920b46e7642a&iss=https%3A%2F%2Fbsky.social&code=cod-92a9a96a7619dbb91857ec25e33c6a58a3d39c2469acbe2586a1ddf5c8edce0d

	return &str, nil
}
