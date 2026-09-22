// Package psession is the signed playback session identifier: the one
// credential a playback URL carries. HLS clients (native players above all)
// can't sign requests, so whatever a playback URL is allowed to fetch has to
// be readable from the URL itself. A session names a playback (its TID, the
// key viewer counts and view logs group by), a scope (what it may fetch), an
// expiry, and is HMAC-signed together with the streamer it belongs to, so a
// session minted for one stream opens no other and a forged one opens
// nothing.
//
// Wire form, carried as the sid query parameter:
//
//	1.<tid>.<scope>.<expiry unix seconds>.<signature>
//
// The signature is the first 16 bytes of HMAC-SHA256 over
// "1|tid|scope|expiry|streamer", base64url without padding.
package psession

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"stream.place/streamplace/pkg/spid"
)

// Scope is what a session may fetch.
type Scope string

const (
	// ScopePublic is an ordinary viewer: published live streams and videos.
	ScopePublic Scope = "p"
	// ScopeOwner is the streamer's own session: it also opens their
	// unpublished (pre-live) stream.
	ScopeOwner Scope = "o"

	version = "1"
	// sigBytes of the HMAC are kept: 128 bits is plenty for a token whose
	// worst-case leak opens one stream to one more viewer.
	sigBytes = 16
)

var (
	ErrMalformed = errors.New("playback session: malformed")
	ErrSignature = errors.New("playback session: bad signature")
	ErrExpired   = errors.New("playback session: expired")
)

// Session is a parsed, verified playback session.
type Session struct {
	// ID is the session's TID: what viewer counts and view logs key by,
	// stable across renewals.
	ID      string
	Scope   Scope
	Expires time.Time
}

func sign(key []byte, id string, scope Scope, exp int64, streamer string) string {
	mac := hmac.New(sha256.New, key)
	fmt.Fprintf(mac, "%s|%s|%s|%d|%s", version, id, scope, exp, streamer)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:sigBytes])
}

func encode(key []byte, id string, scope Scope, exp time.Time, streamer string) string {
	e := exp.Unix()
	return strings.Join([]string{version, id, string(scope), strconv.FormatInt(e, 10), sign(key, id, scope, e, streamer)}, ".")
}

// Mint issues a new session for streamer, valid for ttl from now.
func Mint(key []byte, streamer string, scope Scope, ttl time.Duration, now time.Time) (Session, string) {
	s := Session{ID: spid.TID(), Scope: scope, Expires: now.Add(ttl).Truncate(time.Second).UTC()}
	return s, encode(key, s.ID, s.Scope, s.Expires, streamer)
}

// Renew re-issues s for streamer with a fresh expiry, keeping its ID so the
// playback keeps counting as one session.
func Renew(key []byte, s Session, streamer string, ttl time.Duration, now time.Time) (Session, string) {
	s.Expires = now.Add(ttl).Truncate(time.Second).UTC()
	return s, encode(key, s.ID, s.Scope, s.Expires, streamer)
}

// WellFormed reports whether value has the shape of a session, without
// checking its signature: what a handler can refuse before it has a key.
func WellFormed(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 5 || parts[0] != version {
		return false
	}
	if _, err := syntax.ParseTID(parts[1]); err != nil {
		return false
	}
	if scope := Scope(parts[2]); scope != ScopePublic && scope != ScopeOwner {
		return false
	}
	_, err := strconv.ParseInt(parts[3], 10, 64)
	return err == nil && parts[4] != ""
}

// Parse verifies value as a session for streamer at now. An expired session
// comes back with ErrExpired and the parsed session, so a caller can renew
// it in place.
func Parse(key []byte, value, streamer string, now time.Time) (Session, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 5 || parts[0] != version {
		return Session{}, ErrMalformed
	}
	if _, err := syntax.ParseTID(parts[1]); err != nil {
		return Session{}, ErrMalformed
	}
	scope := Scope(parts[2])
	if scope != ScopePublic && scope != ScopeOwner {
		return Session{}, ErrMalformed
	}
	exp, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return Session{}, ErrMalformed
	}
	if !hmac.Equal([]byte(parts[4]), []byte(sign(key, parts[1], scope, exp, streamer))) {
		return Session{}, ErrSignature
	}
	s := Session{ID: parts[1], Scope: scope, Expires: time.Unix(exp, 0).UTC()}
	if now.After(s.Expires) {
		return s, ErrExpired
	}
	return s, nil
}

// ID returns the session id a sid value names, signed or not: the TID that
// viewer counts and view logs group by. Renewals of one session share it.
func ID(value string) string {
	if i := strings.Index(value, "."); i > 0 && strings.HasPrefix(value, version+".") {
		rest := value[i+1:]
		if j := strings.Index(rest, "."); j > 0 {
			return rest[:j]
		}
		return rest
	}
	return value
}
