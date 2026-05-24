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
	_, ok := cli.ThumbnailModTime(user)
	require.False(t, ok)

	require.NoError(t, cli.ThumbnailWrite(user, func(w io.Writer) error {
		_, err := io.WriteString(w, "first")
		return err
	}))

	// Path lives under the thumbnails dir, colon-sanitized, with a .jpg suffix.
	fpath := cli.ThumbnailFilePath(user)
	require.Equal(t, filepath.Join(cli.DataDir, ThumbnailsDir, "did-plc-abc123.jpg"), fpath)

	data, err := os.ReadFile(fpath)
	require.NoError(t, err)
	require.Equal(t, "first", string(data))

	mt, ok := cli.ThumbnailModTime(user)
	require.True(t, ok)
	require.WithinDuration(t, time.Now(), mt, 5*time.Second)

	// A second write overwrites in place...
	require.NoError(t, cli.ThumbnailWrite(user, func(w io.Writer) error {
		_, err := io.WriteString(w, "second")
		return err
	}))
	data, err = os.ReadFile(fpath)
	require.NoError(t, err)
	require.Equal(t, "second", string(data))

	// ...and leaves no temp files behind.
	requireOnlyThumbnail(t, cli, "did-plc-abc123.jpg")

	// A failed write keeps the previous thumbnail intact and cleans up its temp.
	require.Error(t, cli.ThumbnailWrite(user, func(w io.Writer) error {
		return fmt.Errorf("boom")
	}))
	data, err = os.ReadFile(fpath)
	require.NoError(t, err)
	require.Equal(t, "second", string(data))
	requireOnlyThumbnail(t, cli, "did-plc-abc123.jpg")
}

func requireOnlyThumbnail(t *testing.T, cli *CLI, name string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(cli.DataDir, ThumbnailsDir))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, name, entries[0].Name())
}
