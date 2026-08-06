package model

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/plugin/prometheus"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
)

type DBModel struct {
	DB *gorm.DB
}

type Model interface {
	CreatePlayerEvent(event PlayerEventAPI) error
	ListPlayerEvents(playerID string) ([]PlayerEvent, error)
	PlayerReport(playerID string) (map[string]any, error)
	ClearPlayerEvents() error

	GetIdentity(id string) (*Identity, error)
	UpdateIdentity(ident *Identity) error

	GetRepo(did string) (*Repo, error)
	GetRepoByHandle(handle string) (*Repo, error)
	GetRepoByHandleOrDID(arg string) (*Repo, error)
	GetRepoBySigningKey(signingKey string) (*Repo, error)
	GetAllRepos() ([]Repo, error)
	SearchReposByHandle(query string, limit int) ([]Repo, error)
	UpdateRepo(repo *Repo) error
	UpdateRepoIdentity(did, handle, pds string) error
	AdvanceRepoBackfill(ctx context.Context, did, version, rootCID, floor string, done bool) (bool, error)
	AdvanceRepoVersion(ctx context.Context, did, from, to string) (bool, error)
	MarkRepoForRepair(ctx context.Context, did, from string) (bool, error)
	SetRepoStatus(ctx context.Context, did string, status string) error
	TerminalRepoDIDs(ctx context.Context) ([]string, error)

	UpdateSigningKey(key *SigningKey) error
	GetSigningKey(ctx context.Context, did, repoDID string) (*SigningKey, error)
	GetSigningKeyByRKey(ctx context.Context, rkey string) (*SigningKey, error)
	GetSigningKeysForRepo(repoDID string) ([]SigningKey, error)

	CreateFollow(ctx context.Context, userDID, rev string, follow appbsky.GraphFollow) error
	GetUserFollowing(ctx context.Context, userDID string) ([]Follow, error)
	GetUserFollowers(ctx context.Context, userDID string) ([]Follow, error)
	GetUserFollowingUser(ctx context.Context, userDID, subjectDID string) (*Follow, error)
	CountFollowersBatch(ctx context.Context, dids []string) (map[string]int, error)
	DeleteFollow(ctx context.Context, userDID, rev string) error

	CreateFeedPost(ctx context.Context, post *FeedPost) error
	ListFeedPosts() ([]FeedPost, error)
	ListFeedPostsByType(feedType string, limit int, after int64) ([]FeedPost, error)
	GetFeedPost(uri string) (*FeedPost, error)
	GetReplies(repoDID string) ([]appbsky.FeedDefs_PostView, error)

	CreateLivestream(ctx context.Context, ls *Livestream) error
	GetLivestream(uri string) (*Livestream, error)
	GetLatestLivestreamForRepo(repoDID string) (*Livestream, error)
	GetLivestreamByPostURI(postURI string) (*Livestream, error)
	GetLatestLivestreams(limit int, before *time.Time, dids []string) ([]Livestream, error)

	CreateTeleport(ctx context.Context, tp *Teleport) error
	GetLatestTeleportForRepo(repoDID string) (*Teleport, error)
	GetActiveTeleportsForRepo(repoDID string) ([]Teleport, error)
	GetActiveTeleportsToRepo(targetDID string) ([]Teleport, error)
	GetTeleportByURI(uri string) (*Teleport, error)
	DeleteTeleport(ctx context.Context, uri string) error
	DenyTeleport(ctx context.Context, uri string) error

	CreateBlock(ctx context.Context, block *Block) error
	GetBlock(ctx context.Context, rkey string) (*Block, error)
	GetUserBlock(ctx context.Context, userDID, subjectDID string) (*Block, error)
	DeleteBlock(ctx context.Context, rkey string) error

	CreateChatMessage(ctx context.Context, message *ChatMessage) error
	MostRecentChatMessages(repoDID string) ([]placestream.ChatDefs_MessageView, error)
	GetChatMessage(uri string) (*ChatMessage, error)
	DeleteChatMessage(ctx context.Context, uri string, deletedAt *time.Time) error

	CreateGate(ctx context.Context, gate *Gate) error
	DeleteGate(ctx context.Context, rkey string) error
	GetGate(ctx context.Context, rkey string) (*Gate, error)
	GetUserGates(ctx context.Context, userDID string) ([]*Gate, error)

	CreatePinnedRecord(ctx context.Context, pin *PinnedRecord) error
	DeletePinnedRecord(ctx context.Context, uri string) error
	DeleteAllPinnedRecords(ctx context.Context, streamerDID string) error
	GetPinnedRecord(ctx context.Context, uri string) (*PinnedRecord, error)
	GetActivePinnedRecord(ctx context.Context, streamerDID string) (*PinnedRecord, error)

	CreateChatProfile(ctx context.Context, profile *ChatProfile) error
	GetChatProfile(ctx context.Context, repoDID string) (*ChatProfile, error)

	UpdateServerSettings(ctx context.Context, settings *ServerSettings) error
	GetServerSettings(ctx context.Context, server string, repoDID string) (*ServerSettings, error)
	DeleteServerSettings(ctx context.Context, server string, repoDID string) error

	CreateLabeler(did string) (*Labeler, error)
	GetLabeler(did string) (*Labeler, error)
	UpdateLabelerCursor(did string, cursor int64) error

	GetRelayCursor(host string) (*RelayCursor, error)
	UpsertRelayCursor(host string, cursor int64, lastEventTime int64) error

	CreateLabel(label *Label) error
	GetActiveLabels(uri string) ([]*comatproto.LabelDefs_Label, error)

	UpdateBroadcastOrigin(ctx context.Context, origin placestream.BroadcastOrigin, aturi syntax.ATURI) error
	GetRecentBroadcastOrigins(ctx context.Context) ([]placestream.BroadcastDefs_BroadcastOriginView, error)

	CreateMetadataConfiguration(ctx context.Context, metadata *MetadataConfiguration) error
	GetMetadataConfiguration(ctx context.Context, repoDID string) (*MetadataConfiguration, error)
	DeleteMetadataConfiguration(ctx context.Context, repoDID string) error

	CreateModerationDelegation(ctx context.Context, rec placestream.ModerationPermission, aturi syntax.ATURI) error
	DeleteModerationDelegation(ctx context.Context, rkey string) error
	GetModerationDelegation(ctx context.Context, streamerDID, moderatorDID string) (*placestream.ModerationDefs_PermissionView, error)
	GetModerationDelegations(ctx context.Context, streamerDID, moderatorDID string) ([]placestream.ModerationDefs_PermissionView, error)
	GetModeratorDelegations(ctx context.Context, moderatorDID string) ([]placestream.ModerationDefs_PermissionView, error)
	GetStreamerModerators(ctx context.Context, streamerDID string) ([]placestream.ModerationDefs_PermissionView, error)

	GetRecommendation(userDID string) (*Recommendation, error)
	UpsertRecommendation(rec *Recommendation) error

	UpsertBskyProfile(ctx context.Context, aturi syntax.ATURI, profileBs []byte, wasStreamplace bool) error
	GetBskyProfile(ctx context.Context, did string, wasStreamplace bool) (*appbsky.ActorProfile, error)

	UpsertBadgeDef(ctx context.Context, def *BadgeDef) error
	DeleteBadgeDef(ctx context.Context, uri string) error
	GetBadgeDefByURI(ctx context.Context, uri string) (*BadgeDef, error)
	UpsertBadgeIssuance(ctx context.Context, issuance *BadgeIssuance) error
	DeleteBadgeIssuance(ctx context.Context, uri string) error
	GetBadgeIssuanceByURI(ctx context.Context, uri string) (*BadgeIssuance, error)
	GetBadgeIssuancesForRecipient(ctx context.Context, recipientDID string) ([]*BadgeIssuance, error)

	UpsertVideo(ctx context.Context, rec placestream.Video, aturi syntax.ATURI) error
	DeleteVideo(ctx context.Context, uri string) error
	GetVideoByURI(ctx context.Context, uri string) (*placestream.Video, error)
	GetLatestVideosForRepo(ctx context.Context, repoDID string, limit int) ([]*Video, error)

	UpsertMediaTrack(ctx context.Context, rec placestream.MediaTrack, aturi syntax.ATURI) error
	DeleteMediaTrack(ctx context.Context, uri string) error
	GetMediaTrackByURI(ctx context.Context, uri string) (*placestream.MediaTrack, error)
	GetMediaTracksByBlob(ctx context.Context, blob string) ([]*MediaTrack, error)

	UpsertMediaOrigin(ctx context.Context, rec placestream.MediaOrigin, aturi syntax.ATURI) error
	UpsertOwnMediaOrigin(ctx context.Context, serverDID, blobCID string, size int64, mimeType string) error
	DeleteMediaOrigin(ctx context.Context, uri string) error
	GetMediaOriginByURI(ctx context.Context, uri string) (placestream.MediaOrigin, error)
	GetMediaOriginsByBlob(ctx context.Context, blob string) ([]*MediaOrigin, error)

	UpsertBetaInvite(ctx context.Context, rec placestream.BetaInvite, aturi syntax.ATURI) error
	DeleteBetaInvite(ctx context.Context, uri string) error
	HasBetaInvite(ctx context.Context, fromRepoDID, subjectDID, feature string) (bool, error)
	UpsertBetaRequest(ctx context.Context, rec placestream.BetaRequest, aturi syntax.ATURI) error
	DeleteBetaRequest(ctx context.Context, uri string) error
	HasBetaRequest(ctx context.Context, subjectDID, feature string) (bool, error)

	UpsertMediaViewCount(ctx context.Context, rec placestream.MediaViewCount, aturi syntax.ATURI) error
	DeleteMediaViewCount(ctx context.Context, uri string) error
	GetMediaViewCountByURI(ctx context.Context, uri string) (*placestream.MediaViewCount, error)
	GetVideoView(ctx context.Context, uri string) (*placestream.MediaGetVideo_VideoView, error)
	GetVideoList(ctx context.Context, repoDID string, limit int, cursor string, hostedByServerDID string) (placestream.MediaGetVideoList_Output, error)

	CreateVodComment(ctx context.Context, comment *VodComment) error
	DeleteVodComment(ctx context.Context, uri string, deletedAt *time.Time) error
	GetVodComment(uri string) (*VodComment, error)
	GetCommentsForVideo(ctx context.Context, videoURI string, limit int, cursor *time.Time) ([]placestream.VodDefs_CommentView, *time.Time, error)

	CreateLike(ctx context.Context, like *Like) error
	DeleteLike(ctx context.Context, uri string) error
	GetLike(uri string) (*Like, error)
	GetLikeBySubjectAndUser(ctx context.Context, subject string, repoDID string) (*Like, error)
	GetLikesForSubject(ctx context.Context, subject string, limit int, cursor *time.Time) ([]placestream.GetLikes_LikeView, int64, *time.Time, error)
	GetLikeCount(ctx context.Context, subject string) (int64, error)

	CreateVodGate(ctx context.Context, gate *VodGate) error
	DeleteVodGate(ctx context.Context, rkey string) error
	GetVodGate(ctx context.Context, rkey string) (*VodGate, error)
	GetUserVodGates(ctx context.Context, userDID string) ([]*VodGate, error)
}

// DO NOT UPDATE THIS UNLESS A BREAKING CHANGE IS MADE
// WHICH ALSO SHOULD NOT HAPPEN
var DBRevision = 5

func MakeDB(dbURL string) (Model, error) {
	sqliteSuffix := dbURL
	if dbURL != ":memory:" {
		// Ensure dbURL exists as a directory on the filesystem
		if err := os.MkdirAll(dbURL, os.ModePerm); err != nil {
			return nil, fmt.Errorf("error creating database directory: %w", err)
		}
		dbPath := filepath.Join(dbURL, fmt.Sprintf("index_%d.sqlite", DBRevision))
		sqliteSuffix = dbPath
		// if this isn't ":memory:", ensure that directory exists (eg, if db
		// file is being initialized)
		if err := os.MkdirAll(filepath.Dir(sqliteSuffix), os.ModePerm); err != nil {
			return nil, fmt.Errorf("error creating database path: %w", err)
		}
	}
	log.Log(context.Background(), "starting database", "dbURL", sqliteSuffix)
	// The pragmas ride in the DSN because they are per-connection settings and
	// this pool has more than one: an Exec would configure whichever connection
	// happened to serve it and leave the rest at defaults.
	//
	//   - _busy_timeout: wait for a lock another connection (or the second
	//     process: `streamplace sync` warming a new index) holds, instead of
	//     failing the query with SQLITE_BUSY.
	//   - _journal_mode=WAL: readers run against a snapshot while a writer
	//     writes. This is what lets a boot-time reindex proceed without
	//     blocking the requests the node is serving.
	//   - _synchronous=NORMAL: WAL's standard pairing -- fsync at checkpoints
	//     rather than every commit. A power loss can cost the tail since the
	//     last checkpoint, which this index is allowed to lose: everything in
	//     it is re-derivable from the network, and the sweep re-derives it.
	//   - _txlock=immediate: explicit transactions take the write lock up
	//     front instead of upgrading mid-transaction, which is the classic
	//     multi-connection sqlite deadlock.
	dsn := sqliteSuffix
	pool := IndexDBPoolSize
	if sqliteSuffix == ":memory:" {
		// A pool of :memory: connections would each open a PRIVATE empty
		// database -- with :memory:, one connection IS the database. Tests use
		// this; they keep the old single-connection arrangement.
		pool = 1
	} else {
		dsn = fmt.Sprintf("file:%s?_busy_timeout=%d&_journal_mode=WAL&_synchronous=NORMAL&_txlock=immediate",
			sqliteSuffix, SQLiteBusyTimeout.Milliseconds())
	}
	dial := sqlite.Open(dsn)

	db, err := gorm.Open(dial, &gorm.Config{
		SkipDefaultTransaction: true,
		TranslateError:         true,
		Logger:                 config.GormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("error starting database: %w", err)
	}
	err = db.Exec("PRAGMA journal_mode=WAL;").Error
	if err != nil {
		return nil, fmt.Errorf("error setting journal mode: %w", err)
	}
	if err := SetSQLiteBusyTimeout(db); err != nil {
		return nil, err
	}

	err = db.Use(prometheus.New(prometheus.Config{
		DBName:          "index",
		RefreshInterval: 10,
		StartServer:     false,
	}))
	if err != nil {
		return nil, fmt.Errorf("error using prometheus plugin: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("error getting database: %w", err)
	}
	sqlDB.SetMaxOpenConns(pool)
	sqlDB.SetMaxIdleConns(pool)
	for _, model := range []any{
		PlayerEvent{},
		Identity{},
		Repo{},
		SigningKey{},
		Follow{},
		FeedPost{},
		Livestream{},
		Block{},
		ChatMessage{},
		ChatProfile{},
		Gate{},
		PinnedRecord{},
		ServerSettings{},
		Labeler{},
		RelayCursor{},
		Label{},
		BroadcastOrigin{},
		MetadataConfiguration{},
		Teleport{},
		ModerationDelegation{},
		Recommendation{},
		BskyProfile{},
		BadgeDef{},
		BadgeIssuance{},
		Video{},
		MediaTrack{},
		MediaOrigin{},
		MediaViewCount{},
		BetaInvite{},
		BetaRequest{},
		VodComment{},
		Like{},
		VodGate{},
	} {
		err = db.AutoMigrate(model)
		if err != nil {
			return nil, err
		}
	}
	return &DBModel{DB: db}, nil
}

// SQLiteBusyTimeout is how long a sqlite connection waits for a lock another
// connection holds before giving up with SQLITE_BUSY. That other connection
// is usually a sibling in this process's own pool (writers serialize on
// sqlite's write lock; readers never wait under WAL), and occasionally a
// second process: `streamplace sync` warming a new index revision while the
// server runs.
const SQLiteBusyTimeout = 5 * time.Second

// IndexDBPoolSize is how many connections the index database keeps open.
//
// More than one is what makes WAL worth having: reads run against a snapshot
// on their own connections while a writer writes, so a boot-time reindex or a
// busy sweep stops queueing every request behind it. Writes still serialize --
// on sqlite's write lock, waiting up to [SQLiteBusyTimeout] -- so raising this
// helps read concurrency only, and modestly: past a handful of connections the
// single write lock is the ceiling.
const IndexDBPoolSize = 8

// SetSQLiteBusyTimeout applies [SQLiteBusyTimeout] to an open sqlite database.
// It is a per-connection setting, which is why it is set on the pool rather
// than being part of the DSN nothing else in here uses.
func SetSQLiteBusyTimeout(db *gorm.DB) error {
	ms := SQLiteBusyTimeout.Milliseconds()
	if err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout = %d;", ms)).Error; err != nil {
		return fmt.Errorf("error setting busy timeout: %w", err)
	}
	return nil
}
