package api

import (
	"io/fs"
	"net/http"
	"strings"

	"stream.place/streamplace/js/app"
	"stream.place/streamplace/pkg/stt"
)

// The source revision versions both the bridge and its bundled model set.
const captionerRevision = "4979e04f5dcaccb36057e059bbaed8a2f5288315"
const captionerBasePath = "/api/captioner/" + captionerRevision + "/"

func captionerAssets() http.Handler {
	wasm, _ := fs.Sub(app.AssetFiles, "assets/whisper/"+captionerRevision)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, captionerBasePath)
		var files fs.FS
		switch name {
		case "caption-whisper.mjs", "caption-whisper.wasm", "worker.js", "agreement.js":
			files = wasm
		case "ggml-tiny-q5_1.bin", "ggml-base-q5_1.bin", "ggml-small-q5_1.bin", "ggml-silero-v5.1.2.bin":
			files = stt.ModelFiles
		default:
			http.NotFound(w, r)
			return
		}
		if files == nil {
			http.Error(w, "Browser caption assets are not bundled in this build", http.StatusServiceUnavailable)
			return
		}
		if _, err := fs.Stat(files, name); err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Embedder-Policy", "credentialless")
		if strings.HasSuffix(name, ".wasm") {
			w.Header().Set("Content-Type", "application/wasm")
		} else if strings.HasSuffix(name, ".mjs") || strings.HasSuffix(name, ".js") {
			w.Header().Set("Content-Type", "text/javascript")
		} else {
			w.Header().Set("Content-Type", "application/octet-stream")
		}
		r2 := r.Clone(r.Context())
		u := *r.URL
		u.Path = "/" + name
		r2.URL = &u
		http.FileServer(http.FS(files)).ServeHTTP(w, r2)
	})
}

// Isolation is opt-in: ordinary OAuth popup pages retain their existing policy.
// Captioner signs in with redirects; browser ingest opts in by reloading its URL.
func captionerIsolation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/captioner" || strings.HasPrefix(r.URL.Path, captionerBasePath) || r.URL.Query().Get("deviceCaptions") == "1" {
			w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
			w.Header().Set("Cross-Origin-Embedder-Policy", "credentialless")
		}
		next.ServeHTTP(w, r)
	})
}
