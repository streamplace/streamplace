package spxrpc

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/statedb"
)

// An unset text key from the vocabulary reads as its default (empty when it
// has none), so a client polling it (the front door's defaultVideo) sees ""
// for "cleared"; a key outside the vocabulary is a 404, not a 500.
func TestGetBlobUnsetKey(t *testing.T) {
	ctx := context.Background()
	sdb, err := statedb.MakeDB(ctx, &config.CLI{DBURL: ":memory:"}, nil, nil)
	require.NoError(t, err)
	s := &Server{cli: &config.CLI{BroadcasterHost: "example.com"}, statefulDB: sdb}

	r, err := s.handlePlaceStreamBrandingGetBlob(ctx, "did:web:example.com", "defaultVideo")
	require.NoError(t, err, "a vocabulary key with no value reads as empty")
	b, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Empty(t, b)

	_, err = s.handlePlaceStreamBrandingGetBlob(ctx, "did:web:example.com", "noSuchKey")
	var he *echo.HTTPError
	require.True(t, errors.As(err, &he), "got: %v", err)
	require.Equal(t, http.StatusNotFound, he.Code)

	require.NoError(t, sdb.PutBrandingBlob("did:web:example.com", "defaultVideo", "text/plain", []byte("at://did:plc:x/place.stream.video/abc"), nil, nil))
	r, err = s.handlePlaceStreamBrandingGetBlob(ctx, "did:web:example.com", "defaultVideo")
	require.NoError(t, err)
	b, err = io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, "at://did:plc:x/place.stream.video/abc", string(b))
}
