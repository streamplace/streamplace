package atproto

import (
	"context"
	"encoding/json"
	"os"

	oauth_helpers "github.com/haileyok/atproto-oauth-golang/helpers"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"stream.place/streamplace/pkg/log"
)

func EnsureJWK(ctx context.Context, fPath string) (jwk.Key, error) {
	var key jwk.Key
	_, err := os.Stat(fPath)
	if err == nil {
		b, err := os.ReadFile(fPath)
		if err != nil {
			return nil, err
		}
		key, err = jwk.ParseKey(b)
		if err != nil {
			return nil, err
		}
	} else if os.IsNotExist(err) {
		key, err = oauth_helpers.GenerateKey(nil)
		if err != nil {
			return nil, err
		}

		b, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}

		if err := os.WriteFile(fPath, b, 0600); err != nil {
			return nil, err
		}
		log.Log(ctx, "generated JWK", "path", fPath)
	} else {
		return nil, err
	}

	return key, nil
}
