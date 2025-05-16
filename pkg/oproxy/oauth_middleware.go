package oproxy

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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

func (o *OProxy) OAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// todo: see what these were set to before it got to us.
		w.Header().Set("Access-Control-Allow-Origin", "*") // todo: ehhhhhhhhhhhh
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,DPoP")
		w.Header().Set("Access-Control-Allow-Methods", "*")
		w.Header().Set("Access-Control-Expose-Headers", "DPoP-Nonce")

		ctx := r.Context()
		session, err := o.getOAuthSession(r, w)
		if err != nil {
			if errors.Is(err, dpop.ErrIncorrectNonce) {
				// w.Header().Set("WWW-Authenticate", `DPoP error="use_dpop_nonce", error_description="Invalid nonce"`)
				w.Header().Set("content-type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				bs, _ := json.Marshal(map[string]interface{}{
					"error":             "use_dpop_nonce",
					"error_description": "Authorization server requires nonce in DPoP proof",
				})
				w.Write(bs)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		if session == nil {
			next.ServeHTTP(w, r)
			return
		}
		ctx = context.WithValue(ctx, OAuthSessionContextKey, session)
		ctx = context.WithValue(ctx, OProxyContextKey, o)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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

func (o *OProxy) getOAuthSession(r *http.Request, w http.ResponseWriter) (*OAuthSession, error) {

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, nil
	}
	if !strings.HasPrefix(authHeader, "DPoP ") {
		return nil, fmt.Errorf("invalid authorization header (must start with DPoP)")
	}
	token := strings.TrimPrefix(authHeader, "DPoP ")

	dpopHeader := r.Header.Get("DPoP")
	if dpopHeader == "" {
		return nil, fmt.Errorf("missing DPoP header")
	}

	dpopMethod, err := getMethod(r.Method)
	if err != nil {
		return nil, fmt.Errorf("invalid method: %w", err)
	}

	u, err := url.Parse(r.URL.String())
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	u.Scheme = "https"
	u.Host = r.Host
	u.RawQuery = ""
	u.Fragment = ""

	jkt, nonce, err := getJKT(dpopHeader)

	session, err := o.loadOAuthSession(jkt)
	if err != nil {
		return nil, fmt.Errorf("could not get oauth session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("oauth session not found")
	}
	if session.RevokedAt != nil {
		return nil, fmt.Errorf("oauth session revoked")
	}
	if session.DownstreamDPoPNonce != nonce {
		w.Header().Set("WWW-Authenticate", `DPoP algs="RS256 RS384 RS512 PS256 PS384 PS512 ES256 ES256K ES384 ES512", error="use_dpop_nonce", error_description="Authorization server requires nonce in DPoP proof"`)
		w.Header().Set("DPoP-Nonce", session.DownstreamDPoPNonce)
		return nil, dpop.ErrIncorrectNonce
	}

	session.DownstreamDPoPNonce = makeNonce()
	err = o.updateOAuthSession(session.DownstreamDPoPJKT, session)
	if err != nil {
		return nil, fmt.Errorf("could not update downstream session: %w", err)
	}
	w.Header().Set("DPoP-Nonce", session.DownstreamDPoPNonce)

	proof, err := dpop.Parse(dpopHeader, dpopMethod, u, dpop.ParseOptions{
		Nonce:      nonce,
		TimeWindow: &dpopTimeWindow,
	})
	// Check the error type to determine response
	if err != nil {
		if ok := errors.Is(err, dpop.ErrInvalidProof); ok {
			// Return 'invalid_dpop_proof'
			return nil, fmt.Errorf("invalid DPoP proof: %w", err)
		}
		return nil, fmt.Errorf("error validating proof proof: %w", err)
	}

	// Hash the token with base64 and SHA256
	// Get the access token JWT (introspect if needed)
	// Parse the access token JWT and verify the signature
	// Hash the access token with SHA-256
	hasher := sha256.New()
	hasher.Write([]byte(token))
	hash := hasher.Sum(nil)

	// Encode the hash in URL-safe base64 format without padding
	// accessTokenHash := base64.RawURLEncoding.EncodeToString(hash)
	accessTokenHash := base64.RawURLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash)
	pubKey, err := o.downstreamJWK.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("could not get access jwk public key: %w", err)
	}
	var pubKeyECDSA ecdsa.PublicKey
	err = pubKey.Raw(&pubKeyECDSA)
	if err != nil {
		return nil, fmt.Errorf("could not get access jwk public key: %w", err)
	}

	// Parse the access token JWT
	claims := &dpop.BoundAccessTokenClaims{}
	accessTokenJWT, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (any, error) {
		return &pubKeyECDSA, nil
	})

	if err != nil {
		return nil, fmt.Errorf("could not parse access token: %w", err)
	}

	err = proof.Validate([]byte(accessTokenHash), accessTokenJWT)
	// Check the error type to determine response
	if err != nil {
		return nil, fmt.Errorf("invalid proof: %w", err)
	}

	return session, nil
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

		session, err := o.loadOAuthSession(jkt)
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
