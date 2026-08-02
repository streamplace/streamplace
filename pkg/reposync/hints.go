package reposync

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Backoff hint registry tuning.
const (
	// maxHintHosts bounds the registry. A node syncs repos from a few thousand
	// PDS hosts at most, and only the ones actively throttling us are in here,
	// so this is generous; it exists so that a pathological run cannot grow the
	// map without limit.
	maxHintHosts = 512

	// hintMaxAge is how long an observation is allowed to influence a wait. A
	// host that said "come back in an hour" ten minutes ago is not evidence
	// about the next request: rate limit windows roll, deploys finish, and the
	// wait is clamped to [RetryPolicy.MaxDelay] anyway, so a stale hint can only
	// make us sleep the maximum for no reason.
	hintMaxAge = 5 * time.Minute
)

// BackoffHint is what a host told us about when it wants to be asked again.
type BackoffHint struct {
	// Until is the earliest instant the host said it would answer properly.
	Until time.Time
	// Source names the header Until came from, for logging: "retry-after" or
	// "ratelimit-reset".
	Source string
	// Observed is when the response carrying the header arrived.
	Observed time.Time
}

// BackoffHints remembers, per host, the last backoff a host asked for.
//
// It exists because indigo's xrpc client throws response headers away: it
// returns an [xrpc.Error] carrying a status code, a decoded error body if there
// was one, and a RatelimitInfo only when the full ratelimit-* header set was
// present. It never looks at Retry-After at all. In production every observed
// 429 arrived with Ratelimit nil, so every retry fell back to a guessed ladder
// while the host was telling us exactly how long to wait.
//
// The fix is to watch the responses ourselves: install [BackoffHints.Transport]
// on the http.Client the xrpc.Client uses, point a [RetryPolicy] at the same
// registry, and a retry waits for what the host asked for instead of guessing.
// The two halves are deliberately decoupled -- the transport sees hosts, not
// repos, and the policy reads a host key -- so nothing has to thread a response
// through the walker.
//
// A nil *BackoffHints is a working no-op registry, so a policy without one
// behaves exactly as it did before.
type BackoffHints struct {
	mu    sync.Mutex
	hints map[string]BackoffHint
}

// NewBackoffHints returns an empty registry.
func NewBackoffHints() *BackoffHints {
	return &BackoffHints{hints: map[string]BackoffHint{}}
}

// HostKey normalizes a PDS base URL ("https://porcini.example.net/") or a bare
// host ("porcini.example.net") to the key the registry uses. It is exported
// because callers that shard work per PDS want to agree with the registry about
// what one host is.
func HostKey(hostOrURL string) string {
	s := strings.TrimSpace(hostOrURL)
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	return strings.ToLower(s)
}

// Observe records what host's headers say about backing off, if anything. Only
// 429 and 503 responses are interesting: ratelimit-* headers ride along on
// perfectly good responses too, and treating those as a hint would throttle a
// healthy walk.
func (h *BackoffHints) Observe(host string, status int, header http.Header) {
	h.observeAt(host, status, header, time.Now())
}

func (h *BackoffHints) observeAt(host string, status int, header http.Header, at time.Time) {
	if h == nil {
		return
	}
	switch status {
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
	default:
		return
	}
	key := HostKey(host)
	if key == "" {
		return
	}
	until, source := parseBackoffHeaders(header, at)
	if until.IsZero() || !until.After(at) {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.hints == nil {
		h.hints = map[string]BackoffHint{}
	}
	if _, ok := h.hints[key]; !ok {
		h.makeRoom(at)
	}
	h.hints[key] = BackoffHint{Until: until, Source: source, Observed: at}
}

// makeRoom keeps the map bounded, on the insert path so that no goroutine has to
// exist to do it: drop everything expired, and if that was not enough, drop the
// least recently observed host.
func (h *BackoffHints) makeRoom(now time.Time) {
	if len(h.hints) < maxHintHosts {
		return
	}
	for key, hint := range h.hints {
		if hintExpired(hint, now) {
			delete(h.hints, key)
		}
	}
	for len(h.hints) >= maxHintHosts {
		oldestKey, oldest := "", time.Time{}
		for key, hint := range h.hints {
			if oldest.IsZero() || hint.Observed.Before(oldest) {
				oldestKey, oldest = key, hint.Observed
			}
		}
		delete(h.hints, oldestKey)
	}
}

// Get returns the live hint for host, if there is one.
func (h *BackoffHints) Get(host string) (BackoffHint, bool) {
	return h.get(host, time.Now())
}

func (h *BackoffHints) get(host string, now time.Time) (BackoffHint, bool) {
	if h == nil {
		return BackoffHint{}, false
	}
	key := HostKey(host)
	if key == "" {
		return BackoffHint{}, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	hint, ok := h.hints[key]
	if !ok || hintExpired(hint, now) {
		return BackoffHint{}, false
	}
	return hint, true
}

// Len is the number of hosts currently remembered, live or not.
func (h *BackoffHints) Len() int {
	if h == nil {
		return 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.hints)
}

func hintExpired(hint BackoffHint, now time.Time) bool {
	return !hint.Until.After(now) || now.Sub(hint.Observed) > hintMaxAge
}

// parseBackoffHeaders reads the two ways a host says "not yet": Retry-After, in
// either of its RFC 9110 forms (delay seconds or an HTTP-date), and
// ratelimit-reset, which the atproto reference implementation sends as unix
// seconds. When both are present the later one wins -- they are both promises
// about when the next request can succeed, and the longer promise is the one
// that has to hold.
func parseBackoffHeaders(header http.Header, at time.Time) (time.Time, string) {
	var until time.Time
	source := ""
	consider := func(t time.Time, name string) {
		if t.IsZero() || !t.After(until) {
			return
		}
		until, source = t, name
	}
	if v := strings.TrimSpace(header.Get("Retry-After")); v != "" {
		if secs, err := strconv.ParseInt(v, 10, 64); err == nil {
			if secs > 0 {
				consider(at.Add(time.Duration(secs)*time.Second), "retry-after")
			}
		} else if date, err := http.ParseTime(v); err == nil {
			consider(date, "retry-after")
		}
	}
	if v := strings.TrimSpace(header.Get("ratelimit-reset")); v != "" {
		if secs, err := strconv.ParseInt(v, 10, 64); err == nil && secs > 0 {
			consider(time.Unix(secs, 0), "ratelimit-reset")
		}
	}
	return until, source
}

// Transport wraps inner so that every throttled or unavailable response it sees
// lands in the registry. Nothing else about the request or response changes; in
// particular the body is untouched, so this is safe to install under any client.
//
// A nil inner means http.DefaultTransport, matching net/http.
func (h *BackoffHints) Transport(inner http.RoundTripper) http.RoundTripper {
	if inner == nil {
		inner = http.DefaultTransport
	}
	if h == nil {
		return inner
	}
	return &hintTransport{inner: inner, hints: h}
}

type hintTransport struct {
	inner http.RoundTripper
	hints *BackoffHints
}

var _ http.RoundTripper = (*hintTransport)(nil)

func (t *hintTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.inner.RoundTrip(req)
	if err != nil || resp == nil {
		return resp, err
	}
	t.hints.Observe(req.URL.Host, resp.StatusCode, resp.Header)
	return resp, err
}
