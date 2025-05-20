package oproxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/AxisCommunications/go-dpop"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
)

type PAR struct {
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	State               string `json:"state"`
	LoginHint           string `json:"login_hint"`
	ResponseMode        string `json:"response_mode"`
	ResponseType        string `json:"response_type"`
	Scope               string `json:"scope"`
}

type PARResponse struct {
	RequestURI string `json:"request_uri"`
	ExpiresIn  int    `json:"expires_in"`
}

var ErrFirstNonce = echo.NewHTTPError(http.StatusBadRequest, "first time seeing this key, come back with a nonce")

func (o *OProxy) HandleOAuthPAR(c echo.Context) error {
	ctx, span := otel.Tracer("server").Start(c.Request().Context(), "HandleOAuthPAR")
	defer span.End()
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")
	var par PAR
	if err := json.NewDecoder(c.Request().Body).Decode(&par); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	dpopHeader := c.Request().Header.Get("DPoP")
	if dpopHeader == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "DPoP header is required")
	}

	resp, err := o.NewPAR(ctx, c, &par, dpopHeader)
	if errors.Is(err, ErrFirstNonce) {
		res := map[string]interface{}{
			"error":             "use_dpop_nonce",
			"error_description": "Authorization server requires nonce in DPoP proof",
		}
		return c.JSON(http.StatusBadRequest, res)
	} else if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, resp)
}

func (o *OProxy) NewPAR(ctx context.Context, c echo.Context, par *PAR, dpopHeader string) (*PARResponse, error) {
	jkt, claims, err := getJKT(dpopHeader)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to get JKT from DPoP header header=%s: %s", dpopHeader, err))
	}
	session, err := o.getOAuthSession(jkt)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to load OAuth session: %s", err))
	}
	// special case - if this is the first request, we need to send it back for a new nonce
	if session == nil {
		_, err := dpop.Parse(dpopHeader, dpop.POST, &url.URL{Host: o.host, Scheme: "https", Path: "/oauth/par"}, dpop.ParseOptions{
			Nonce:      claims.Nonce,
			TimeWindow: &dpopTimeWindow,
		})
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("failed to parse DPoP header: %s", err))
		}
		newNoncePad := makeNoncePad()
		err = o.createOAuthSession(jkt, &OAuthSession{
			DownstreamDPoPJKT:      jkt,
			DownstreamDPoPNoncePad: newNoncePad,
		})
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to create OAuth session: %s", err))
		}
		nonces := generateValidNonces(newNoncePad, time.Now())
		// come back later, nerd
		c.Response().Header().Set("DPoP-Nonce", nonces[0])
		return nil, ErrFirstNonce
	}
	nonces := generateValidNonces(session.DownstreamDPoPNoncePad, time.Now())
	if !slices.Contains(nonces, claims.Nonce) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid nonce")
	}
	proof, err := dpop.Parse(dpopHeader, dpop.POST, &url.URL{Host: o.host, Scheme: "https", Path: "/oauth/par"}, dpop.ParseOptions{
		Nonce:      claims.Nonce,
		TimeWindow: &dpopTimeWindow,
	})
	// Check the error type to determine response
	if err != nil {
		// if ok := errors.Is(err, dpop.ErrInvalidProof); ok {
		// 	apierrors.WriteHTTPBadRequest(w, "invalid DPoP proof", nil)
		// 	return
		// }
		// apierrors.WriteHTTPBadRequest(w, "invalid DPoP proof", err)
		// return
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid DPoP proof: %s", err))
	}
	if proof.PublicKey() != jkt {
		panic("invalid code path: parsed DPoP proof twice and got different keys?!")
	}

	clientMetadata, err := o.GetDownstreamMetadata(par.RedirectURI)
	if err != nil {
		return nil, err
	}
	if par.ClientID != clientMetadata.ClientID {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid client_id: expected %s, got %s", clientMetadata.ClientID, par.ClientID))
	}

	if !slices.Contains(clientMetadata.RedirectURIs, par.RedirectURI) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid redirect_uri: %s not in allowed URIs", par.RedirectURI))
	}

	if par.CodeChallengeMethod != "S256" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid code challenge method: expected S256, got %s", par.CodeChallengeMethod))
	}

	if par.ResponseMode != "query" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid response mode: expected query, got %s", par.ResponseMode))
	}

	if par.ResponseType != "code" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid response type: expected code, got %s", par.ResponseType))
	}

	if par.Scope != o.scope {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid scope")
	}

	if par.LoginHint == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "login hint is required to find your PDS")
	}

	if par.State == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "state is required")
	}

	if par.Scope != o.scope {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid scope (expected %s, got %s)", o.scope, par.Scope))
	}

	urn := makeURN(jkt)

	err = o.updateOAuthSession(jkt, &OAuthSession{
		DownstreamDPoPJKT:       jkt,
		DownstreamPARRequestURI: urn,
		DownstreamCodeChallenge: par.CodeChallenge,
		DownstreamState:         par.State,
		DownstreamRedirectURI:   par.RedirectURI,
		Handle:                  par.LoginHint,
	})
	if err != nil {
		return nil, fmt.Errorf("could not create oauth session: %w", err)
	}
	c.Response().Header().Set("DPoP-Nonce", nonces[0])

	resp := &PARResponse{
		RequestURI: urn,
		ExpiresIn:  int(dpopTimeWindow.Seconds()),
	}

	return resp, nil
}
