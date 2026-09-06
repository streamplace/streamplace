package bunny

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The expected tokens were computed independently with bunny's
// published reference algorithm (sha256(key + path + expires + sorted
// "k=v" params), base64url, no padding), so a drift here means bunny
// would 403 every segment.
func TestSignerMatchesReference(t *testing.T) {
	s := Signer{Key: "secret-key"}
	exp := time.Unix(1700000000, 0)

	u, _ := url.Parse("https://cdn.example.com/blobs/bafyblob.mp4?did=did%3Aplc%3Aabc&sid=tid123")
	require.Equal(t,
		"https://cdn.example.com/blobs/bafyblob.mp4?token=KIxqgRCnXmqKj07FcqY4y6jxhYQ47kyCj2QLdC-wlo4&did=did%3Aplc%3Aabc&sid=tid123&expires=1700000000",
		s.SignURL(u, exp))

	// A path prefix is part of the signed path; an absent sid is left
	// out of both the hash and the URL.
	u, _ = url.Parse("https://cdn.example.com/vods/blobs/bafyblob.mp4?did=did%3Aplc%3Aabc")
	require.Equal(t,
		"https://cdn.example.com/vods/blobs/bafyblob.mp4?token=hSHnZu2PJHcclObmqnsI66szGuzpeePfHt_rYt327Fk&did=did%3Aplc%3Aabc&expires=1700000000",
		s.SignURL(u, exp))
}

func TestSignerTokenBoundToPath(t *testing.T) {
	s := Signer{Key: "secret-key"}
	exp := time.Unix(1700000000, 0)
	a, _ := url.Parse("https://cdn.example.com/blobs/bafyblob.mp4?did=x")
	b, _ := url.Parse("https://cdn.example.com/blobs/bafyother.mp4?did=x")
	require.NotEqual(t, s.SignURL(a, exp), s.SignURL(b, exp))
	// The input URL is not mutated.
	require.Equal(t, "did=x", a.RawQuery)
}
