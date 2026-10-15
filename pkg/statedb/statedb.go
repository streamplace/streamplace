package statedb

import (
	"context"
	"fmt"
	"io"
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
	"stream.place/streamplace/pkg/kvfs"
	"stream.place/streamplace/pkg/kvfs/slatedb"
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
	// closer releases what the database was opened on when the SQL driver is
	// not the only thing to close: a slatedb+ URL's store holds the object
	// store's single writer claim until it is closed.
	closer io.Closer
	// pokeQueue is used to wake up the queue processor when a new task is enqueued
	pokeQueue chan struct{}
	// pgLockConn is used to hold a connection to the database for locking
	pgLockConn   *gorm.DB
	pgLockConnMu sync.Mutex
	// brandingCache holds one brandingSnapshot per broadcaster (see
	// BrandingBlobs), so the many per-key reads of a page render cost one
	// query per TTL instead of one each.
	brandingCache sync.Map
	OATProxy      *oatproxy.OATProxy
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
	// videoPublisher publishes a finalized livestream VOD's video record
	// when the finalize task asks for it (SetVideoPublisher).
	videoPublisher VideoPublisher
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
	StreamViewTotal{},
	LivestreamViewTotal{},
	CertmagicItem{},
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
	var storeCloser io.Closer
	if dbURL == ":memory:" {
		dial = sqlite.Open(":memory:")
		dbType = DBTypeSQLite
	} else if strings.HasPrefix(dbURL, "sqlite://") {
		dial = sqlite.Open(dbURL[len("sqlite://"):])
		dbType = DBTypeSQLite
	} else if strings.HasPrefix(dbURL, "slatedb+") {
		// A SlateDB store: the SQLite database's files live in a key/value
		// database in object storage, which is what lets a node keep state
		// without a database server. Everything past the scheme is a URL the
		// store understands, most usefully slatedb+s3.
		var err error
		dial, storeCloser, err = slateDBDialector(dbURL)
		if err != nil {
			return nil, err
		}
		dbType = DBTypeSQLite
	} else if strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://") {
		dial = postgres.Open(dbURL)
		dbType = DBTypePostgres
	} else {
		return nil, fmt.Errorf("unsupported database URL (most start with sqlite://, slatedb+s3:// or postgresql://): %s", redactDBURL(dbURL))
	}

	// Opening the store is not the finish line: everything from here on can
	// still fail, and a slatedb+ store holds the database's single writer
	// claim, which a process that leaks it cannot open again. Every path that
	// gives the store up sets keepStore first.
	keepStore := false
	defer func() {
		if !keepStore && storeCloser != nil {
			_ = storeCloser.Close()
		}
	}()

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
	if err := migrateStreamViewTotals(ctx, db); err != nil {
		return nil, fmt.Errorf("carrying over stream view totals: %w", err)
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
		closer:    storeCloser,
	}
	if state.Type == DBTypePostgres {
		err = state.startPostgresLockerConn(ctx)
		if err != nil {
			return nil, fmt.Errorf("error starting postgres locker connection: %w", err)
		}
	}
	keepStore = true
	return state, nil
}

// Close releases the state database, including whatever it was opened on
// besides the SQL driver: a slatedb+ URL's store sits on the object store's
// single writer claim, which has to be given back before the same database can
// be opened again by this process.
func (state *StatefulDB) Close() error {
	sqlDB, err := state.DB.DB()
	if err != nil {
		return fmt.Errorf("error getting database: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("error closing database: %w", err)
	}
	state.pgLockConnMu.Lock()
	lockConn := state.pgLockConn
	state.pgLockConn = nil
	state.pgLockConnMu.Unlock()
	if lockConn != nil {
		if sqlDB, err := lockConn.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	if state.closer != nil {
		return state.closer.Close()
	}
	return nil
}

// slateDBFile is the name the SQLite database is stored under in a SlateDB
// store: one store holds one state database.
const slateDBFile = "state"

// slateDBDialector opens the SlateDB database named by a slatedb+ URL and
// returns the sqlite dialector that keeps the state database in it, plus the
// closer that gives the store's write claim back. The rest of the URL is the
// object store ("slatedb+s3://bucket/prefix?endpoint=..."): the bucket and
// prefix say where in the store the database lives, and the query string
// carries the connection options the object store's client wants. The VFS name
// is generated, so callers get their data source name from the VFS rather than
// the URL.
func slateDBDialector(dbURL string) (gorm.Dialector, io.Closer, error) {
	store, err := slatedb.Open(dbURL, slatedb.Options{})
	if err != nil {
		return nil, nil, err
	}
	vfs, err := kvfs.New(store, kvfs.Options{})
	if err != nil {
		// The store is already holding the database's writer claim.
		_ = store.Close()
		return nil, nil, err
	}
	return sqlite.Open(vfs.DSN(slateDBFile)), vfs, nil
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
		// Nodes of a station start together; one rebuilds, the others wait
		// for the lock and then find the hash index in place.
		if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "index-fix:"+i.name).Error; err != nil {
				return err
			}
			var def string
			if err := tx.Raw("SELECT indexdef FROM pg_indexes WHERE schemaname = CURRENT_SCHEMA() AND tablename = ? AND indexname = ?", i.table, i.name).Scan(&def).Error; err != nil {
				return err
			}
			if strings.Contains(def, "USING hash") {
				return nil // another node got there first
			}
			if err := tx.Exec(fmt.Sprintf(`DROP INDEX IF EXISTS %q`, i.name)).Error; err != nil {
				return fmt.Errorf("dropping index %s: %w", i.name, err)
			}
			if err := tx.Exec(fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %q ON %q USING hash (%q)`, i.name, i.table, i.column)).Error; err != nil {
				return fmt.Errorf("creating hash index %s: %w", i.name, err)
			}
			return nil
		}); err != nil {
			return fmt.Errorf("rebuilding index %s: %w", i.name, err)
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
	pgMinOpenConns    = 4
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
	// The advisory-lock connection is held for the process lifetime, and a
	// node needs a few queries in flight beside it. A cap an operator sets
	// below that is a misconfiguration to report, not a number to quietly
	// raise past what they sized max_connections for.
	if maxOpen < pgMinOpenConns {
		return fmt.Errorf("--db-max-open-conns must be at least %d (one connection holds the advisory lock for the node's lifetime); got %d", pgMinOpenConns, maxOpen)
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

// redactDBURL strips the secrets a database URL can carry before the URL goes
// into a log line or an error: the password in the userinfo, and the store
// options kvfs.RedactURLSecrets knows about (an object store URL carries its
// credentials in the query string).
func redactDBURL(dbURL string) string {
	u, err := url.Parse(dbURL)
	if err != nil {
		return "db url is malformed"
	}
	if u.User != nil {
		u.User = url.UserPassword(u.User.Username(), "redacted")
		dbURL = u.String()
	}
	return kvfs.RedactURLSecrets(dbURL)
}
