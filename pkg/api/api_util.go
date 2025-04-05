package api

import "net/http"

// get rendition from query params, defaulting to "source"
func getRendition(r *http.Request) string {
	rendition := r.URL.Query().Get("rendition")
	if rendition == "" {
		rendition = "source"
	}
	return rendition
}
