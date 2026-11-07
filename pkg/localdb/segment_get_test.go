package localdb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// GetSegment is the receive-side dedup check: a known id must come back as
// the segment, not as an error, or a replayed segment (every MoQ reconnect
// is primed with recent ones) drops the whole session.
func TestGetSegment(t *testing.T) {
	db, err := MakeDB(":memory:")
	require.NoError(t, err)
	ldb := db.(*LocalDatabase)

	seg, err := ldb.GetSegment("urn:c2pa:unknown")
	require.NoError(t, err)
	require.Nil(t, seg)

	require.NoError(t, ldb.CreateSegment(&Segment{ID: "urn:c2pa:known", RepoDID: "did:plc:a", StartTime: time.Now().UTC()}))
	seg, err = ldb.GetSegment("urn:c2pa:known")
	require.NoError(t, err)
	require.NotNil(t, seg)
	require.Equal(t, "did:plc:a", seg.RepoDID)
}
