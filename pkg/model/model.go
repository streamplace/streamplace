package model

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aquareum.tv/aquareum/pkg/log"
	"github.com/lmittmann/tint"
	slogGorm "github.com/orandin/slog-gorm"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type DBModel struct {
	DB *gorm.DB
}

type Model interface {
	CreateNotification(token string) error
	ListNotifications() ([]Notification, error)

	CreatePlayerEvent(event PlayerEventAPI) error
	ListPlayerEvents(playerId string) ([]PlayerEvent, error)
	PlayerReport(playerId string) (map[string]float64, error)
	ClearPlayerEvents() error

	CreateSegment(segment *Segment) error
	MostRecentSegments() ([]Segment, error)

	CreateThumbnail(thumb *Thumbnail) error
	LatestThumbnailForUser(user string) (Thumbnail, error)
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

	gormLogger := slogGorm.New(slogGorm.WithHandler(tint.NewHandler(os.Stderr, &tint.Options{
		TimeFormat: time.RFC3339,
	})))

	db, err := gorm.Open(dial, &gorm.Config{
		SkipDefaultTransaction: true,
		TranslateError:         true,
		Logger:                 gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("error starting database: %w", err)
	}
	for _, model := range []any{Notification{}, PlayerEvent{}, Segment{}, Thumbnail{}} {
		err = db.AutoMigrate(model)
		if err != nil {
			return nil, err
		}
	}
	return &DBModel{DB: db}, nil
}
