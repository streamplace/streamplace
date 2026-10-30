//go:build ignore

// Materializes place.stream.authFull from the place.stream record lexicons.
// Run via `make lexicons`, which does so before any other codegen so the Go,
// JS, and docs outputs include it. Builds nothing but the lexicon bundle.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"

	"stream.place/streamplace/lexicons"
)

func main() {
	output := flag.String("o", "lexicons/place/stream/authFull.json", "file to write the generated lexicon to")
	flag.Parse()

	lexs, err := lexicons.PlaceStream(lexicons.AllFiles)
	if err != nil {
		panic(err)
	}
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(lexicons.AuthFull(lexs)); err != nil {
		panic(err)
	}
	if err := os.WriteFile(*output, buf.Bytes(), 0644); err != nil {
		panic(err)
	}
}
