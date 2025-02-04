package model

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/lmittmann/tint"
	slogGorm "github.com/orandin/slog-gorm"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/log"
)

type DBModel struct {
	DB *gorm.DB
}

type Model interface {
	CreateNotification(token, repoDID string) error
	ListNotifications() ([]Notification, error)

	CreatePlayerEvent(event PlayerEventAPI) error
	ListPlayerEvents(playerId string) ([]PlayerEvent, error)
	PlayerReport(playerId string) (map[string]any, error)
	ClearPlayerEvents() error

	CreateSegment(segment *Segment) error
	MostRecentSegments() ([]Segment, error)
	LatestSegmentForUser(user string) (*Segment, error)
	CreateThumbnail(thumb *Thumbnail) error
	LatestThumbnailForUser(user string) (*Thumbnail, error)

	GetIdentity(id string) (*Identity, error)
	UpdateIdentity(ident *Identity) error

	GetRepo(did string) (*Repo, error)
	GetRepoByHandle(handle string) (*Repo, error)
	GetRepoByHandleOrDID(arg string) (*Repo, error)
	GetRepoBySigningKey(signingKey string) (*Repo, error)
	UpdateRepo(repo *Repo) error

	GetLiveUsers() ([]Segment, error)

	UpdateSigningKey(key *SigningKey) error
	GetSigningKey(did, repoDID string) (*SigningKey, error)
	GetSigningKeysForRepo(repoDID string) ([]SigningKey, error)

	CreateFollow(ctx context.Context, userDID, rev string, follow *bsky.GraphFollow) error
	GetUserFollowing(ctx context.Context, userDID string) ([]Follow, error)
	GetUserFollowers(ctx context.Context, userDID string) ([]Follow, error)
	DeleteFollow(ctx context.Context, userDID, rev string) error
	GetFollowersNotificationTokens(userDID string) ([]string, error)
}

func MakeDB(dbURL string) (Model, error) {
	log.Log(context.Background(), "starting database", "dbURL", dbURL)
	if !strings.HasPrefix(dbURL, "sqlite://") {
		dbURL = fmt.Sprintf("sqlite://%s", dbURL)
	}
	sqliteSuffix := dbURL[len("sqlite://"):]
	// if this isn't ":memory:", ensure that directory exists (eg, if db
	// file is being initialized)
	if !strings.Contains(sqliteSuffix, ":?") {
		os.MkdirAll(filepath.Dir(sqliteSuffix), os.ModePerm)
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
	for _, model := range []any{Notification{}, PlayerEvent{}, Segment{}, Thumbnail{}, Identity{}, Repo{}, SigningKey{}, Follow{}} {
		err = db.AutoMigrate(model)
		if err != nil {
			return nil, err
		}
	}
	return &DBModel{DB: db}, nil
}
