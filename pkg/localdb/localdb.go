package localdb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/plugin/prometheus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
)

type LocalDB interface {
	CreateSegment(segment *Segment) error
	MostRecentSegments() ([]Segment, error)
	LatestSegmentForUser(user string) (*Segment, error)
	LatestSegmentsForUser(user string, limit int, includeUnpublished bool, before *time.Time, after *time.Time) ([]Segment, error)
	FilterLiveRepoDIDs(repoDIDs []string) ([]string, error)
	CreateThumbnail(thumb *Thumbnail) error
	LatestThumbnailForUser(user string) (*Thumbnail, error)
	GetSegment(id string) (*Segment, error)
	GetExpiredSegments(ctx context.Context) ([]Segment, error)
	DeleteSegment(ctx context.Context, id string) error
	StartSegmentCleaner(ctx context.Context) error
	SegmentCleaner(ctx context.Context) error
	GetViewLogSalt(date string) ([]byte, error)
	PutViewLogSalt(date string, salt []byte) error
	DeleteViewLogSaltsBefore(date string) error
}

type LocalDatabase struct {
	DB *gorm.DB
}

func MakeDB(dbURL string) (LocalDB, error) {
	log.Log(context.Background(), "starting database", "dbURL", dbURL)
	if strings.HasPrefix(dbURL, "sqlite://") {
		dbURL = dbURL[len("sqlite://"):]
	} else if dbURL != ":memory:" {
		return nil, fmt.Errorf("unsupported database URL (most start with sqlite://): %s", dbURL)
	}
	dial := sqlite.Open(dbURL)

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
		DBName:          "localdb",
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
		Segment{},
		Thumbnail{},
		ViewLogSalt{},
	} {
		err = db.AutoMigrate(model)
		if err != nil {
			return nil, err
		}
	}
	return &LocalDatabase{DB: db}, nil
}
