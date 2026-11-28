//go:build !linux || !cgo || !stationslate

package stationshard

import (
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/s3"
)

func TestUnlinkedShardsDoNotFallBack(t *testing.T) {
	store, err := Open(Config{Owner: "node", S3: s3.Config{
		Endpoint: "http://127.0.0.1", Bucket: "unused", AccessKeyID: "test", SecretAccessKey: "test",
	}})
	require.Nil(t, store)
	require.ErrorContains(t, err, "stationslate")
}
