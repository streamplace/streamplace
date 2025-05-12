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
	"strings"
	"time"

	"github.com/AxisCommunications/go-dpop"
	"github.com/golang-jwt/jwt/v5"
	"stream.place/streamplace/pkg/log"
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
		ctx := r.Context()
		session, err := o.getOAuthSession(r)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
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

func (o *OProxy) getOAuthSession(r *http.Request) (*OAuthSession, error) {
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

	thirtySec := time.Duration(30) * time.Second
	u, err := url.Parse(r.URL.String())
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	u.Scheme = "https"
	u.Host = r.Host
	u.RawQuery = ""
	u.Fragment = ""
	log.Log(r.Context(), "doing dpop", "dpop", dpopHeader, "method", r.Method, "url", u.String())

	proof, err := dpop.Parse(dpopHeader, dpopMethod, u, dpop.ParseOptions{
		Nonce:      "",
		TimeWindow: &thirtySec,
	})
	// Check the error type to determine response
	if err != nil {
		if ok := errors.Is(err, dpop.ErrInvalidProof); ok {
			// Return 'invalid_dpop_proof'
			return nil, err
		}
		return nil, err
	}

	session, err := o.loadOAuthSession(proof.PublicKey())
	if err != nil {
		return nil, fmt.Errorf("could not get oauth session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("oauth session not found")
	}

	if session.RevokedAt != nil {
		return nil, fmt.Errorf("oauth session revoked")
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
