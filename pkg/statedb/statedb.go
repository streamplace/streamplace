package statedb

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/plugin/prometheus"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/indexdb"
	"stream.place/streamplace/pkg/log"
	notificationpkg "stream.place/streamplace/pkg/notifications"
	"stream.place/streamplace/pkg/placestream"
)

type DBType string

const (
	DBTypeSQLite   DBType = "sqlite"
	DBTypePostgres DBType = "postgres"
)

// indexStore is the subset of the index database that statedb's queue
// processor and session handling read, declared consumer-side so statedb
// depends on five methods instead of the full indexdb.Model. Repo and
// Follow are indexer-state types with no lexicon view (see
// docs/architecture.md); everything else crosses the seam as placestream
// view types.
type indexStore interface {
	GetRepo(did string) (*indexdb.Repo, error)
	GetLivestream(uri string) (*placestream.Livestream_LivestreamView, error)
	GetLatestLivestreamForRepo(repoDID string) (*placestream.Livestream_LivestreamView, error)
	GetUserFollowers(ctx context.Context, userDID string) ([]indexdb.Follow, error)
	UpsertOwnMediaOrigin(ctx context.Context, serverDID, blobCID string, size int64, mimeType string) error
}

type StatefulDB struct {
	DB    *gorm.DB
	CLI   *config.CLI
	Type  DBType
	locks *NamedLocks
	noter notificationpkg.Notifier
	model indexStore
	// pokeQueue is used to wake up the queue processor when a new task is enqueued
	pokeQueue chan struct{}
	// pgLockConn is used to hold a connection to the database for locking
	pgLockConn   *gorm.DB
	pgLockConnMu sync.Mutex
	OATProxy     *oatproxy.OATProxy
	// vodProcessor runs the gstreamer + muxl + S3 pipeline for a VOD
	// upload task. Installed via SetVODProcessor at bootstrap so
	// pkg/statedb doesn't have to depend on the gstreamer-heavy pkg/vod.
	vodProcessor VODProcessor
	// viewCountAggregator collapses a window of view-log files into
	// place.stream.media.viewCount records. Installed via
	// SetViewCountAggregator at bootstrap so pkg/statedb doesn't have
	// to depend on the blob.Store-heavy pkg/viewlog.
	viewCountAggregator ViewCountAggregator
	// livestreamVODFinalizer concatenates a finished livestream's recorded
	// MUXL objects into a VOD. Installed via SetLivestreamVODFinalizer at
	// bootstrap, same indirection as vodProcessor.
	livestreamVODFinalizer LivestreamVODFinalizer
	// webhookSender delivers outbound webhooks for notification/chat/
	// stream-received tasks. Installed via SetWebhookSender at bootstrap
	// so pkg/statedb doesn't import the delivery integrations.
	webhookSender WebhookSender
}

// WebhookSender delivers outbound webhooks on behalf of the queue
// processor. Implemented by pkg/integrations/webhook and installed via
// SetWebhookSender.
type WebhookSender interface {
	SendChatWebhook(ctx context.Context, webhook *placestream.ServerDefs_Webhook, authorDID string, scm *placestream.ChatDefs_MessageView) error
	SendLivestreamWebhook(ctx context.Context, webhook *placestream.ServerDefs_Webhook, pdsURL string, lsv *placestream.Livestream_LivestreamView, postView *appbsky.FeedDefs_PostView, spcp *placestream.ChatProfile) error
	SendStreamReceivedWebhook(ctx context.Context, webhook *placestream.ServerDefs_Webhook, streamerDID string) error
}

// list tables here so we can migrate them
var StatefulDBModels = []any{
	oatproxy.OAuthSession{},
	Notification{},
	Config{},
	XrpcStreamEvent{},
	AppTask{},
	Repo{},
	Webhook{},
	MultistreamTarget{},
	MultistreamEvent{},
	BrandingBlob{},
	ModerationAuditLog{},
	Storage{},
	BroadcastOrigin{},
	S3Segment{},
	Upload{},
	DraftVideo{},
}

var NoPostgresDatabaseCode = "3D000"

// Stateful database for storing private streamplace state
func MakeDB(ctx context.Context, cli *config.CLI, noter notificationpkg.Notifier, model indexStore) (*StatefulDB, error) {
	dbURL := cli.DBURL
	log.Log(ctx, "starting stateful database", "dbURL", redactDBURL(dbURL))
	var dial gorm.Dialector
	var dbType DBType
	if dbURL == ":memory:" {
		dial = sqlite.Open(":memory:")
		dbType = DBTypeSQLite
	} else if strings.HasPrefix(dbURL, "sqlite://") {
		dial = sqlite.Open(dbURL[len("sqlite://"):])
		dbType = DBTypeSQLite
	} else if strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://") {
		dial = postgres.Open(dbURL)
		dbType = DBTypePostgres
	} else {
		return nil, fmt.Errorf("unsupported database URL (most start with sqlite:// or postgresql://): %s", redactDBURL(dbURL))
	}

	db, err := openDB(dial)

	if err != nil {
		if dbType == DBTypePostgres && strings.Contains(err.Error(), NoPostgresDatabaseCode) {
			db, err = makePostgresDB(dbURL)
			if err != nil {
				return nil, fmt.Errorf("error creating streamplace database: %w", err)
			}
		} else {
			return nil, fmt.Errorf("error starting database: %w", err)
		}
	}
	if dbType == DBTypeSQLite {
		err = db.Exec("PRAGMA journal_mode=WAL;").Error
		if err != nil {
			return nil, fmt.Errorf("error setting journal mode: %w", err)
		}
		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("error getting database: %w", err)
		}
		sqlDB.SetMaxOpenConns(1)
	}
	for _, model := range StatefulDBModels {
		err = db.AutoMigrate(model)
		if err != nil {
			return nil, err
		}
	}

	err = db.Use(prometheus.New(prometheus.Config{
		DBName:          "state",
		RefreshInterval: 10,
		StartServer:     false,
	}))
	if err != nil {
		return nil, fmt.Errorf("error using prometheus plugin: %w", err)
	}

	state := &StatefulDB{
		DB:        db,
		CLI:       cli,
		Type:      dbType,
		locks:     NewNamedLocks(),
		model:     model,
		pokeQueue: make(chan struct{}, 1),
		noter:     noter,
	}
	if state.Type == DBTypePostgres {
		err = state.startPostgresLockerConn(ctx)
		if err != nil {
			return nil, fmt.Errorf("error starting postgres locker connection: %w", err)
		}
	}
	return state, nil
}

func openDB(dial gorm.Dialector) (*gorm.DB, error) {
	return gorm.Open(dial, &gorm.Config{
		SkipDefaultTransaction: true,
		TranslateError:         true,
		Logger:                 config.GormLogger,
	})
}

// helper function for creating the requested postgres database
func makePostgresDB(dbURL string) (*gorm.DB, error) {
	u, err := url.Parse(dbURL)
	if err != nil {
		return nil, err
	}
	dbName := strings.TrimPrefix(u.Path, "/")
	u.Path = "/postgres"

	rootDial := postgres.Open(u.String())

	db, err := openDB(rootDial)
	if err != nil {
		return nil, err
	}

	// postgres doesn't support prepared statements for CREATE DATABASE. don't SQL inject yourself.
	err = db.Exec(fmt.Sprintf("CREATE DATABASE %s;", dbName)).Error
	if err != nil {
		return nil, err
	}

	log.Warn(context.Background(), "created postgres database", "dbName", dbName)

	realDial := postgres.Open(dbURL)

	return openDB(realDial)
}

func redactDBURL(dbURL string) string {
	u, err := url.Parse(dbURL)
	if err != nil {
		return "db url is malformed"
	}
	if u.User != nil {
		u.User = url.UserPassword(u.User.Username(), "redacted")
	}
	return u.String()
}
