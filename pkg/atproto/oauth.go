package atproto

import (
	"context"
	"fmt"
	"net/url"

	oauth "github.com/haileyok/atproto-oauth-golang"
	"github.com/haileyok/atproto-oauth-golang/helpers"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/streamplace"
)

func Login(ctx context.Context, cli *config.CLI, input *streamplace.AccountLogin_Input) (*streamplace.AccountDefs_LoginResponse, error) {
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

	// If you already have a did or a URL, you can skip this step
	did, err := resolveHandle(ctx, input.HandleOrDID) // returns did:plc:abc123 or did:web:test.com
	if err != nil {
		return nil, err
	}

	// If you already have a URL, you can skip this step
	service, err := resolveService(ctx, did) // returns https://pds.haileyok.com
	if err != nil {
		return nil, err
	}

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

	parResp, err := oclient.SendParAuthRequest(ctx, authserver, authmeta, input.HandleOrDID, meta.Scope, k)
	if err != nil {
		return nil, err
	}

	log.Log(ctx, "parResp", "parResp", parResp)

	u, _ := url.Parse(authmeta.AuthorizationEndpoint)
	u.RawQuery = fmt.Sprintf("client_id=%s&request_uri=%s", url.QueryEscape(meta.ClientID), parResp.RequestUri)
	str := u.String()

	return &streamplace.AccountDefs_LoginResponse{
		RedirectUrl: str,
	}, nil
}
