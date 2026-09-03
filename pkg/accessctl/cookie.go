package accessctl

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ViewerCookieName carries a signed statement that the bearer is did,
// so routes that browsers hit without an Authorization header (playback,
// thumbnails, chat and livestream websockets) can apply the viewer gate.
// It is minted on every authenticated XRPC response from an allowed viewer
// and never on its own is proof of anything beyond "this DID was allowed
// when the cookie was minted"; the gate re-checks the DID against the live
// snapshot on every request.
const ViewerCookieName = "sp_access"

const viewerCookieTTL = 7 * 24 * time.Hour

func (c *Controller) sign(did string, exp int64) []byte {
	mac := hmac.New(sha256.New, c.cookieKey)
	mac.Write([]byte(did))
	mac.Write([]byte{0})
	mac.Write([]byte(strconv.FormatInt(exp, 10)))
	return mac.Sum(nil)
}

// ViewerCookie implements access.Manager.
func (c *Controller) ViewerCookie(did string) *http.Cookie {
	exp := time.Now().Add(viewerCookieTTL).Unix()
	enc := base64.RawURLEncoding
	value := enc.EncodeToString([]byte(did)) + "." + strconv.FormatInt(exp, 10) + "." + enc.EncodeToString(c.sign(did, exp))
	return &http.Cookie{
		Name:     ViewerCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(viewerCookieTTL / time.Second),
		HttpOnly: true,
		Secure:   c.cli.HasHTTPS() || c.cli.BehindHTTPSProxy,
		SameSite: http.SameSiteLaxMode,
	}
}

// ViewerFromCookie implements access.Manager.
func (c *Controller) ViewerFromCookie(r *http.Request) (string, bool) {
	ck, err := r.Cookie(ViewerCookieName)
	if err != nil {
		return "", false
	}
	return c.verify(ck.Value, time.Now())
}

func (c *Controller) verify(value string, now time.Time) (string, bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return "", false
	}
	enc := base64.RawURLEncoding
	didBs, err := enc.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || now.Unix() > exp {
		return "", false
	}
	sig, err := enc.DecodeString(parts[2])
	if err != nil {
		return "", false
	}
	if !hmac.Equal(sig, c.sign(string(didBs), exp)) {
		return "", false
	}
	return string(didBs), true
}
