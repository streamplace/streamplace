package spaces

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/xrpc"
)

// notifySecret keys the tokens of this process, so a restart issues new ones.
var notifySecret = []byte(rand.Text())

// NotifyToken is the secret a PDS presents to this process with each
// notification about space, a space URI.
func NotifyToken(space string) string {
	mac := hmac.New(sha256.New, notifySecret)
	mac.Write([]byte(space))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// Follow asks did's PDS to notify endpoint, the HTTPS base URL of this node,
// when their personal space changes (com.atproto.space.notifySpaceUpdate). The
// PDS stops at the returned time unless asked again before it. user acts as
// did; pds reaches their PDS.
func Follow(ctx context.Context, user Client, pds *xrpc.Client, did, endpoint string) (time.Time, error) {
	space := PersonalSpaceURI(did)
	var delegation struct {
		Token string `json:"token"`
	}
	err := user.Do(ctx, "GET", "", "com.atproto.space.getDelegationToken", map[string]any{"space": space}, nil, &delegation)
	if err != nil {
		return time.Time{}, fmt.Errorf("get space delegation token: %w", err)
	}
	// The PDS answers with a credential bound to a key. Nothing here reads
	// the space with it, so the key is not kept.
	key, err := atcrypto.GeneratePrivateKeyP256()
	if err != nil {
		return time.Time{}, err
	}
	pub, err := key.PublicKey()
	if err != nil {
		return time.Time{}, err
	}
	var out struct {
		Credential string `json:"credential"`
	}
	err = pds.Do(ctx, "POST", "application/json", "com.atproto.space.authorizeSpaceSync", nil, map[string]any{
		"space":           space,
		"credentialKey":   pub.DIDKey(),
		"delegationToken": delegation.Token,
		"notify":          map[string]string{"endpoint": endpoint, "token": NotifyToken(space)},
	}, &out)
	if err != nil {
		return time.Time{}, fmt.Errorf("authorize space sync: %w", err)
	}
	// The registration lasts as long as the credential, a JWT.
	var claims struct {
		Exp float64 `json:"exp"`
	}
	if parts := strings.Split(out.Credential, "."); len(parts) == 3 {
		payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
		_ = json.Unmarshal(payload, &claims)
	}
	if claims.Exp == 0 {
		return time.Time{}, errors.New("space credential has no expiry")
	}
	return time.Unix(int64(claims.Exp), 0), nil
}
