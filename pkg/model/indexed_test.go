package model

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/appbsky"
)

// indexedTestDB returns an empty in-memory index. The concrete type is handed
// back so assertions can count rows directly, without going through the query
// helpers (several of which filter deleted rows or need joined repos).
func indexedTestDB(t *testing.T) *DBModel {
	t.Helper()
	mod, err := MakeDB(":memory:")
	require.NoError(t, err)
	db, ok := mod.(*DBModel)
	require.True(t, ok)
	return db
}

func countRows(t *testing.T, db *DBModel, model any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.DB.Model(model).Count(&n).Error)
	return n
}

// TestCreateOrVerifyBlock covers both arms of the conflict path on a row keyed
// by rkey with a separate CID column: the same record twice is a silent no-op,
// and a new CID at the same key overwrites.
func TestCreateOrVerifyBlock(t *testing.T) {
	ctx := context.Background()
	mod := indexedTestDB(t)

	block := &Block{
		RKey:       "3lblock1",
		CID:        "bafyfirst",
		RepoDID:    "did:plc:blocker",
		SubjectDID: "did:plc:blocked",
		Record:     []byte("one"),
		CreatedAt:  time.Now().UTC(),
	}
	require.NoError(t, mod.CreateBlock(ctx, block))
	require.Equal(t, int64(1), countRows(t, mod, &Block{}))

	// Redelivery: same key, same CID.
	again := *block
	require.ErrorIs(t, mod.CreateBlock(ctx, &again), ErrAlreadyIndexed)
	require.Equal(t, int64(1), countRows(t, mod, &Block{}))
	stored, err := mod.GetBlock(ctx, block.RKey)
	require.NoError(t, err)
	require.Equal(t, "bafyfirst", stored.CID)
	require.Equal(t, []byte("one"), stored.Record)

	// An update we missed: same key, different CID.
	updated := *block
	updated.CID = "bafysecond"
	updated.SubjectDID = "did:plc:blocked-someone-else"
	updated.Record = []byte("two")
	require.NoError(t, mod.CreateBlock(ctx, &updated))
	require.Equal(t, int64(1), countRows(t, mod, &Block{}))
	stored, err = mod.GetBlock(ctx, block.RKey)
	require.NoError(t, err)
	require.Equal(t, "bafysecond", stored.CID)
	require.Equal(t, "did:plc:blocked-someone-else", stored.SubjectDID)
	require.Equal(t, []byte("two"), stored.Record)
}

// TestCreateOrVerifyFeedPost is the same shape on a URI-keyed table.
func TestCreateOrVerifyFeedPost(t *testing.T) {
	ctx := context.Background()
	mod := indexedTestDB(t)

	now := time.Now().UTC()
	body := []byte("first")
	post := &FeedPost{
		URI:       "at://did:plc:poster/app.bsky.feed.post/3lpost1",
		CID:       "bafyfirst",
		CreatedAt: now,
		FeedPost:  &body,
		RepoDID:   "did:plc:poster",
		Type:      "reply",
		IndexedAt: &now,
	}
	require.NoError(t, mod.CreateFeedPost(ctx, post))

	again := *post
	require.ErrorIs(t, mod.CreateFeedPost(ctx, &again), ErrAlreadyIndexed)
	require.Equal(t, int64(1), countRows(t, mod, &FeedPost{}))
	stored, err := mod.GetFeedPost(post.URI)
	require.NoError(t, err)
	require.Equal(t, "bafyfirst", stored.CID)
	require.Equal(t, "first", string(*stored.FeedPost))

	newBody := []byte("edited")
	updated := *post
	updated.CID = "bafysecond"
	updated.FeedPost = &newBody
	require.NoError(t, mod.CreateFeedPost(ctx, &updated))
	require.Equal(t, int64(1), countRows(t, mod, &FeedPost{}))
	stored, err = mod.GetFeedPost(post.URI)
	require.NoError(t, err)
	require.Equal(t, "bafysecond", stored.CID)
	require.Equal(t, "edited", string(*stored.FeedPost))
}

// TestCreateOrVerifyChatMessage covers the CID-keyed tables. There a conflict
// can only ever be a redelivery, and an edited message -- a different CID -- is
// a row of its own, which is how the table has always worked.
func TestCreateOrVerifyChatMessage(t *testing.T) {
	ctx := context.Background()
	mod := indexedTestDB(t)

	now := time.Now().UTC()
	body := []byte("hello")
	msg := &ChatMessage{
		CID:             "bafymsg1",
		URI:             "at://did:plc:chatter/place.stream.chat.message/3lmsg1",
		CreatedAt:       now,
		ChatMessage:     &body,
		RepoDID:         "did:plc:chatter",
		StreamerRepoDID: "did:plc:streamer",
		IndexedAt:       &now,
	}
	require.NoError(t, mod.CreateChatMessage(ctx, msg))

	again := *msg
	require.ErrorIs(t, mod.CreateChatMessage(ctx, &again), ErrAlreadyIndexed)
	require.Equal(t, int64(1), countRows(t, mod, &ChatMessage{}))

	edited := *msg
	edited.CID = "bafymsg2"
	require.NoError(t, mod.CreateChatMessage(ctx, &edited))
	require.Equal(t, int64(2), countRows(t, mod, &ChatMessage{}),
		"a message keyed by CID gets a new row when its content changes")
}

// TestCreateFollowIsIdempotent guards the Save-based writers, which never had a
// duplicate problem to fix but must not grow one.
func TestCreateFollowIsIdempotent(t *testing.T) {
	ctx := context.Background()
	mod := indexedTestDB(t)

	follow := appbsky.GraphFollow{
		LexiconTypeID: "app.bsky.graph.follow",
		Subject:       "did:plc:followed",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	require.NoError(t, mod.CreateFollow(ctx, "did:plc:follower", "3lfollow1", follow))
	require.NoError(t, mod.CreateFollow(ctx, "did:plc:follower", "3lfollow1", follow))
	require.Equal(t, int64(1), countRows(t, mod, &Follow{}))

	got, err := mod.GetUserFollowingUser(ctx, "did:plc:follower", "did:plc:followed")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "3lfollow1", got.RKey)
}

// TestCreateChatProfileIsIdempotent: one row per repo, no CID to compare, so
// re-storing it is a plain overwrite rather than ErrAlreadyIndexed.
func TestCreateChatProfileIsIdempotent(t *testing.T) {
	ctx := context.Background()
	mod := indexedTestDB(t)

	rec := []byte("profile")
	profile := &ChatProfile{RepoDID: "did:plc:chatter", Record: &rec}
	require.NoError(t, mod.CreateChatProfile(ctx, profile))
	require.NoError(t, mod.CreateChatProfile(ctx, profile))
	require.Equal(t, int64(1), countRows(t, mod, &ChatProfile{}))
}

// TestRepoStatus covers the account lifecycle column: setting a status leaves
// the sync state alone, and the terminal list is what the boot sweep filters on.
func TestRepoStatus(t *testing.T) {
	ctx := context.Background()
	mod := indexedTestDB(t)

	require.NoError(t, mod.UpdateRepo(&Repo{
		DID:     "did:plc:gone",
		Handle:  "gone.test",
		PDS:     "https://pds.test",
		Version: "3lrev0000",
		RootCID: "bafyroot",
	}))
	require.NoError(t, mod.UpdateRepo(&Repo{DID: "did:plc:fine", Version: "3lrev0001"}))

	dids, err := mod.TerminalRepoDIDs(ctx)
	require.NoError(t, err)
	require.Empty(t, dids)

	require.NoError(t, mod.SetRepoStatus(ctx, "did:plc:gone", RepoStatusDeactivated))
	stored, err := mod.GetRepo("did:plc:gone")
	require.NoError(t, err)
	require.Equal(t, RepoStatusDeactivated, stored.Status)
	require.True(t, stored.TerminalStatus())
	require.Equal(t, "3lrev0000", stored.Version, "status must not disturb the sync state")
	require.Equal(t, "bafyroot", stored.RootCID)

	dids, err = mod.TerminalRepoDIDs(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"did:plc:gone"}, dids)

	require.NoError(t, mod.SetRepoStatus(ctx, "did:plc:gone", RepoStatusOK))
	dids, err = mod.TerminalRepoDIDs(ctx)
	require.NoError(t, err)
	require.Empty(t, dids)
}
