package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/AxisCommunications/go-dpop"
	"github.com/haileyok/atproto-oauth-golang/helpers"
	"github.com/julienschmidt/httprouter"
	"go.opentelemetry.io/otel"
	"stream.place/streamplace/pkg/atproto"
	apierrors "stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
)

func (a *StreamplaceAPI) HandleATProtoOAuthUpstream(ctx context.Context, platform string) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		host, _, err := net.SplitHostPort(req.Host)
		if err != nil {
			host = req.Host
		}
		if !slices.Contains(atproto.AllowedPlatforms, platform) {
			apierrors.WriteHTTPBadRequest(w, "unsupported platform", nil)
			return
		}

		meta := atproto.GetUpstreamMetadata(host, platform, a.CLI.AppBundleID)
		bs, err := json.Marshal(meta)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not marshal metadata", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(bs)
	}
}

func (a *StreamplaceAPI) HandleATProtoOAuthDownstream(ctx context.Context, platform string) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		host, _, err := net.SplitHostPort(req.Host)
		if err != nil {
			host = req.Host
		}
		if !slices.Contains(atproto.AllowedPlatforms, platform) {
			apierrors.WriteHTTPBadRequest(w, "unsupported platform", nil)
			return
		}

		meta := atproto.GetDownstreamMetadata(host, platform, a.CLI.AppBundleID)
		bs, err := json.Marshal(meta)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not marshal metadata", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(bs)
	}
}

func (a *StreamplaceAPI) HandleJWKPublic(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		pubKey, err := a.CLI.JWK.PublicKey()
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not get public key", err)
			return
		}
		bs, err := json.Marshal(helpers.CreateJwksResponseObject(pubKey))
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not marshal public key", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(bs)
	}
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
		"jwks_uri":                              fmt.Sprintf("https://%s/api/oauth/jwks", host),
		"authorization_endpoint":                fmt.Sprintf("https://%s/api/oauth/authorize", host),
		"token_endpoint":                        fmt.Sprintf("https://%s/api/oauth/token", host),
		"token_endpoint_auth_methods_supported": []string{"none", "private_key_jwt"},
		"revocation_endpoint":                   fmt.Sprintf("https://%s/api/oauth/revoke", host),
		"introspection_endpoint":                fmt.Sprintf("https://%s/api/oauth/introspect", host),
		"pushed_authorization_request_endpoint": fmt.Sprintf("https://%s/api/oauth/par", host),
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

func (a *StreamplaceAPI) HandleOAuthAuthorizationServer(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(generateOAuthServerMetadata("longos.iameli.link"))
	}
}

func (a *StreamplaceAPI) HandleOAuthProtectedResource(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"resource": "https://longos.iameli.link",
			"authorization_servers": []string{
				"https://longos.iameli.link",
			},
			"scopes_supported": []string{},
			"bearer_methods_supported": []string{
				"header",
			},
			"resource_documentation": "https://atproto.com",
		})
	}
}

func (a *StreamplaceAPI) HandleOAuthPAR(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		var par model.PAR
		if err := json.NewDecoder(r.Body).Decode(&par); err != nil {
			apierrors.WriteHTTPBadRequest(w, "invalid request", err)
			return
		}

		dpopHeader := r.Header.Get("DPoP")
		if dpopHeader == "" {
			apierrors.WriteHTTPBadRequest(w, "DPoP header is required", nil)
			return
		}

		thirtySec := time.Duration(30 * time.Second)
		proof, err := dpop.Parse(dpopHeader, dpop.POST, &url.URL{Host: r.Host, Scheme: "https", Path: "/api/oauth/par"}, dpop.ParseOptions{
			Nonce:      "",
			TimeWindow: &thirtySec,
		})
		// Check the error type to determine response
		if err != nil {
			if ok := errors.Is(err, dpop.ErrInvalidProof); ok {
				apierrors.WriteHTTPBadRequest(w, "invalid DPoP proof", nil)
				return
			}
			apierrors.WriteHTTPBadRequest(w, "invalid DPoP proof", err)
			return
		}

		// proof is valid, get public key to associate with access token
		par.JKT = proof.PublicKey()

		if err := a.Model.CreatePAR(&par); err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not create par", err)
			return
		}
		resp := par.ToPARResponse()
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(resp)
	}
}

func (a *StreamplaceAPI) HandleOAuthAuthorize(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		query := r.URL.Query()
		parID := query.Get("request_uri")
		if parID == "" {
			apierrors.WriteHTTPBadRequest(w, "request_uri is required", nil)
			return
		}
		par, err := a.Model.GetPAR(parID)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not get par", err)
			return
		}
		if par == nil {
			apierrors.WriteHTTPBadRequest(w, "par not found", nil)
			return
		}
		if par.ExpiresAt.Before(time.Now()) {
			apierrors.WriteHTTPBadRequest(w, "par expired", nil)
			return
		}
		if par.LoginHint == "" {
			apierrors.WriteHTTPBadRequest(w, "login hint is required", nil)
			return
		}
		redirectURL, err := atproto.Login(ctx, a.CLI, par, a.Model)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not login", err)
			return
		}
		http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
	}
}

func (a *StreamplaceAPI) HandleOAuthReturn(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := otel.Tracer("server").Start(ctx, "HandlePlaceStreamAccountOauthReturn")
		defer span.End()
		code := r.URL.Query().Get("code")
		iss := r.URL.Query().Get("iss")
		state := r.URL.Query().Get("state")
		upstreamSession, err := atproto.HandleOauthReturn(ctx, a.CLI, code, iss, state, a.Model)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not handle oauth return", err)
			return
		}
		if upstreamSession == nil {
			log.Error(ctx, "no upstream session found", "upstreamSession", upstreamSession)
			apierrors.WriteHTTPBadRequest(w, "no upstream session found", nil)
			return
		}
		if upstreamSession.DownstreamPAR == nil {
			log.Error(ctx, "no downstream par found", "upstreamSession", upstreamSession)
			apierrors.WriteHTTPBadRequest(w, "no downstream par found", nil)
			return
		}

		u, err := url.Parse("https://longos.iameli.link/login")
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "could not parse redirect url", err)
			return
		}
		q := u.Query()
		q.Set("iss", "https://longos.iameli.link")
		q.Set("state", upstreamSession.DownstreamPAR.State)
		q.Set("code", "asdf")
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusTemporaryRedirect)
	}
}

// TokenRequest represents the structure of an OAuth token request

func (a *StreamplaceAPI) HandleOAuthToken(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var tokenRequest atproto.TokenRequest
		if err := json.NewDecoder(r.Body).Decode(&tokenRequest); err != nil {
			apierrors.WriteHTTPBadRequest(w, "invalid request", err)
			return
		}

		// Verify the token request parameters
		if tokenRequest.GrantType == "authorization_code" {
			a.handleAuthToken(w, r, tokenRequest)
			return
		} else if tokenRequest.GrantType == "refresh_token" {
			a.handleRefreshToken(w, r, tokenRequest)
			return
		}

		apierrors.WriteHTTPBadRequest(w, "unsupported grant type", nil)
	}
}

func (a *StreamplaceAPI) handleAuthToken(w http.ResponseWriter, r *http.Request, tokenRequest atproto.TokenRequest) {
	if tokenRequest.Code == "" || tokenRequest.CodeVerifier == "" {
		apierrors.WriteHTTPBadRequest(w, "missing required parameters", nil)
		return
	}

	session, err := atproto.HandleOAuthToken(r.Context(), a.CLI, &tokenRequest, a.Model)
	if err != nil {
		apierrors.WriteHTTPBadRequest(w, "could not handle oauth token", err)
		return
	}

	response := map[string]interface{}{
		"access_token":  session.DownstreamAccessToken,
		"token_type":    "DPoP",
		"refresh_token": session.DownstreamRefreshToken,
		"scope":         "atproto transition:generic",
		"expires_in":    atproto.OAuthTokenExpiry.Seconds(),
		"sub":           session.RepoDID,
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(response)
}

func (a *StreamplaceAPI) handleRefreshToken(w http.ResponseWriter, r *http.Request, tokenRequest atproto.TokenRequest) {
	session, err := atproto.HandleOAuthRefreshToken(r.Context(), a.CLI, &tokenRequest, a.Model)
	if err != nil {
		apierrors.WriteHTTPBadRequest(w, "could not handle oauth token", err)
		return
	}

	response := map[string]interface{}{
		"access_token":  session.DownstreamAccessToken,
		"token_type":    "DPoP",
		"refresh_token": session.DownstreamRefreshToken,
		"scope":         "atproto transition:generic",
		"expires_in":    atproto.OAuthTokenExpiry.Seconds(),
		"sub":           session.RepoDID,
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(response)
}
