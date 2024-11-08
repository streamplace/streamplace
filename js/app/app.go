package app

import (
	"embed"
	"encoding/json"
	"io/fs"
)

//go:embed all:dist/**
var AllFiles embed.FS

//go:embed package.json
var pkg []byte

// fetch a static snapshot of the current Aquareum web app
func Files() (fs.FS, error) {
	rootFiles, err := fs.Sub(AllFiles, "dist")
	if err != nil {
		return nil, err
	}
	return rootFiles, nil
}

func PackageJSON() (map[string]any, error) {
	var data map[string]any
	err := json.Unmarshal(pkg, &data)
	if err != nil {
		return nil, err
	}
	return data, nil
}
