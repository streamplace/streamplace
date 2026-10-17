package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist/**
var allFiles embed.FS

// Files returns the embedded Vite build output (js/web/dist/).
func Files() (fs.FS, error) {
	rootFiles, err := fs.Sub(allFiles, "dist")
	if err != nil {
		return nil, err
	}
	return rootFiles, nil
}
