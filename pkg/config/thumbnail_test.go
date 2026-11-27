package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestThumbnailWriteAndRead(t *testing.T) {
	cli := &CLI{DataDir: t.TempDir()}
	const user = "did:plc:abc123"

	// No thumbnail exists yet.
	_, ok := cli.ThumbnailModTime(user, true)
	require.False(t, ok)

	require.NoError(t, cli.ThumbnailWrite(user, true, func(w io.Writer) error {
		_, err := io.WriteString(w, "first")
		return err
	}))

	// Path lives under the thumbnails dir, colon-sanitized, with a .jpg suffix.
	fpath := cli.ThumbnailFilePath(user, true)
	require.Equal(t, filepath.Join(cli.DataDir, ThumbnailsDir, "did-plc-abc123.jpg"), fpath)
	// DIDs contain colons, which Windows forbids in filenames, so the filename
	// must not carry them through.
	require.NotContains(t, filepath.Base(fpath), ":")

	data, err := os.ReadFile(fpath)
	require.NoError(t, err)
	require.Equal(t, "first", string(data))

	mt, ok := cli.ThumbnailModTime(user, true)
	require.True(t, ok)
	require.WithinDuration(t, time.Now(), mt, 5*time.Second)

	// A second write overwrites in place...
	require.NoError(t, cli.ThumbnailWrite(user, true, func(w io.Writer) error {
		_, err := io.WriteString(w, "second")
		return err
	}))
	data, err = os.ReadFile(fpath)
	require.NoError(t, err)
	require.Equal(t, "second", string(data))

	// ...and leaves no temp files behind.
	requireOnlyThumbnail(t, cli, "did-plc-abc123.jpg")

	// A failed write keeps the previous thumbnail intact and cleans up its temp.
	require.Error(t, cli.ThumbnailWrite(user, true, func(w io.Writer) error {
		return fmt.Errorf("boom")
	}))
	data, err = os.ReadFile(fpath)
	require.NoError(t, err)
	require.Equal(t, "second", string(data))
	requireOnlyThumbnail(t, cli, "did-plc-abc123.jpg")
}

// Pre-live frames never land in the public thumbnail; go-live picks the newest.
func TestPreliveThumbnail(t *testing.T) {
	cli := &CLI{DataDir: t.TempDir()}
	const user = "did:plc:abc123"
	write := func(published bool, body string) {
		require.NoError(t, cli.ThumbnailWrite(user, published, func(w io.Writer) error {
			_, err := io.WriteString(w, body)
			return err
		}))
	}

	write(false, "prelive")
	_, ok := cli.ThumbnailModTime(user, true)
	require.False(t, ok, "no public thumbnail from a pre-live frame")
	require.Equal(t, cli.ThumbnailFilePath(user, false), cli.LatestThumbnailFilePath(user))

	write(true, "public")
	require.NoError(t, os.Chtimes(cli.ThumbnailFilePath(user, false), time.Now().Add(-time.Hour), time.Now().Add(-time.Hour)))
	require.Equal(t, cli.ThumbnailFilePath(user, true), cli.LatestThumbnailFilePath(user))
	write(false, "prelive again")
	require.Equal(t, cli.ThumbnailFilePath(user, false), cli.LatestThumbnailFilePath(user))
}

func requireOnlyThumbnail(t *testing.T, cli *CLI, name string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(cli.DataDir, ThumbnailsDir))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, name, entries[0].Name())
}
