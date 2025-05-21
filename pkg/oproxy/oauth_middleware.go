package oproxy

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/AxisCommunications/go-dpop"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

var OAuthSessionContextKey = oauthSessionContextKeyType{}

type oauthSessionContextKeyType struct{}

var OProxyContextKey = oproxyContextKeyType{}

type oproxyContextKeyType struct{}

func GetOAuthSession(ctx context.Context) (*OAuthSession, *XrpcClient) {
	o, ok := ctx.Value(OProxyContextKey).(*OProxy)
	if !ok {
		return nil, nil
	}
	session, ok := ctx.Value(OAuthSessionContextKey).(*OAuthSession)
	if !ok {
		return nil, nil
	}
	client, err := o.GetXrpcClient(session)
	if err != nil {
		return nil, nil
	}
	return session, client
}

func getMethod(method string) (dpop.HTTPVerb, error) {
	switch method {
	case "POST":
		return dpop.POST, nil
	case "GET":
		return dpop.GET, nil
	}
	return "", fmt.Errorf("invalid method")
}
func (o *OProxy) OAuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Set CORS headers
		c.Response().Header().Set("Access-Control-Allow-Origin", "*") // todo: ehhhhhhhhhhhh
		c.Response().Header().Set("Access-Control-Allow-Headers", "Content-Type,DPoP")
		c.Response().Header().Set("Access-Control-Allow-Methods", "*")
		c.Response().Header().Set("Access-Control-Expose-Headers", "DPoP-Nonce")

		authHeader := c.Request().Header.Get("Authorization")
		if authHeader == "" {
			return next(c)
		}
		if !strings.HasPrefix(authHeader, "DPoP ") {
			return next(c)
		}
		token := strings.TrimPrefix(authHeader, "DPoP ")

		dpopHeader := c.Request().Header.Get("DPoP")
		if dpopHeader == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "missing DPoP header")
		}

		dpopMethod, err := getMethod(c.Request().Method)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid method: %v", err))
		}

		u, err := url.Parse(c.Request().URL.String())
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid url: %v", err))
		}
		u.Scheme = "https"
		u.Host = c.Request().Host
		u.RawQuery = ""
		u.Fragment = ""

		jkt, dpopClaims, err := getJKT(dpopHeader)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		session, err := o.getOAuthSession(jkt)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("could not get oauth session: %v", err))
		}
		if session == nil {
			// this can happen for stuff like getFeedSkeleton where they've submitted oauth credentials
			// but they're not actually for this server
			return next(c)
		}
		if session.RevokedAt != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "oauth session revoked")
		}

		validNonces := generateValidNonces(session.DownstreamDPoPNoncePad, time.Now())
		if !slices.Contains(validNonces, dpopClaims.Nonce) {
			c.Response().Header().Set("WWW-Authenticate", `DPoP algs="RS256 RS384 RS512 PS256 PS384 PS512 ES256 ES256K ES384 ES512", error="use_dpop_nonce", error_description="Authorization server requires nonce in DPoP proof"`)
			c.Response().Header().Set("DPoP-Nonce", validNonces[0])
			return c.JSON(http.StatusUnauthorized, map[string]interface{}{
				"error":             "use_dpop_nonce",
				"error_description": "Authorization server requires nonce in DPoP proof",
			})
		}
		c.Response().Header().Set("DPoP-Nonce", validNonces[0])

		proof, err := dpop.Parse(dpopHeader, dpopMethod, u, dpop.ParseOptions{
			Nonce:      dpopClaims.Nonce,
			TimeWindow: &dpopTimeWindow,
		})
		if err != nil {
			if errors.Is(err, dpop.ErrInvalidProof) {
				return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("invalid DPoP proof: %v", err))
			}
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("error validating proof: %v", err))
		}

		hasher := sha256.New()
		hasher.Write([]byte(token))
		hash := hasher.Sum(nil)
		accessTokenHash := base64.RawURLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash)

		pubKey, err := o.downstreamJWK.PublicKey()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("could not get access jwk public key: %v", err))
		}

		var pubKeyECDSA ecdsa.PublicKey
		err = pubKey.Raw(&pubKeyECDSA)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("could not get access jwk public key: %v", err))
		}

		accessClaims := &dpop.BoundAccessTokenClaims{}
		accessTokenJWT, err := jwt.ParseWithClaims(token, accessClaims, func(token *jwt.Token) (any, error) {
			return &pubKeyECDSA, nil
		})
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("could not parse access token: %v", err))
		}

		err = proof.Validate([]byte(accessTokenHash), accessTokenJWT)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("invalid proof: %v", err))
		}

		err = session.CacheJTI(dpopClaims.ID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("could not cache jti: %v", err))
		}

		err = o.updateOAuthSession(session.DownstreamDPoPJKT, session)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("could not update oauth session: %v", err))
		}

		// Set session in context
		c.Set("session", session)
		c.Set("oproxy", o)

		o.slog.Info("msg", "oauth middleware", "session", session.DownstreamDPoPJKT)

		// Also set it in request context for non-echo handlers
		ctx := c.Request().Context()
		ctx = context.WithValue(ctx, oproxyContextKeyType{}, o)
		ctx = context.WithValue(ctx, OAuthSessionContextKey, session)
		c.SetRequest(c.Request().WithContext(ctx))
		return next(c)
	}
}

func (o *OProxy) DPoPNonceMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		dpopHeader := c.Request().Header.Get("DPoP")
		if dpopHeader == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "missing DPoP header")
		}

		jkt, _, err := getJKT(dpopHeader)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		session, err := o.getOAuthSession(jkt)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		c.Set("session", session)
		return next(c)
	}
}

func (o *OProxy) ErrorHandlingMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := next(c)
		if err == nil {
			return nil
		}
		httpError, ok := err.(*echo.HTTPError)
		if ok {
			o.slog.Error("oauth error", "code", httpError.Code, "message", httpError.Message, "internal", httpError.Internal)
			return err
		}
		o.slog.Error("unhandled error", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
}
