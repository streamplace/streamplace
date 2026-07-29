package reposync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	"github.com/bluesky-social/indigo/xrpc"
	"stream.place/streamplace/pkg/log"
)

// Retry defaults. A walk of a large repo is 500-1100 sequential getBlocks calls
// at [DefaultChunkSize], so a single 429 or a single restarting PDS must not be
// able to kill it.
const (
	// DefaultMaxAttempts is the total number of tries (not extra tries) a
	// request gets before its error is returned.
	DefaultMaxAttempts = 5
	// DefaultRetryBaseDelay is the wait after the first failure; it doubles
	// from there.
	DefaultRetryBaseDelay = time.Second
	// DefaultRetryMaxDelay caps the wait between attempts, including waits
	// derived from a server's ratelimit-reset header. Sleeping longer than this
	// is worse than failing: backfills serialize their fetches per PDS, so a
	// long sleep here stalls every other repo on that host, and a repo whose
	// backfill fails is simply retried later.
	DefaultRetryMaxDelay = 30 * time.Second
)

// RetryPolicy bounds how hard a fetcher retries a transient XRPC failure.
// The zero value means the defaults above.
type RetryPolicy struct {
	// MaxAttempts is the total number of tries. Zero means
	// [DefaultMaxAttempts]; a value of 1 disables retrying.
	MaxAttempts int
	// BaseDelay is the wait after the first failure. Zero means
	// [DefaultRetryBaseDelay].
	BaseDelay time.Duration
	// MaxDelay caps every wait. Zero means [DefaultRetryMaxDelay].
	MaxDelay time.Duration
	// Hints, when set, is where waits come from whenever a host has said what
	// it wants: see [BackoffHints]. Nil means the ladder plus whatever indigo
	// happened to parse onto the error.
	Hints *BackoffHints
	// Host is the PDS these calls go to -- a base URL or a bare host, either
	// way -- and the key into Hints. Fetchers fill it in from their client, so
	// callers only have to set Hints.
	Host string
}

// forHost returns p keyed to host, leaving an explicitly set Host alone.
func (p RetryPolicy) forHost(host string) RetryPolicy {
	if p.Host == "" {
		p.Host = host
	}
	return p
}

func (p RetryPolicy) withDefaults() RetryPolicy {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = DefaultMaxAttempts
	}
	if p.BaseDelay <= 0 {
		p.BaseDelay = DefaultRetryBaseDelay
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = DefaultRetryMaxDelay
	}
	if p.MaxDelay < p.BaseDelay {
		p.MaxDelay = p.BaseDelay
	}
	return p
}

// hintPad is added to a wait derived from a server's clock: the second it named
// has to have actually elapsed by the time we ask again.
const hintPad = 250 * time.Millisecond

// delay is how long to wait after the attempt'th failure (1-based), and where
// that wait came from ("" for the computed ladder).
//
// Exponential from BaseDelay, capped at MaxDelay, then scaled by a random
// factor in [0.75, 1) so that a fleet of workers that hit the same rate limit
// does not march back in lockstep. Jitter is multiplicative rather than
// additive so the result never exceeds MaxDelay, including at the cap.
//
// If the server said when to come back, and that is further out than the
// computed backoff, wait for what it said instead -- still clamped to MaxDelay,
// see the note there. There are two places that can come from: the ratelimit-*
// headers indigo parsed onto the error, and [BackoffHints], which is our own
// record of every header indigo discarded (notably Retry-After, which it never
// reads). Whichever reaches further out wins.
func (p RetryPolicy) delay(attempt int, err error) (time.Duration, string) {
	p = p.withDefaults()
	d := p.MaxDelay
	if attempt >= 1 && attempt < 31 {
		if shifted := p.BaseDelay << (attempt - 1); shifted > 0 && shifted < p.MaxDelay {
			d = shifted
		}
	}
	d = time.Duration(float64(d) * (0.75 + 0.25*rand.Float64())) //nolint:gosec // jitter, not crypto

	until, source := ratelimitReset(err), "ratelimit-reset"
	if hint, ok := p.Hints.Get(p.Host); ok && hint.Until.After(until) {
		until, source = hint.Until, hint.Source
	}
	if !until.IsZero() {
		if wait := time.Until(until) + hintPad; wait > d {
			return min(wait, p.MaxDelay), source
		}
	}
	return d, ""
}

// do runs fn until it succeeds, fails with something not worth retrying, or
// runs out of attempts. what names the call for logging only; the error
// returned is fn's, unwrapped when it was not retryable and wrapped with the
// attempt count when the budget ran out.
func (p RetryPolicy) do(ctx context.Context, what string, fn func() error) error {
	p = p.withDefaults()
	for attempt := 1; ; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		if !isRetryable(err) {
			return err
		}
		if attempt >= p.MaxAttempts {
			return fmt.Errorf("giving up after %d attempts: %w", attempt, err)
		}
		d, source := p.delay(attempt, err)
		kv := []any{"call", what, "attempt", attempt, "wait", d}
		if source != "" {
			// Worth saying out loud: it is the difference between "we guessed"
			// and "the host told us", which is the first thing an operator
			// looking at a throttled sweep wants to know.
			kv = append(kv, "waitSource", source)
		}
		kv = append(kv, "err", errForLog(err))
		log.Warn(ctx, "retrying transient xrpc failure", kv...)
		if serr := sleepCtx(ctx, d); serr != nil {
			return fmt.Errorf("aborted after %d attempts: %w", attempt, errors.Join(err, serr))
		}
	}
}

// sleepCtx waits for d, or returns the context's error as soon as it is done.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// errForLog renders a retryable failure for the warning line above.
//
// It exists for one shape: a host that throttles or 502s us with an HTML error
// page. indigo tries to JSON-decode every non-200 body, so what surfaces is
// `XRPC ERROR 429: failed to decode xrpc error message: invalid character '<'
// looking for beginning of value` -- forty characters of JSON parser trivia in
// front of the one fact that matters, repeated for every retry of every walk.
// Say "HTTP 429 (undecodable error body)" instead. Only this log line is
// compressed; the error returned to the caller keeps the whole chain.
func errForLog(err error) any {
	var xe *xrpc.Error
	if !errors.As(err, &xe) || xe.StatusCode == 0 || !isUndecodableBody(xe.Wrapped) {
		return err
	}
	return fmt.Sprintf("HTTP %d (undecodable error body)", xe.StatusCode)
}

// undecodableBodyPrefix is indigo's wrapper around a response body that is not
// the JSON error object the lexicon promises.
const undecodableBodyPrefix = "failed to decode xrpc error message"

func isUndecodableBody(err error) bool {
	if err == nil {
		return false
	}
	var xe *xrpc.XRPCError
	if errors.As(err, &xe) {
		// A decoded (if empty) error object: the host answered properly.
		return false
	}
	return strings.HasPrefix(err.Error(), undecodableBodyPrefix)
}

// isRetryable reports whether err is the kind of failure that is likely to go
// away on its own: the host throttled us, the host is briefly broken, or the
// connection died under us.
//
// Everything else -- 4xx (including "Could not find cids", which means the repo
// moved and needs a new head, not a retry), block verification failures, a
// cancelled context -- fails fast.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	// A cancelled context can surface as a *url.Error, which would otherwise
	// look like a transport blip; check it first.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var xe *xrpc.Error
	if errors.As(err, &xe) {
		switch {
		case xe.StatusCode == http.StatusTooManyRequests:
			return true
		case xe.StatusCode == http.StatusNotImplemented:
			// 5xx numerically, but it is a permanent answer: this host does
			// not implement the method, and the caller wants to hear that
			// immediately so it can fall back.
			return false
		case xe.StatusCode >= 500 && xe.StatusCode <= 599:
			return true
		}
		return false
	}
	// No HTTP response at all.
	var nerr net.Error
	if errors.As(err, &nerr) && nerr.Timeout() {
		return true
	}
	return errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, io.EOF)
}

// ratelimitReset pulls the reset time out of an XRPC error, if the host sent
// ratelimit-* headers. indigo parses those into xrpc.Error.Ratelimit; note it
// only does so when a ratelimit-limit header is present, and it does not look
// at Retry-After at all, so this is often zero even for a 429.
func ratelimitReset(err error) time.Time {
	var xe *xrpc.Error
	if !errors.As(err, &xe) || xe.Ratelimit == nil {
		return time.Time{}
	}
	return xe.Ratelimit.Reset
}
