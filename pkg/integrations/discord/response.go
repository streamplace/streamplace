package discord

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// maxResponseBodyLogBytes caps how much of an external webhook response body
// we read for logging and error messages. Webhook endpoints are
// user-configured, so their bodies are untrusted input.
const maxResponseBodyLogBytes = 1024

// readResponseBody reads a bounded, log-safe rendering of an external webhook
// response body: truncated to maxResponseBodyLogBytes with control characters
// stripped so external content can't disrupt log parsing.
func readResponseBody(r io.Reader) (string, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxResponseBodyLogBytes))
	if err != nil {
		return "", err
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, string(body)), nil
}

type webhookRateLimitError struct{ retryAt time.Time }

func (e *webhookRateLimitError) Error() string      { return ErrWebhookRateLimited.Error() }
func (e *webhookRateLimitError) Unwrap() error      { return ErrWebhookRateLimited }
func (e *webhookRateLimitError) RetryAt() time.Time { return e.retryAt }

func webhookRetryAt(resp *http.Response, now time.Time) time.Time {
	const maxDelaySeconds = (1<<63 - 1) / int64(time.Second)
	var at time.Time
	header := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if seconds, err := strconv.ParseUint(header, 10, 64); err == nil && seconds <= uint64(maxDelaySeconds) {
		at = now.Add(time.Duration(seconds) * time.Second)
	} else if date, err := http.ParseTime(header); err == nil {
		at = date
	}

	// Discord can supply fractional seconds in the response body.
	var hint struct {
		RetryAfter float64 `json:"retry_after"`
	}
	if body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyLogBytes)); err == nil &&
		json.Unmarshal(body, &hint) == nil && hint.RetryAfter > 0 && hint.RetryAfter < float64(maxDelaySeconds) {
		if until := now.Add(time.Duration(hint.RetryAfter * float64(time.Second))); until.After(at) {
			at = until
		}
	}
	if !at.After(now) {
		at = now.Add(5 * time.Second)
	}
	return at.UTC()
}
