package remote

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Streamplace team can add new files here with hack/upload-fixture.sh

var fixtureURL = "https://storage.googleapis.com/streamplace-fixtures"

func RemoteFixture(name string) string {
	parts := strings.Split(name, "/")
	if len(parts) != 2 {
		panic("fixture name must be in format HASH/FILENAME")
	}
	expectedHash := parts[0]
	filename := parts[1]

	// Check if file already exists in cache
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	cacheDir := filepath.Join(homeDir, ".streamplace-test-cache")
	finalPath := filepath.Join(cacheDir, expectedHash, filename)
	if _, err := os.Stat(finalPath); err == nil {
		return finalPath
	}

	// Create temp dir if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		panic(err)
	}

	// Download to temporary file
	resp, err := http.Get(fixtureURL + "/" + name)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	tmpFile, err := os.CreateTemp(cacheDir, "download-*")
	if err != nil {
		panic(err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Calculate hash while downloading
	hash := sha256.New()
	writer := io.MultiWriter(tmpFile, hash)

	if _, err := io.Copy(writer, resp.Body); err != nil {
		panic(err)
	}
	tmpFile.Close()

	// Verify hash
	actualHash := hex.EncodeToString(hash.Sum(nil))
	if actualHash != expectedHash {
		panic(fmt.Sprintf("hash mismatch: expected %s, got %s", expectedHash, actualHash))
	}

	// Move to final location
	finalDir := filepath.Join(cacheDir, expectedHash)
	if err := os.MkdirAll(finalDir, 0755); err != nil {
		panic(err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		panic(err)
	}

	return finalPath
}

// takes a tarball, returns a directory with the contents
func RemoteArchive(name string) string {
	fpath := RemoteFixture(name)

	// Create extracted directory adjacent to the archive file
	dir := filepath.Dir(fpath)
	extractedDir := filepath.Join(dir, "extracted")

	if err := os.MkdirAll(extractedDir, 0755); err != nil {
		panic(err)
	}

	// Extract the tarball contents into the directory
	file, err := os.Open(fpath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Create gzip reader
	gzr, err := gzip.NewReader(file)
	if err != nil {
		panic(err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}

		target := filepath.Join(extractedDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				panic(err)
			}
		case tar.TypeReg:
			// Create parent directories if needed
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				panic(err)
			}

			outFile, err := os.Create(target)
			if err != nil {
				panic(err)
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				panic(err)
			}
			outFile.Close()
		}
	}

	return extractedDir
}
