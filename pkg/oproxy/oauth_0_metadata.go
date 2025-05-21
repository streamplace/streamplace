package oproxy

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/haileyok/atproto-oauth-golang/helpers"
	"github.com/labstack/echo/v4"
)

func (o *OProxy) HandleOAuthAuthorizationServer(c echo.Context) error {
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")
	c.Response().Header().Set("Content-Type", "application/json")
	c.Response().WriteHeader(200)
	json.NewEncoder(c.Response().Writer).Encode(generateOAuthServerMetadata(o.host))
	return nil
}

func (o *OProxy) HandleOAuthProtectedResource(c echo.Context) error {
	return c.JSON(200, map[string]interface{}{
		"resource": fmt.Sprintf("https://%s", o.host),
		"authorization_servers": []string{
			fmt.Sprintf("https://%s", o.host),
		},
		"scopes_supported": []string{},
		"bearer_methods_supported": []string{
			"header",
		},
		"resource_documentation": "https://atproto.com",
	})
}

func (o *OProxy) HandleClientMetadataUpstream(c echo.Context) error {
	meta := o.GetUpstreamMetadata()
	return c.JSON(200, meta)
}

func (o *OProxy) HandleJwksUpstream(c echo.Context) error {
	pubKey, err := o.upstreamJWK.PublicKey()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not get public key")
	}
	return c.JSON(200, helpers.CreateJwksResponseObject(pubKey))
}

func (o *OProxy) HandleClientMetadataDownstream(c echo.Context) error {
	redirectURI := c.QueryParam("redirect_uri")
	meta, err := o.GetDownstreamMetadata(redirectURI)
	if err != nil {
		return err
	}
	return c.JSON(200, meta)
}

func (o *OProxy) GetUpstreamMetadata() *OAuthClientMetadata {
	meta := *o.clientMetadata
	meta.ClientID = fmt.Sprintf("https://%s/oauth/upstream/client-metadata.json", o.host)
	meta.JwksURI = fmt.Sprintf("https://%s/oauth/upstream/jwks.json", o.host)
	meta.ClientURI = fmt.Sprintf("https://%s", o.host)
	meta.TokenEndpointAuthMethod = "private_key_jwt"
	meta.ResponseTypes = []string{"code"}
	meta.GrantTypes = []string{"authorization_code", "refresh_token"}
	meta.DPoPBoundAccessTokens = boolPtr(true)
	meta.TokenEndpointAuthSigningAlg = "ES256"
	meta.RedirectURIs = []string{fmt.Sprintf("https://%s/oauth/return", o.host)}
	return &meta
}

func generateOAuthServerMetadata(host string) map[string]any {
	oauthServerMetadata := map[string]any{
		"issuer":                                         fmt.Sprintf("https://%s", host),
		"request_parameter_supported":                    true,
		"request_uri_parameter_supported":                true,
		"require_request_uri_registration":               true,
		"scopes_supported":                               []string{"atproto", "transition:generic", "transition:chat.bsky"},
		"subject_types_supported":                        []string{"public"},
		"response_types_supported":                       []string{"code"},
		"response_modes_supported":                       []string{"query", "fragment", "form_post"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":               []string{"S256"},
		"ui_locales_supported":                           []string{"en-US"},
		"display_values_supported":                       []string{"page", "popup", "touch"},
		"authorization_response_iss_parameter_supported": true,
		"request_object_encryption_alg_values_supported": []string{},
		"request_object_encryption_enc_values_supported": []string{},
		"jwks_uri":                              fmt.Sprintf("https://%s/oauth/jwks", host),
		"authorization_endpoint":                fmt.Sprintf("https://%s/oauth/authorize", host),
		"token_endpoint":                        fmt.Sprintf("https://%s/oauth/token", host),
		"token_endpoint_auth_methods_supported": []string{"none", "private_key_jwt"},
		"revocation_endpoint":                   fmt.Sprintf("https://%s/oauth/revoke", host),
		"introspection_endpoint":                fmt.Sprintf("https://%s/oauth/introspect", host),
		"pushed_authorization_request_endpoint": fmt.Sprintf("https://%s/oauth/par", host),
		"require_pushed_authorization_requests": true,
		"client_id_metadata_document_supported": true,
		"request_object_signing_alg_values_supported": []string{
			"RS256", "RS384", "RS512", "PS256", "PS384", "PS512",
			"ES256", "ES256K", "ES384", "ES512", "none",
		},
		"token_endpoint_auth_signing_alg_values_supported": []string{
			"RS256", "RS384", "RS512", "PS256", "PS384", "PS512",
			"ES256", "ES256K", "ES384", "ES512",
		},
		"dpop_signing_alg_values_supported": []string{
			"RS256", "RS384", "RS512", "PS256", "PS384", "PS512",
			"ES256", "ES256K", "ES384", "ES512",
		},
	}
	return oauthServerMetadata
}

func (o *OProxy) GetDownstreamMetadata(redirectURI string) (*OAuthClientMetadata, error) {
	meta := *o.clientMetadata
	meta.ClientID = fmt.Sprintf("https://%s/oauth/downstream/client-metadata.json", o.host)
	meta.ClientURI = fmt.Sprintf("https://%s", o.host)
	meta.TokenEndpointAuthMethod = "none"
	meta.ResponseTypes = []string{"code"}
	meta.GrantTypes = []string{"authorization_code", "refresh_token"}
	meta.DPoPBoundAccessTokens = boolPtr(true)
	meta.ApplicationType = "web"
	if redirectURI != "" {
		found := false
		for _, uri := range meta.RedirectURIs {
			if uri == redirectURI {
				found = true
				break
			}
		}
		if !found {
			return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid redirect_uri: %s not in allowed URIs", redirectURI))
		}
		meta.RedirectURIs = []string{redirectURI}
	}
	return &meta, nil
}
