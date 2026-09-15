package statedb

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/plugin/prometheus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	notificationpkg "stream.place/streamplace/pkg/notifications"
)

type DBType string

const (
	DBTypeSQLite   DBType = "sqlite"
	DBTypePostgres DBType = "postgres"
)

type StatefulDB struct {
	DB    *gorm.DB
	CLI   *config.CLI
	Type  DBType
	locks *NamedLocks
	noter notificationpkg.Notifier
	model model.Model
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
	// cdnLogIngester pulls archived CDN access logs into the view-log
	// store; nil when no CDN log source is configured.
	cdnLogIngester CDNLogIngester
	// livestreamVODFinalizer concatenates a finished livestream's recorded
	// MUXL objects into a VOD. Installed via SetLivestreamVODFinalizer at
	// bootstrap, same indirection as vodProcessor.
	livestreamVODFinalizer LivestreamVODFinalizer
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
	CDNLogPart{},
}

var NoPostgresDatabaseCode = "3D000"

// Stateful database for storing private streamplace state
func MakeDB(ctx context.Context, cli *config.CLI, noter notificationpkg.Notifier, model model.Model) (*StatefulDB, error) {
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
		if err := sqlitePragmas(db); err != nil {
			return nil, err
		}
		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("error getting database: %w", err)
		}
		sqlDB.SetMaxOpenConns(1)
	}
	if dbType == DBTypePostgres {
		if err := boundPostgresPool(ctx, db, cli.DBMaxOpenConns); err != nil {
			return nil, err
		}
	}
	for _, model := range StatefulDBModels {
		err = db.AutoMigrate(model)
		if err != nil {
			return nil, err
		}
	}
	if dbType == DBTypePostgres {
		if err := postgresIndexFixes(ctx, db); err != nil {
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

// postgresIndexFixes swaps indexes AutoMigrate builds as btrees for hash
// indexes where the column is a long token looked up only by equality: a
// btree entry tops out at ~2.7KB and a downstream OAuth access token (a JWT
// carrying the session's whole scope list) can be bigger, which makes the
// INSERT of a perfectly good session fail. AutoMigrate leaves an index alone
// once one of that name exists, so this holds across restarts.
func postgresIndexFixes(ctx context.Context, db *gorm.DB) error {
	type ix struct{ table, column, name string }
	for _, i := range []ix{
		{"oauth_sessions", "downstream_access_token", "idx_oauth_sessions_downstream_access_token"},
	} {
		var def string
		err := db.WithContext(ctx).Raw(
			"SELECT indexdef FROM pg_indexes WHERE schemaname = CURRENT_SCHEMA() AND tablename = ? AND indexname = ?", i.table, i.name,
		).Scan(&def).Error
		if err != nil {
			return fmt.Errorf("checking index %s: %w", i.name, err)
		}
		if def == "" || strings.Contains(def, "USING hash") {
			continue
		}
		log.Log(ctx, "rebuilding index as hash (btree can't hold long tokens)", "index", i.name)
		if err := db.WithContext(ctx).Exec(fmt.Sprintf(`DROP INDEX IF EXISTS %q`, i.name)).Error; err != nil {
			return fmt.Errorf("dropping index %s: %w", i.name, err)
		}
		if err := db.WithContext(ctx).Exec(fmt.Sprintf(`CREATE INDEX %q ON %q USING hash (%q)`, i.name, i.table, i.column)).Error; err != nil {
			return fmt.Errorf("creating hash index %s: %w", i.name, err)
		}
	}
	return nil
}

// sqlitePragmas applies the two settings a sqlite state database needs: WAL, so
// readers do not block the writer, and a busy timeout, so a writer in another
// process (`streamplace sync`, warming a new index) is waited for instead of
// erroring out. It is a function rather than two lines in MakeDB because
// MakeDB's `model` parameter shadows the package the timeout lives in.
func sqlitePragmas(db *gorm.DB) error {
	if err := db.Exec("PRAGMA journal_mode=WAL;").Error; err != nil {
		return fmt.Errorf("error setting journal mode: %w", err)
	}
	return model.SetSQLiteBusyTimeout(db)
}

func openDB(dial gorm.Dialector) (*gorm.DB, error) {
	return gorm.Open(dial, &gorm.Config{
		SkipDefaultTransaction: true,
		TranslateError:         true,
		Logger:                 config.GormLogger,
	})
}

// Postgres pool bounds. database/sql opens connections without limit by
// default, so a station of N nodes under load could each grab as many as
// the server allows and collectively hit max_connections — at which point
// every node fails (SQLSTATE 53300) at once. A cap per node means a busy
// node queues on its own pool instead. Idle connections are trimmed so a
// quiet node doesn't sit on its whole allowance, and connections are
// recycled hourly so a failover or pooler restart is followed.
const (
	pgMaxIdleConns    = 8
	pgConnMaxIdleTime = 5 * time.Minute
	pgConnMaxLifetime = time.Hour
)

func boundPostgresPool(ctx context.Context, db *gorm.DB, maxOpen int) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("error getting database: %w", err)
	}
	if maxOpen <= 0 {
		maxOpen = 30
	}
	// The advisory-lock connection is held for the process lifetime; keep
	// room for at least a few queries beside it.
	if maxOpen < 4 {
		maxOpen = 4
	}
	idle := pgMaxIdleConns
	if idle > maxOpen {
		idle = maxOpen
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(idle)
	sqlDB.SetConnMaxIdleTime(pgConnMaxIdleTime)
	sqlDB.SetConnMaxLifetime(pgConnMaxLifetime)
	log.Log(ctx, "postgres pool bounded", "maxOpen", maxOpen, "maxIdle", idle, "idleTime", pgConnMaxIdleTime, "lifetime", pgConnMaxLifetime)
	return nil
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
	// The bootstrap connection to the maintenance database isn't needed
	// again; left open it would count against max_connections for the
	// life of the process.
	if rootSQL, dbErr := db.DB(); dbErr == nil {
		rootSQL.Close()
	}
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
