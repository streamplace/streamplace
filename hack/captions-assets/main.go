// Captions build downloads: content-addressed cache, verified before embedding.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type asset struct{ name, url, hash string }

var assets = []asset{
	{"ggml-tiny-q5_1.bin", "https://huggingface.co/ggerganov/whisper.cpp/resolve/5359861c739e955e79d9a303bcbc70fb988958b1/ggml-tiny-q5_1.bin", "818710568da3ca15689e31a743197b520007872ff9576237bda97bd1b469c3d7"},
	{"ggml-base-q5_1.bin", "https://huggingface.co/ggerganov/whisper.cpp/resolve/5359861c739e955e79d9a303bcbc70fb988958b1/ggml-base-q5_1.bin", "422f1ae452ade6f30a004d7e5c6a43195e4433bc370bf23fac9cc591f01a8898"},
	{"ggml-small-q5_1.bin", "https://huggingface.co/ggerganov/whisper.cpp/resolve/5359861c739e955e79d9a303bcbc70fb988958b1/ggml-small-q5_1.bin", "ae85e4a935d7a567bd102fe55afc16bb595bdb618e11b2fc7591bc08120411bb"},
	{"ggml-silero-v5.1.2.bin", "https://huggingface.co/ggml-org/whisper-vad/resolve/main/ggml-silero-v5.1.2.bin", "29940d98d42b91fbd05ce489f3ecf7c72f0a42f027e4875919a28fb4c04ea2cf"},
	{"jfk.wav", "https://raw.githubusercontent.com/ggml-org/whisper.cpp/4979e04f5dcaccb36057e059bbaed8a2f5288315/samples/jfk.wav", "59dfb9a4acb36fe2a2affc14bacbee2920ff435cb13cc314a08c13f66ba7860e"},
}

func valid(path, hash string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return false
	}
	return hex.EncodeToString(h.Sum(nil)) == hash
}
func fetch(root string, a asset) error {
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	cache = filepath.Join(cache, "streamplace", "captions", a.hash)
	dest := filepath.Join(root, "pkg/stt/assets", a.name)
	if valid(dest, a.hash) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(cache), 0755); err != nil {
		return err
	}
	if !valid(cache, a.hash) {
		client := &http.Client{Timeout: 10 * time.Minute}
		resp, err := client.Get(a.url)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("%s: HTTP %d", a.name, resp.StatusCode)
		}
		f, err := os.CreateTemp(filepath.Dir(cache), "download-")
		if err != nil {
			return err
		}
		defer os.Remove(f.Name())
		_, copyErr := io.Copy(f, resp.Body)
		err = f.Close()
		if copyErr != nil {
			return copyErr
		}
		if err != nil {
			return err
		}
		if !valid(f.Name(), a.hash) {
			return fmt.Errorf("%s: SHA256 mismatch", a.name)
		}
		if err = os.Rename(f.Name(), cache); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	// Hard links share the cached bytes, with a portable copy fallback across filesystems.
	_ = os.Remove(dest)
	if os.Link(cache, dest) == nil {
		return nil
	}
	src, err := os.Open(cache)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(dest)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, src)
	closeErr := dst.Close()
	if err != nil {
		return err
	}
	return closeErr
}
func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	for _, a := range assets {
		if err := fetch(root, a); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, "pkg/licenses/attributions.txt"))
	if err == nil {
		err = os.WriteFile(filepath.Join(root, "pkg/stt/assets/licenses.txt"), data, 0644)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
