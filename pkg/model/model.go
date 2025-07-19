package model

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/lmittmann/tint"
	slogGorm "github.com/orandin/slog-gorm"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/streamplace"
)

type DBModel struct {
	DB  *gorm.DB
	CLI *config.CLI
}

type Model interface {
	CreateNotification(token, repoDID string) error
	ListNotifications() ([]Notification, error)

	CreatePlayerEvent(event PlayerEventAPI) error
	ListPlayerEvents(playerID string) ([]PlayerEvent, error)
	PlayerReport(playerID string) (map[string]any, error)
	ClearPlayerEvents() error

	CreateSegment(segment *Segment) error
	MostRecentSegments() ([]Segment, error)
	LatestSegmentForUser(user string) (*Segment, error)
	LatestSegmentsForUser(user string, limit int, before *time.Time) ([]Segment, error)
	CreateThumbnail(thumb *Thumbnail) error
	LatestThumbnailForUser(user string) (*Thumbnail, error)
	GetSegment(id string) (*Segment, error)
	StartSegmentCleaner(ctx context.Context) error

	GetIdentity(id string) (*Identity, error)
	UpdateIdentity(ident *Identity) error

	GetRepo(did string) (*Repo, error)
	GetRepoByHandle(handle string) (*Repo, error)
	GetRepoByHandleOrDID(arg string) (*Repo, error)
	GetRepoBySigningKey(signingKey string) (*Repo, error)
	GetAllRepos() ([]Repo, error)
	UpdateRepo(repo *Repo) error

	UpdateSigningKey(key *SigningKey) error
	GetSigningKey(ctx context.Context, did, repoDID string) (*SigningKey, error)
	GetSigningKeyByRKey(ctx context.Context, rkey string) (*SigningKey, error)
	GetSigningKeysForRepo(repoDID string) ([]SigningKey, error)

	CreateFollow(ctx context.Context, userDID, rev string, follow *bsky.GraphFollow) error
	GetUserFollowing(ctx context.Context, userDID string) ([]Follow, error)
	GetUserFollowers(ctx context.Context, userDID string) ([]Follow, error)
	GetUserFollowingUser(ctx context.Context, userDID, subjectDID string) (*Follow, error)
	DeleteFollow(ctx context.Context, userDID, rev string) error
	GetFollowersNotificationTokens(userDID string) ([]string, error)

	CreateFeedPost(ctx context.Context, post *FeedPost) error
	ListFeedPosts() ([]FeedPost, error)
	ListFeedPostsByType(feedType string, limit int, after int64) ([]FeedPost, error)
	GetFeedPost(cid string) (*FeedPost, error)
	GetReplies(repoDID string) ([]*bsky.FeedDefs_PostView, error)

	CreateLivestream(ctx context.Context, ls *Livestream) error
	GetLatestLivestreamForRepo(repoDID string) (*Livestream, error)
	GetLivestreamByPostCID(postCID string) (*Livestream, error)
	GetLatestLivestreams(limit int, before *time.Time) ([]Livestream, error)

	CreateBlock(ctx context.Context, block *Block) error
	GetBlock(ctx context.Context, rkey string) (*Block, error)
	GetUserBlock(ctx context.Context, userDID, subjectDID string) (*Block, error)
	DeleteBlock(ctx context.Context, rkey string) error

	CreateChatMessage(ctx context.Context, message *ChatMessage) error
	MostRecentChatMessages(repoDID string) ([]*streamplace.ChatDefs_MessageView, error)
	GetChatMessage(cid string) (*ChatMessage, error)

	CreateChatProfile(ctx context.Context, profile *ChatProfile) error
	GetChatProfile(ctx context.Context, repoDID string) (*ChatProfile, error)

	CreateOAuthSession(id string, session *oatproxy.OAuthSession) error
	LoadOAuthSession(id string) (*oatproxy.OAuthSession, error)
	UpdateOAuthSession(id string, session *oatproxy.OAuthSession) error
	ListOAuthSessions() ([]oatproxy.OAuthSession, error)
	GetSessionByDID(did string) (*oatproxy.OAuthSession, error)

	UpdateServerSettings(ctx context.Context, settings *ServerSettings) error
	GetServerSettings(ctx context.Context, server string, repoDID string) (*ServerSettings, error)
	DeleteServerSettings(ctx context.Context, server string, repoDID string) error

	CreateCommitEvent(commit *comatproto.SyncSubscribeRepos_Commit, signedData string) error
	GetCommitEventsSince(repoDID string, t time.Time) ([]*XrpcStreamEvent, error)
	GetCommitEventsSinceSeq(repoDID string, seq int64) ([]*XrpcStreamEvent, error)
	GetMostRecentCommitEvent(repoDID string) (*XrpcStreamEvent, error)
}

func MakeDB(dbURL string) (Model, error) {
	log.Log(context.Background(), "starting database", "dbURL", dbURL)
	sqliteSuffix := dbURL
	if dbURL != ":memory:" {
		if !strings.HasPrefix(dbURL, "sqlite://") {
			dbURL = fmt.Sprintf("sqlite://%s", dbURL)
		}
		sqliteSuffix := dbURL[len("sqlite://"):]
		// if this isn't ":memory:", ensure that directory exists (eg, if db
		// file is being initialized)
		if !strings.Contains(sqliteSuffix, ":?") {
			if err := os.MkdirAll(filepath.Dir(sqliteSuffix), os.ModePerm); err != nil {
				return nil, fmt.Errorf("error creating database path: %w", err)
			}
		}
	}
	dial := sqlite.Open(sqliteSuffix)

	gormLogger := slogGorm.New(
		slogGorm.WithHandler(tint.NewHandler(os.Stderr, &tint.Options{
			TimeFormat: time.RFC3339,
		})),
		// slogGorm.WithTraceAll(),
	)

	db, err := gorm.Open(dial, &gorm.Config{
		SkipDefaultTransaction: true,
		TranslateError:         true,
		Logger:                 gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("error starting database: %w", err)
	}
	err = db.Exec("PRAGMA journal_mode=WAL;").Error
	if err != nil {
		return nil, fmt.Errorf("error setting journal mode: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("error getting database: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	for _, model := range []any{
		Notification{},
		PlayerEvent{},
		Segment{},
		Thumbnail{},
		Identity{},
		Repo{},
		SigningKey{},
		Follow{},
		FeedPost{},
		Livestream{},
		Block{},
		ChatMessage{},
		ChatProfile{},
		oatproxy.OAuthSession{},
		ServerSettings{},
		XrpcStreamEvent{},
	} {
		err = db.AutoMigrate(model)
		if err != nil {
			return nil, err
		}
	}
	return &DBModel{DB: db}, nil
}
