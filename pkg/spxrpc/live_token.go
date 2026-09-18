package spxrpc

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Pre-live HLS playback tokens. HLS requests carry no session, so a streamer
// previewing their own unpublished stream over HLS proves who they are with
// a token minted through the authenticated getLiveToken query: the DID and
// an expiry, HMAC-signed with a key shared through statedb (so
// every node in a station honours it).

const liveTokenTTL = time.Hour

// liveTokenKey is the station's token key, read from statedb once: token
// checks happen on every pre-live playlist and segment request, and the key
// never changes once it exists.
func (s *Server) liveTokenKey(ctx context.Context) ([]byte, error) {
	if key := s.liveTokenKeyCache.Load(); key != nil {
		return *key, nil
	}
	if s.statefulDB == nil {
		return nil, fmt.Errorf("no statedb")
	}
	key, err := s.statefulDB.EnsureLiveTokenKey(ctx)
	if err != nil {
		return nil, err
	}
	s.liveTokenKeyCache.Store(&key)
	return key, nil
}

func liveTokenSig(key []byte, did string, exp int64) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(did + "\x00" + strconv.FormatInt(exp, 10)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// mintLiveToken returns a token for did valid until the returned time.
func mintLiveToken(key []byte, did string, now time.Time) (string, time.Time) {
	exp := now.Add(liveTokenTTL).Unix()
	tok := "v1." + base64.RawURLEncoding.EncodeToString([]byte(did)) + "." + strconv.FormatInt(exp, 10) + "." + liveTokenSig(key, did, exp)
	return tok, time.Unix(exp, 0)
}

// verifyLiveToken reports whether token was minted for did and is unexpired.
func verifyLiveToken(key []byte, token, did string, now time.Time) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 4 || parts[0] != "v1" {
		return false
	}
	rawDID, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || string(rawDID) != did {
		return false
	}
	exp, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || now.Unix() > exp {
		return false
	}
	return hmac.Equal([]byte(parts[3]), []byte(liveTokenSig(key, did, exp)))
}

// liveTokenAllows reports whether token lets the caller watch did's
// unpublished stream.
func (s *Server) liveTokenAllows(ctx context.Context, token, did string) bool {
	if token == "" {
		return false
	}
	key, err := s.liveTokenKey(ctx)
	if err != nil {
		return false
	}
	return verifyLiveToken(key, token, did, time.Now())
}
