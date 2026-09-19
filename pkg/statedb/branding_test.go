package statedb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/config"
)

// The snapshot serves repeated reads without touching the database and is
// dropped by writes through this node.
func TestBrandingSnapshot(t *testing.T) {
	state, err := MakeDB(context.Background(), &config.CLI{DBURL: ":memory:"}, nil, nil)
	require.NoError(t, err)
	const id = "did:web:example.com"
	_, err = state.GetBrandingBlob(id, "siteTitle")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	require.NoError(t, state.PutBrandingBlob(id, "siteTitle", "text/plain", []byte("one"), nil, nil))
	blob, err := state.GetBrandingBlob(id, "siteTitle")
	require.NoError(t, err)
	require.Equal(t, "one", string(blob.Data), "a write drops the snapshot that said the key was missing")

	// Write behind the snapshot's back: the stale answer stands until the TTL.
	require.NoError(t, state.DB.Model(&BrandingBlob{}).Where("broadcaster_id = ? AND key = ?", id, "siteTitle").Update("data", []byte("two")).Error)
	blob, err = state.GetBrandingBlob(id, "siteTitle")
	require.NoError(t, err)
	require.Equal(t, "one", string(blob.Data))
	v, _ := state.brandingCache.Load(id)
	v.(*brandingSnapshot).at = time.Now().Add(-brandingSnapshotTTL)
	blob, err = state.GetBrandingBlob(id, "siteTitle")
	require.NoError(t, err)
	require.Equal(t, "two", string(blob.Data))

	keys, err := state.ListBrandingKeys(id)
	require.NoError(t, err)
	require.Equal(t, []string{"siteTitle"}, keys)
	require.NoError(t, state.DeleteBrandingBlob(id, "siteTitle"))
	_, err = state.GetBrandingBlob(id, "siteTitle")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
