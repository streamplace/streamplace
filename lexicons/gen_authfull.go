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
	overlay := flag.String("overlay", "pkg/spaces/lexicons/place.stream.authFull.json", "file to write the spaces permission overlay to")
	declaration := flag.String("space", "pkg/spaces/lexicons/place.stream.personal.json", "space declaration")
	flag.Parse()

	lexs, err := lexicons.PlaceStream(lexicons.AllFiles)
	if err != nil {
		panic(err)
	}
	authFull := lexicons.AuthFull(lexs)
	writeJSON(*output, authFull)

	bs, err := os.ReadFile(*declaration)
	if err != nil {
		panic(err)
	}
	var space struct {
		ID   string `json:"id"`
		Defs struct {
			Main struct {
				Collections []string `json:"collections"`
			} `json:"main"`
		} `json:"defs"`
	}
	if err := json.Unmarshal(bs, &space); err != nil {
		panic(err)
	}
	bs, err = json.Marshal(authFull)
	if err != nil {
		panic(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(bs, &doc); err != nil {
		panic(err)
	}
	main := doc["defs"].(map[string]any)["main"].(map[string]any)
	main["permissions"] = append(main["permissions"].([]any), map[string]any{
		"type": "permission", "resource": "space", "spaceType": space.ID,
		"authority": "self", "collection": space.Defs.Main.Collections,
		"action": []string{"read", "create", "update", "delete"}, "manage": []string{"create"},
	})
	writeJSON(*overlay, doc)
}

func writeJSON(path string, doc any) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		panic(err)
	}
}
