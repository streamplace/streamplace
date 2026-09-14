package spxrpc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLiveToken(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	tok, exp := mintLiveToken(key, "did:plc:abc", now)
	require.Equal(t, now.Add(liveTokenTTL).Unix(), exp.Unix())
	require.True(t, verifyLiveToken(key, tok, "did:plc:abc", now))
	require.True(t, verifyLiveToken(key, tok, "did:plc:abc", now.Add(59*time.Minute)))
	require.False(t, verifyLiveToken(key, tok, "did:plc:abc", now.Add(61*time.Minute)), "expired")
	require.False(t, verifyLiveToken(key, tok, "did:plc:other", now), "bound to the DID")
	require.False(t, verifyLiveToken([]byte("another key............................"), tok, "did:plc:abc", now), "wrong key")
	require.False(t, verifyLiveToken(key, tok+"x", "did:plc:abc", now), "tampered")
	require.False(t, verifyLiveToken(key, "", "did:plc:abc", now))
}
