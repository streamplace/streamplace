package spaces

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/xrpc"
)

// ChangedTopic is the bus topic that wakes whatever follows did's personal space.
func ChangedTopic(did string) string {
	return PersonalSpaceURI(did)
}

// The components a space credential's HTTP message signature covers.
const signatureParams = `("authorization" "atproto-space-audience");tag="atproto-space-credential"`

// RegisterNotify asks did's PDS to call service, a DID with a service
// fragment, when their personal space changes (com.atproto.space.notifyWrite),
// until the returned time. user acts as did; pds reaches their PDS.
func RegisterNotify(ctx context.Context, user Client, pds xrpc.Client, did, service string) (time.Time, error) {
	space := PersonalSpaceURI(did)
	// The user's session delegates to a credential bound to a key made here.
	var delegation struct {
		Token string `json:"token"`
	}
	err := user.Do(ctx, "GET", "", "com.atproto.space.getDelegationToken", map[string]any{"space": space}, nil, &delegation)
	if err != nil {
		return time.Time{}, fmt.Errorf("get space delegation token: %w", err)
	}
	key, err := atcrypto.GeneratePrivateKeyP256()
	if err != nil {
		return time.Time{}, err
	}
	pub, err := key.PublicKey()
	if err != nil {
		return time.Time{}, err
	}
	var issued struct {
		Credential string `json:"credential"`
	}
	err = pds.Do(ctx, "POST", "application/json", "com.atproto.space.getSpaceCredential", nil, map[string]any{
		"space":           space,
		"credentialKid":   pub.DIDKey(),
		"delegationToken": delegation.Token,
	}, &issued)
	if err != nil {
		return time.Time{}, fmt.Errorf("get space credential: %w", err)
	}
	// The key signs the two headers that present the credential to its audience.
	authorization := "Atproto-Space-Credential " + issued.Credential
	sig, err := key.HashAndSign([]byte(`"authorization": ` + authorization + "\n" +
		`"atproto-space-audience": ` + did + "\n" +
		`"@signature-params": ` + signatureParams))
	if err != nil {
		return time.Time{}, err
	}
	pds.Headers = map[string]string{
		"Authorization":          authorization,
		"Atproto-Space-Audience": did,
		"Signature-Input":        "atproto-space-credential=" + signatureParams,
		"Signature":              "atproto-space-credential=:" + base64.StdEncoding.EncodeToString(sig) + ":",
	}
	var out struct {
		ExpiresAt time.Time `json:"expiresAt"`
	}
	err = pds.Do(ctx, "POST", "application/json", "com.atproto.space.registerNotify", nil, map[string]any{
		"space":   space,
		"service": service,
	}, &out)
	if err != nil {
		return time.Time{}, fmt.Errorf("register for space notifications: %w", err)
	}
	return out.ExpiresAt, nil
}
