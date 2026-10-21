package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGetRepoByHandleRefusesUnusableHandles: "handle.invalid" and "" are shared
// by every account whose handle did not verify, so matching on them would hand
// back whichever of those accounts the table happens to list first. The
// accounts themselves must stay reachable by DID.
func TestGetRepoByHandleRefusesUnusableHandles(t *testing.T) {
	db := indexedTestDB(t)
	for _, repo := range []Repo{
		{DID: "did:plc:a", Handle: "handle.invalid"},
		{DID: "did:plc:b", Handle: "handle.invalid"},
		{DID: "did:plc:c"},
		{DID: "did:plc:d", Handle: "alice.example"},
	} {
		require.NoError(t, db.UpdateRepo(&repo))
	}

	for _, unusable := range []string{"handle.invalid", "HANDLE.INVALID", ""} {
		got, err := db.GetRepoByHandleOrDID(unusable)
		require.NoError(t, err)
		require.Nil(t, got, "%q must not resolve to an arbitrary account", unusable)
	}

	for _, did := range []string{"did:plc:a", "did:plc:b", "did:plc:c"} {
		got, err := db.GetRepoByHandleOrDID(did)
		require.NoError(t, err)
		require.NotNil(t, got, "%s stays reachable by DID", did)
		require.Equal(t, did, got.DID)
	}

	// Handles are case-insensitive, and the table stores them normalized.
	got, err := db.GetRepoByHandleOrDID("Alice.Example")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "did:plc:d", got.DID)
}

// TestUpdateRepoIdentityEvictsStaleHandleOwner: a handle verified for one DID
// is not verified for any other. When it changes hands and this node only hears
// about the new owner, the old owner's row must stop answering to it -- it may be
// an account the sweep never visits again.
func TestUpdateRepoIdentityEvictsStaleHandleOwner(t *testing.T) {
	db := indexedTestDB(t)
	old := fullRepo("did:plc:old")
	old.Handle = "alice.example"
	require.NoError(t, db.UpdateRepo(old))
	next := fullRepo("did:plc:new")
	next.Handle = "alice.example"
	require.NoError(t, db.UpdateRepo(next))

	// A newly indexed owner can expose a stale claim before either identity
	// has been refreshed. Ambiguity must not route to an arbitrary account,
	// and it must not look like "nobody": a nil sends callers off to index the
	// handle through a cache that may still name the old owner.
	_, err := db.GetRepoByHandleOrDID("alice.example")
	require.ErrorIs(t, err, ErrAmbiguousHandle)

	evicted, err := db.UpdateRepoIdentity("did:plc:new", "alice.example", "https://new-pds.example")
	require.NoError(t, err)
	require.Equal(t, []string{"did:plc:old"}, evicted, "the caller is told whose cache to drop")

	got, err := db.GetRepoByHandleOrDID("alice.example")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "did:plc:new", got.DID, "the handle follows the DID it verifies for")
	require.Equal(t, "https://new-pds.example", got.PDS)
	require.Equal(t, next.Version, got.Version, "identity writes leave the sync state alone")
	require.Equal(t, next.BackfillFloor, got.BackfillFloor)

	stale, err := db.GetRepo("did:plc:old")
	require.NoError(t, err)
	require.Empty(t, stale.Handle, "the stale owner no longer claims the handle")
	require.Equal(t, old.Version, stale.Version, "and keeps everything else")

	// A broken handle claims nothing, so it evicts nobody...
	evicted, err = db.UpdateRepoIdentity("did:plc:old", "handle.invalid", old.PDS)
	require.NoError(t, err)
	require.Empty(t, evicted)
	got, err = db.GetRepoByHandleOrDID("alice.example")
	require.NoError(t, err)
	require.Equal(t, "did:plc:new", got.DID)

	// ...and an identity refresh is not how an account gets indexed.
	evicted, err = db.UpdateRepoIdentity("did:plc:stranger", "carol.example", "https://pds.example")
	require.NoError(t, err)
	require.Empty(t, evicted)
	stranger, err := db.GetRepo("did:plc:stranger")
	require.NoError(t, err)
	require.Nil(t, stranger)
}

// TestSearchReposByHandleSkipsInvalidHandles: typeahead suggestions are names
// to pick; the sentinel is not one.
func TestSearchReposByHandleSkipsInvalidHandles(t *testing.T) {
	db := indexedTestDB(t)
	require.NoError(t, db.UpdateRepo(&Repo{DID: "did:plc:a", Handle: "handle.invalid"}))
	require.NoError(t, db.UpdateRepo(&Repo{DID: "did:plc:b", Handle: "handle.example"}))

	got, err := db.SearchReposByHandle("handle", 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "did:plc:b", got[0].DID)
}

func TestHandleOrDID(t *testing.T) {
	require.Equal(t, "alice.example", HandleOrDID("alice.example", "did:plc:a"))
	require.Equal(t, "did:plc:a", HandleOrDID("handle.invalid", "did:plc:a"))
	require.Equal(t, "did:plc:a", HandleOrDID("", "did:plc:a"))
}
