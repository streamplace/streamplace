package atproto

import (
	"fmt"
	"slices"
	"strings"
)

var AllowedPlatforms = []string{"ios", "android", "web"}

type OAuthClientMetadata struct {
	RedirectURIs                          []string `json:"redirect_uris"`
	ResponseTypes                         []string `json:"response_types,omitempty"`
	GrantTypes                            []string `json:"grant_types,omitempty"`
	Scope                                 string   `json:"scope,omitempty"`
	TokenEndpointAuthMethod               string   `json:"token_endpoint_auth_method,omitempty"`
	TokenEndpointAuthSigningAlg           string   `json:"token_endpoint_auth_signing_alg,omitempty"`
	UserinfoSignedResponseAlg             string   `json:"userinfo_signed_response_alg,omitempty"`
	UserinfoEncryptedResponseAlg          string   `json:"userinfo_encrypted_response_alg,omitempty"`
	JwksURI                               string   `json:"jwks_uri,omitempty"`
	ApplicationType                       string   `json:"application_type,omitempty"` // "web" or "native"
	SubjectType                           string   `json:"subject_type,omitempty"`     // "public" or "pairwise"
	RequestObjectSigningAlg               string   `json:"request_object_signing_alg,omitempty"`
	IDTokenSignedResponseAlg              string   `json:"id_token_signed_response_alg,omitempty"`
	AuthorizationSignedResponseAlg        string   `json:"authorization_signed_response_alg,omitempty"`
	AuthorizationEncryptedResponseEnc     string   `json:"authorization_encrypted_response_enc,omitempty"`
	AuthorizationEncryptedResponseAlg     string   `json:"authorization_encrypted_response_alg,omitempty"`
	ClientID                              string   `json:"client_id,omitempty"`
	ClientName                            string   `json:"client_name,omitempty"`
	ClientURI                             string   `json:"client_uri,omitempty"`
	PolicyURI                             string   `json:"policy_uri,omitempty"`
	TosURI                                string   `json:"tos_uri,omitempty"`
	LogoURI                               string   `json:"logo_uri,omitempty"`
	DefaultMaxAge                         int      `json:"default_max_age,omitempty"`
	RequireAuthTime                       *bool    `json:"require_auth_time,omitempty"`
	Contacts                              []string `json:"contacts,omitempty"`
	TLSClientCertificateBoundAccessTokens *bool    `json:"tls_client_certificate_bound_access_tokens,omitempty"`
	DPoPBoundAccessTokens                 *bool    `json:"dpop_bound_access_tokens,omitempty"`
	AuthorizationDetailsTypes             []string `json:"authorization_details_types,omitempty"`
	// Jwks                                  *JWKSet  `json:"jwks,omitempty"`             // You'll need to define JWKSet type
}

func boolPtr(b bool) *bool {
	return &b
}

func GetMetadata(host string, platform string) *OAuthClientMetadata {
	meta := &OAuthClientMetadata{
		ClientID:  fmt.Sprintf("https://%s/api/atproto-oauth/%s", host, platform),
		ClientURI: fmt.Sprintf("https://%s", host),
		// RedirectURIs:            []string{fmt.Sprintf("https://%s/login", host)},
		Scope:                   "atproto transition:generic",
		TokenEndpointAuthMethod: "none",
		ClientName:              "Aquareum",
		ResponseTypes:           []string{"code"},
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		DPoPBoundAccessTokens:   boolPtr(true),
	}
	if platform == "web" {
		meta.RedirectURIs = []string{fmt.Sprintf("https://%s/login", host)}
		meta.ApplicationType = "web"
	} else {
		hostParts := strings.Split(host, ".")
		slices.Reverse(hostParts)
		meta.RedirectURIs = []string{fmt.Sprintf("%s:/login", strings.Join(hostParts, "."))}
		meta.ApplicationType = "native"
	}
	return meta
}

// clientMetadata: {
// 	client_id: "http://localhost?scope=atproto%20transition:generic",
// 	redirect_uris: ["http://127.0.0.1:38081"],
// 	scope: "atproto transition:generic",
// 	token_endpoint_auth_method: "none",
// 	// jwks_uri: "https://my-app.example/jwks.json",
// 	client_name: "Loopback client",
// 	response_types: ["code"],
// 	grant_types: ["authorization_code", "refresh_token"],
// 	application_type: "native",
// 	dpop_bound_access_tokens: true,
// },
