package model

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/placestream"
)

func TestModerationDelegationsUseRepoScopedKeys(t *testing.T) {
	ctx := context.Background()
	db := indexedTestDB(t)
	record := placestream.ModerationPermission{
		LexiconTypeID: "place.stream.moderation.permission",
		Moderator:     "did:plc:moderator",
		Permissions:   []string{"ban"},
		CreatedAt:     "2026-09-28T00:00:00.000Z",
	}

	for index, streamer := range []string{"did:plc:streamer-one", "did:plc:streamer-two"} {
		aturi, err := syntax.ParseATURI("at://" + streamer + "/place.stream.moderation.permission/shared-key")
		require.NoError(t, err)
		require.NoError(t, db.CreateModerationDelegation(ctx, record, aturi))
		if index == 0 {
			require.ErrorIs(t, db.CreateModerationDelegation(ctx, record, aturi), ErrAlreadyIndexed)
		}
	}

	require.Equal(t, int64(2), countRows(t, db, &ModerationDelegation{}))
	require.NoError(t, db.DeleteModerationDelegation(ctx, "did:plc:streamer-one", "shared-key"))
	require.Equal(t, int64(1), countRows(t, db, &ModerationDelegation{}))

	streamerOneRecords, err := db.GetStreamerModerators(ctx, "did:plc:streamer-one")
	require.NoError(t, err)
	require.Empty(t, streamerOneRecords)

	streamerTwoRecords, err := db.GetStreamerModerators(ctx, "did:plc:streamer-two")
	require.NoError(t, err)
	require.Len(t, streamerTwoRecords, 1)
}

func TestModerationDelegationMigrationPreservesLegacyKeys(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), fmt.Sprintf("index_%d.sqlite", DBRevision))
	legacyDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, legacyDB.Exec(`CREATE TABLE moderation_delegations (
		rkey TEXT PRIMARY KEY,
		cid TEXT,
		repo_did TEXT,
		moderator_did TEXT,
		record BLOB,
		expiration_time DATETIME,
		indexed_at DATETIME
	)`).Error)
	require.NoError(t, legacyDB.Exec(`INSERT INTO moderation_delegations
		(rkey, cid, repo_did, moderator_did, record, indexed_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		"shared-key", "bafy-old", "did:plc:streamer-one", "did:plc:moderator", []byte("legacy record"),
	).Error)
	sqlDB, err := legacyDB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	mod, err := MakeDB(filepath.Dir(dbPath))
	require.NoError(t, err)
	db := mod.(*DBModel)
	var legacy ModerationDelegation
	require.NoError(t, db.DB.Where("repo_did = ?", "did:plc:streamer-one").First(&legacy).Error)
	require.Equal(t, "shared-key", legacy.RKey)
	require.Equal(t, []byte("legacy record"), legacy.Record)

	aturi, err := syntax.ParseATURI("at://did:plc:streamer-two/place.stream.moderation.permission/shared-key")
	require.NoError(t, err)
	require.NoError(t, db.CreateModerationDelegation(context.Background(), placestream.ModerationPermission{
		LexiconTypeID: "place.stream.moderation.permission",
		Moderator:     "did:plc:moderator",
		Permissions:   []string{"ban"},
		CreatedAt:     "2026-09-28T00:00:00.000Z",
	}, aturi))
	require.Equal(t, int64(2), countRows(t, db, &ModerationDelegation{}))
}
