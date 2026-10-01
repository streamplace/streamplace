// Package licenses exposes notices for Streamplace and its bundled speech components.
package licenses

import (
	_ "embed"
	"io"
	"net/http"
)

//go:embed attributions.txt
var Text string

// Handler serves the same notices as streamplace --licenses.
func Handler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, Text)
}
