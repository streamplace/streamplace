package api

// Packaging a robust set of MIME types from Debian so everything
// doesn't break when we add a new image type to the app or something.

import (
	"bufio"
	"bytes"
	_ "embed"
	"mime"
	"strings"
)

//go:embed mime.types
var mimeTypes []byte

func loadEmbeddedMimes() {
	scanner := bufio.NewScanner(bytes.NewReader(mimeTypes))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) <= 1 || fields[0][0] == '#' {
			continue
		}
		mimeType := fields[0]
		for _, ext := range fields[1:] {
			if ext[0] == '#' {
				break
			}
			if err := mime.AddExtensionType("."+ext, mimeType); err != nil {
				panic(err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
}
