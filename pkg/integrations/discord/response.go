package discord

import (
	"io"
	"strings"
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
