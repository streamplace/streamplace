// Package bunny is the bunny.net CDN provider: Token Authentication
// URL signing for pull zones and an ingester for the pull zone's
// permanent log storage.
package bunny

import (
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Signer implements cdn.Signer with bunny.net Token Authentication in
// its query-string form.
type Signer struct {
	// Key is the pull zone's Token Authentication key.
	Key string
}

// SignURL mirrors bunny's reference implementation: the token is the
// unpadded base64url SHA-256 of `key + decodedPath + expires + params`,
// where params is every non-empty query parameter as `k=v`, sorted by
// key and joined with `&`. The signed URL carries the same params
// (URL-encoded) plus `token` and `expires`; bunny recomputes the hash
// on its side and refuses anything that doesn't match or has expired.
// Because the path is in the hash, a token for one blob can't be
// replayed against another.
func (s Signer) SignURL(u *url.URL, expires time.Time) string {
	q := u.Query()
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var hashParams, urlParams strings.Builder
	for _, k := range keys {
		for _, v := range q[k] {
			if v == "" {
				continue
			}
			if hashParams.Len() > 0 {
				hashParams.WriteByte('&')
			}
			hashParams.WriteString(k + "=" + v)
			urlParams.WriteString("&" + url.QueryEscape(k) + "=" + url.QueryEscape(v))
		}
	}
	exp := strconv.FormatInt(expires.Unix(), 10)
	sum := sha256.Sum256([]byte(s.Key + u.Path + exp + hashParams.String()))
	token := base64.RawURLEncoding.EncodeToString(sum[:])
	return u.Scheme + "://" + u.Host + u.EscapedPath() +
		"?token=" + token + urlParams.String() + "&expires=" + exp
}
