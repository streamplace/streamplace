package spxrpc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUniqueURIs(t *testing.T) {
	one := " at://a "
	require.Equal(t, []string{"at://a", "at://b"}, uniqueURIs(&one, []string{"at://b", "", "at://a"}))
	require.Nil(t, uniqueURIs(nil, nil))
}

func TestVideoOwner(t *testing.T) {
	did, err := videoOwner("at://did:plc:abc/place.stream.video/3k")
	require.NoError(t, err)
	require.Equal(t, "did:plc:abc", did)
	_, err = videoOwner("at://did:plc:abc/place.stream.livestream/3k")
	require.Error(t, err)
	_, err = videoOwner("nope")
	require.Error(t, err)
}
