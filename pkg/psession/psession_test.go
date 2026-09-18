package psession

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSessionRoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	s, sid := Mint(key, "did:plc:streamer", ScopePublic, time.Hour, now)
	require.Equal(t, s.ID, ID(sid))
	require.Len(t, s.ID, 13, "a TID")

	got, err := Parse(key, sid, "did:plc:streamer", now.Add(30*time.Minute))
	require.NoError(t, err)
	require.Equal(t, s, got)

	// Another stream, another key, a tampered scope: all refused.
	_, err = Parse(key, sid, "did:plc:other", now)
	require.ErrorIs(t, err, ErrSignature)
	_, err = Parse([]byte("another key another key another"), sid, "did:plc:streamer", now)
	require.ErrorIs(t, err, ErrSignature)
	tampered := s.ID + ".o." + sid[len(s.ID)+5:]
	_, err = Parse(key, "1."+tampered, "did:plc:streamer", now)
	require.ErrorIs(t, err, ErrSignature)

	// Expired: reported as such, with the session so it can be renewed.
	late, err := Parse(key, sid, "did:plc:streamer", now.Add(2*time.Hour))
	require.ErrorIs(t, err, ErrExpired)
	require.Equal(t, s.ID, late.ID)
	renewed, sid2 := Renew(key, late, "did:plc:streamer", time.Hour, now.Add(2*time.Hour))
	require.Equal(t, s.ID, renewed.ID, "renewal keeps the id")
	require.NotEqual(t, sid, sid2)
	_, err = Parse(key, sid2, "did:plc:streamer", now.Add(2*time.Hour+time.Minute))
	require.NoError(t, err)
	require.Equal(t, s.ID, ID(sid2))

	// Garbage and legacy bare TIDs are malformed, not signed.
	for _, bad := range []string{"", "nope", s.ID, "2." + sid[2:], "1.x.p.1.sig"} {
		_, err := Parse(key, bad, "did:plc:streamer", now)
		require.ErrorIs(t, err, ErrMalformed, bad)
	}
	require.Equal(t, s.ID, ID(s.ID), "a bare id is its own id")
}

func TestOwnerScope(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	now := time.Now()
	_, sid := Mint(key, "did:plc:streamer", ScopeOwner, time.Hour, now)
	s, err := Parse(key, sid, "did:plc:streamer", now)
	require.NoError(t, err)
	require.Equal(t, ScopeOwner, s.Scope)
}
