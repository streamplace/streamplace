package reposync

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
)

// retryAfter is a 429 that says when to come back the way a rate limiter
// actually does: a Retry-After header and no ratelimit-* set at all, which is
// the shape indigo throws away entirely.
func retryAfter(value string) failure {
	return failure{
		status: http.StatusTooManyRequests,
		body:   `{"error":"RateLimitExceeded","message":"Rate Limit Exceeded"}`,
		header: map[string]string{"Content-Type": "application/json", "Retry-After": value},
	}
}

func TestHostKey(t *testing.T) {
	for in, want := range map[string]string{
		"https://porcini.us-east.host.bsky.network":  "porcini.us-east.host.bsky.network",
		"https://porcini.us-east.host.bsky.network/": "porcini.us-east.host.bsky.network",
		"http://PDS.Example:2583/xrpc/whatever":      "pds.example:2583",
		"pds.example":                                "pds.example",
		" pds.example ":                              "pds.example",
		"https://pds.example?x=1":                    "pds.example",
		"":                                           "",
	} {
		require.Equal(t, want, HostKey(in), "HostKey(%q)", in)
	}
}

// TestBackoffHintsObserve covers what the registry makes of a response, without
// any HTTP involved: which statuses count, both Retry-After forms,
// ratelimit-reset, and the junk a host might send instead.
func TestBackoffHintsObserve(t *testing.T) {
	now := time.Now()
	header := func(kv ...string) http.Header {
		h := http.Header{}
		for i := 0; i+1 < len(kv); i += 2 {
			h.Set(kv[i], kv[i+1])
		}
		return h
	}

	for _, tc := range []struct {
		name   string
		status int
		header http.Header
		want   time.Duration // wait recorded, 0 for "nothing recorded"
		source string
	}{
		{"429 retry-after seconds", 429, header("Retry-After", "3"), 3 * time.Second, "retry-after"},
		{"429 retry-after http-date", 429,
			header("Retry-After", now.Add(90*time.Second).UTC().Format(http.TimeFormat)),
			90 * time.Second, "retry-after"},
		{"503 retry-after", 503, header("Retry-After", "12"), 12 * time.Second, "retry-after"},
		{"429 ratelimit-reset", 429,
			header("ratelimit-reset", strconv.FormatInt(now.Add(45*time.Second).Unix(), 10)),
			45 * time.Second, "ratelimit-reset"},
		// Both present: the longer promise is the one that has to hold.
		{"the later of the two wins (reset)", 429,
			header("Retry-After", "5", "ratelimit-reset", strconv.FormatInt(now.Add(60*time.Second).Unix(), 10)),
			60 * time.Second, "ratelimit-reset"},
		{"the later of the two wins (retry-after)", 429,
			header("Retry-After", "60", "ratelimit-reset", strconv.FormatInt(now.Add(5*time.Second).Unix(), 10)),
			60 * time.Second, "retry-after"},
		// Statuses that say nothing about backing off. ratelimit-* headers ride
		// along on healthy responses, and recording those would throttle a walk
		// that is doing fine.
		{"200 is not a backoff", 200, header("ratelimit-reset", strconv.FormatInt(now.Add(60*time.Second).Unix(), 10)), 0, ""},
		{"500 is not a backoff", 500, header("Retry-After", "30"), 0, ""},
		{"400 is not a backoff", 400, header("Retry-After", "30"), 0, ""},
		// Nothing usable in the headers.
		{"429 with no headers", 429, header(), 0, ""},
		{"unparseable retry-after", 429, header("Retry-After", "soon"), 0, ""},
		{"retry-after zero", 429, header("Retry-After", "0"), 0, ""},
		{"negative retry-after", 429, header("Retry-After", "-5"), 0, ""},
		{"retry-after in the past", 429,
			header("Retry-After", now.Add(-time.Hour).UTC().Format(http.TimeFormat)), 0, ""},
		{"reset in the past", 429,
			header("ratelimit-reset", strconv.FormatInt(now.Add(-time.Hour).Unix(), 10)), 0, ""},
		{"reset is not a number", 429, header("ratelimit-reset", "later"), 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewBackoffHints()
			h.observeAt("https://pds.example", tc.status, tc.header, now)
			hint, ok := h.get("pds.example", now)
			if tc.want == 0 {
				require.False(t, ok, "nothing should have been recorded")
				require.Zero(t, h.Len())
				return
			}
			require.True(t, ok)
			require.Equal(t, tc.source, hint.Source)
			require.WithinDuration(t, now.Add(tc.want), hint.Until, 1500*time.Millisecond)
			require.Equal(t, now, hint.Observed)
		})
	}

	t.Run("a hint stops applying once its own deadline passes", func(t *testing.T) {
		h := NewBackoffHints()
		h.observeAt("pds.example", 429, header("Retry-After", "10"), now)
		_, ok := h.get("pds.example", now.Add(9*time.Second))
		require.True(t, ok)
		_, ok = h.get("pds.example", now.Add(11*time.Second))
		require.False(t, ok, "the wait it asked for has elapsed")
	})

	t.Run("a stale observation stops applying however long it asked for", func(t *testing.T) {
		h := NewBackoffHints()
		h.observeAt("pds.example", 429, header("Retry-After", "3600"), now)
		_, ok := h.get("pds.example", now.Add(hintMaxAge-time.Second))
		require.True(t, ok)
		_, ok = h.get("pds.example", now.Add(hintMaxAge+time.Second))
		require.False(t, ok)
	})

	t.Run("an unknown host has no hint", func(t *testing.T) {
		h := NewBackoffHints()
		h.observeAt("pds.example", 429, header("Retry-After", "10"), now)
		_, ok := h.get("other.example", now)
		require.False(t, ok)
		_, ok = h.get("", now)
		require.False(t, ok)
	})

	t.Run("the nil registry is a working no-op", func(t *testing.T) {
		var h *BackoffHints
		h.Observe("pds.example", 429, header("Retry-After", "10"))
		_, ok := h.Get("pds.example")
		require.False(t, ok)
		require.Zero(t, h.Len())
		require.Equal(t, http.DefaultTransport, h.Transport(nil))
	})

	t.Run("the map stays bounded", func(t *testing.T) {
		h := NewBackoffHints()
		// Twice the cap of live hints, all with the same deadline, so nothing
		// can be pruned for being expired and the eviction path has to run.
		for i := 0; i < maxHintHosts*2; i++ {
			h.observeAt(fmt.Sprintf("pds%d.example", i), 429, header("Retry-After", "60"), now)
			require.LessOrEqual(t, h.Len(), maxHintHosts)
		}
		require.Equal(t, maxHintHosts, h.Len())
		// The most recent observation is always the one kept.
		_, ok := h.get(fmt.Sprintf("pds%d.example", maxHintHosts*2-1), now)
		require.True(t, ok)
		// Re-observing a host already in the map does not grow it.
		before := h.Len()
		h.observeAt(fmt.Sprintf("pds%d.example", maxHintHosts*2-1), 429, header("Retry-After", "90"), now)
		require.Equal(t, before, h.Len())
	})
}

// TestBackoffHintsFromTransport is the mechanism end to end: a real HTTP round
// trip through the wrapped transport, the real getBlocks path, and a retry that
// waits for what the host asked for instead of guessing.
func TestBackoffHintsFromTransport(t *testing.T) {
	ctx := context.Background()
	sr := buildSignedRepo(t, testDID, exactnessPaths())

	newFetcher := func(t *testing.T, retry RetryPolicy, script ...failure) (*XRPCBlockFetcher, *fakeHost, *BackoffHints) {
		t.Helper()
		hints := NewBackoffHints()
		host := newFakeHost(sr)
		host.blocksFailures = script
		client := host.start(t)
		client.Client.Transport = hints.Transport(client.Client.Transport)
		retry.Hints = hints
		return &XRPCBlockFetcher{Client: client, DID: testDID, Retry: retry}, host, hints
	}

	// One second is the smallest Retry-After a host can express, and the point
	// of the test is that we really wait it out, so this subtest costs a second.
	t.Run("Retry-After is waited for", func(t *testing.T) {
		f, host, hints := newFetcher(t,
			// A ladder that would retry in a millisecond if left to itself, so
			// the elapsed time can only have come from the header.
			RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Second},
			retryAfter("1"))
		start := time.Now()
		blocks, err := f.GetBlocks(ctx, []cid.Cid{sr.root})
		elapsed := time.Since(start)
		require.NoError(t, err)
		require.Contains(t, blocks, sr.root)
		require.Equal(t, 2, host.requests)
		// A ladder that short-circuits in a millisecond and a response carrying
		// no ratelimit-* headers at all: a wait of a second can only have come
		// from the Retry-After the transport captured.
		require.GreaterOrEqual(t, elapsed, time.Second, "the host asked for a second")
		require.Less(t, elapsed, 4*time.Second, "and not much more than a second")

		// And the hint retires the moment the wait it asked for has elapsed, so
		// the next call to this host starts from the ladder again.
		_, ok := hints.Get(f.Client.Host)
		require.False(t, ok)
	})

	// The rest only need the observation, so they fail fast and never sleep.
	observed := func(t *testing.T, script ...failure) (*BackoffHints, *XRPCBlockFetcher) {
		t.Helper()
		f, host, hints := newFetcher(t, RetryPolicy{MaxAttempts: 1}, script...)
		_, err := f.GetBlocks(ctx, []cid.Cid{sr.root})
		require.Error(t, err)
		require.Equal(t, 1, host.requests)
		return hints, f
	}

	t.Run("Retry-After as an HTTP-date", func(t *testing.T) {
		hints, f := observed(t, retryAfter(time.Now().Add(30*time.Second).UTC().Format(http.TimeFormat)))
		hint, ok := hints.Get(f.Client.Host)
		require.True(t, ok)
		require.Equal(t, "retry-after", hint.Source)
		require.WithinDuration(t, time.Now().Add(30*time.Second), hint.Until, 2*time.Second)
	})

	t.Run("503 Retry-After", func(t *testing.T) {
		hints, f := observed(t, failure{
			status: http.StatusServiceUnavailable,
			body:   "<html>restarting</html>",
			header: map[string]string{"Retry-After": "7"},
		})
		hint, ok := hints.Get(f.Client.Host)
		require.True(t, ok)
		require.Equal(t, "retry-after", hint.Source)
		require.WithinDuration(t, time.Now().Add(7*time.Second), hint.Until, 2*time.Second)
	})

	t.Run("ratelimit-reset without Retry-After", func(t *testing.T) {
		hints, f := observed(t, throttled(time.Now().Add(20*time.Second)))
		hint, ok := hints.Get(f.Client.Host)
		require.True(t, ok)
		require.Equal(t, "ratelimit-reset", hint.Source)
	})

	t.Run("a 429 that says nothing leaves the ladder alone", func(t *testing.T) {
		hints, _ := observed(t, htmlThrottled)
		require.Zero(t, hints.Len())
	})

	t.Run("a successful walk records nothing", func(t *testing.T) {
		f, host, hints := newFetcher(t, RetryPolicy{MaxAttempts: 1})
		_, err := f.GetBlocks(ctx, []cid.Cid{sr.root})
		require.NoError(t, err)
		require.Equal(t, 1, host.requests)
		require.Zero(t, hints.Len())
	})
}
