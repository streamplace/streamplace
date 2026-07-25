package s3

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseURL(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		bucket, key, err := ParseURL("s3://my-bucket/path/to/object.mp4")
		require.NoError(t, err)
		require.Equal(t, "my-bucket", bucket)
		require.Equal(t, "path/to/object.mp4", key)
	})

	t.Run("simple key", func(t *testing.T) {
		bucket, key, err := ParseURL("s3://b/k")
		require.NoError(t, err)
		require.Equal(t, "b", bucket)
		require.Equal(t, "k", key)
	})

	t.Run("missing scheme", func(t *testing.T) {
		_, _, err := ParseURL("/just/a/path")
		require.Error(t, err)
	})

	t.Run("missing key", func(t *testing.T) {
		_, _, err := ParseURL("s3://only-bucket")
		require.Error(t, err)
	})

	t.Run("empty key", func(t *testing.T) {
		_, _, err := ParseURL("s3://bucket/")
		require.Error(t, err)
	})

	t.Run("missing bucket", func(t *testing.T) {
		_, _, err := ParseURL("s3:///key")
		require.Error(t, err)
	})
}
