package model

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/plugin/prometheus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/streamplace"
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

	UpdateSigningKey(key *SigningKey) error
	GetSigningKey(ctx context.Context, did, repoDID string) (*SigningKey, error)
	GetSigningKeyByRKey(ctx context.Context, rkey string) (*SigningKey, error)
	GetSigningKeysForRepo(repoDID string) ([]SigningKey, error)

	CreateFollow(ctx context.Context, userDID, rev string, follow *bsky.GraphFollow) error
	GetUserFollowing(ctx context.Context, userDID string) ([]Follow, error)
	GetUserFollowers(ctx context.Context, userDID string) ([]Follow, error)
	GetUserFollowingUser(ctx context.Context, userDID, subjectDID string) (*Follow, error)
	CountFollowersBatch(ctx context.Context, dids []string) (map[string]int, error)
	DeleteFollow(ctx context.Context, userDID, rev string) error

	CreateFeedPost(ctx context.Context, post *FeedPost) error
	ListFeedPosts() ([]FeedPost, error)
	ListFeedPostsByType(feedType string, limit int, after int64) ([]FeedPost, error)
	GetFeedPost(uri string) (*FeedPost, error)
	GetReplies(repoDID string) ([]*bsky.FeedDefs_PostView, error)

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
	MostRecentChatMessages(repoDID string) ([]*streamplace.ChatDefs_MessageView, error)
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
	UpsertRelayCursor(host string, cursor int64) error

	CreateLabel(label *Label) error
	GetActiveLabels(uri string) ([]*comatproto.LabelDefs_Label, error)

	UpdateBroadcastOrigin(ctx context.Context, origin *streamplace.BroadcastOrigin, aturi syntax.ATURI) error
	GetRecentBroadcastOrigins(ctx context.Context) ([]*streamplace.BroadcastDefs_BroadcastOriginView, error)

	CreateMetadataConfiguration(ctx context.Context, metadata *MetadataConfiguration) error
	GetMetadataConfiguration(ctx context.Context, repoDID string) (*MetadataConfiguration, error)
	DeleteMetadataConfiguration(ctx context.Context, repoDID string) error

	CreateModerationDelegation(ctx context.Context, rec *streamplace.ModerationPermission, aturi syntax.ATURI) error
	DeleteModerationDelegation(ctx context.Context, rkey string) error
	GetModerationDelegation(ctx context.Context, streamerDID, moderatorDID string) (*streamplace.ModerationDefs_PermissionView, error)
	GetModerationDelegations(ctx context.Context, streamerDID, moderatorDID string) ([]*streamplace.ModerationDefs_PermissionView, error)
	GetModeratorDelegations(ctx context.Context, moderatorDID string) ([]*streamplace.ModerationDefs_PermissionView, error)
	GetStreamerModerators(ctx context.Context, streamerDID string) ([]*streamplace.ModerationDefs_PermissionView, error)

	GetRecommendation(userDID string) (*Recommendation, error)
	UpsertRecommendation(rec *Recommendation) error

	UpsertBskyProfile(ctx context.Context, aturi syntax.ATURI, profileBs []byte, wasStreamplace bool) error
	GetBskyProfile(ctx context.Context, did string, wasStreamplace bool) (*bsky.ActorProfile, error)

	UpsertBadgeDef(ctx context.Context, def *BadgeDef) error
	DeleteBadgeDef(ctx context.Context, uri string) error
	GetBadgeDefByURI(ctx context.Context, uri string) (*BadgeDef, error)
	UpsertBadgeIssuance(ctx context.Context, issuance *BadgeIssuance) error
	DeleteBadgeIssuance(ctx context.Context, uri string) error
	GetBadgeIssuanceByURI(ctx context.Context, uri string) (*BadgeIssuance, error)
	GetBadgeIssuancesForRecipient(ctx context.Context, recipientDID string) ([]*BadgeIssuance, error)

	UpsertVideo(ctx context.Context, rec *streamplace.Video, aturi syntax.ATURI) error
	DeleteVideo(ctx context.Context, uri string) error
	GetVideoByURI(ctx context.Context, uri string) (*streamplace.Video, error)
	GetLatestVideosForRepo(ctx context.Context, repoDID string, limit int) ([]*Video, error)

	UpsertMediaTrack(ctx context.Context, rec *streamplace.MediaTrack, aturi syntax.ATURI) error
	DeleteMediaTrack(ctx context.Context, uri string) error
	GetMediaTrackByURI(ctx context.Context, uri string) (*streamplace.MediaTrack, error)
	GetMediaTracksByBlob(ctx context.Context, blob string) ([]*MediaTrack, error)

	UpsertMediaOrigin(ctx context.Context, rec *streamplace.MediaOrigin, aturi syntax.ATURI) error
	DeleteMediaOrigin(ctx context.Context, uri string) error
	GetMediaOriginByURI(ctx context.Context, uri string) (*streamplace.MediaOrigin, error)
	GetMediaOriginsByBlob(ctx context.Context, blob string) ([]*MediaOrigin, error)

	UpsertBetaInvite(ctx context.Context, rec *streamplace.BetaInvite, aturi syntax.ATURI) error
	DeleteBetaInvite(ctx context.Context, uri string) error
	HasBetaInvite(ctx context.Context, fromRepoDID, subjectDID, feature string) (bool, error)
	UpsertBetaRequest(ctx context.Context, rec *streamplace.BetaRequest, aturi syntax.ATURI) error
	DeleteBetaRequest(ctx context.Context, uri string) error
	HasBetaRequest(ctx context.Context, subjectDID, feature string) (bool, error)

	UpsertMediaViewCount(ctx context.Context, rec *streamplace.MediaViewCount, aturi syntax.ATURI) error
	DeleteMediaViewCount(ctx context.Context, uri string) error
	GetMediaViewCountByURI(ctx context.Context, uri string) (*streamplace.MediaViewCount, error)
	GetVideoView(ctx context.Context, uri string) (*streamplace.MediaGetVideo_VideoView, error)
	GetVideoList(ctx context.Context, repoDID string, limit int, cursor string, hostedByServerDID string) (*streamplace.MediaGetVideoList_Output, error)

	CreateVodComment(ctx context.Context, comment *VodComment) error
	DeleteVodComment(ctx context.Context, uri string, deletedAt *time.Time) error
	GetVodComment(uri string) (*VodComment, error)
	GetCommentsForVideo(ctx context.Context, videoURI string, limit int, cursor *time.Time) ([]*streamplace.VodDefs_CommentView, *time.Time, error)

	CreateLike(ctx context.Context, like *Like) error
	DeleteLike(ctx context.Context, uri string) error
	GetLike(uri string) (*Like, error)
	GetLikeBySubjectAndUser(ctx context.Context, subject string, repoDID string) (*Like, error)
	GetLikesForSubject(ctx context.Context, subject string, limit int, cursor *time.Time) ([]*streamplace.GetLikes_LikeView, int64, *time.Time, error)
	GetLikeCount(ctx context.Context, subject string) (int64, error)

	CreateVodGate(ctx context.Context, gate *VodGate) error
	DeleteVodGate(ctx context.Context, rkey string) error
	GetVodGate(ctx context.Context, rkey string) (*VodGate, error)
	GetUserVodGates(ctx context.Context, userDID string) ([]*VodGate, error)
}

// DO NOT UPDATE THIS UNLESS A BREAKING CHANGE IS MADE
// WHICH ALSO SHOULD NOT HAPPEN
var DBRevision = 4

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
	dial := sqlite.Open(sqliteSuffix)

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
	sqlDB.SetMaxOpenConns(1)
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
