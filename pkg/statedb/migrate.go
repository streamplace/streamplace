package statedb

import (
	"context"
	"os"
	"time"

	"github.com/lmittmann/tint"
	slogGorm "github.com/orandin/slog-gorm"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
)

func Migrate(cli *config.CLI) error {
	gormLogger := slogGorm.New(
		slogGorm.WithHandler(tint.NewHandler(os.Stderr, &tint.Options{
			TimeFormat: time.RFC3339,
		})),
		// slogGorm.WithTraceAll(),
	)

	newDB, err := MakeDB(context.Background(), cli, nil, nil)
	if err != nil {
		return err
	}

	oldDB, err := gorm.Open(sqlite.Open(cli.DataFilePath([]string{"db.sqlite"})), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return err
	}

	var sessions []oatproxy.OAuthSession
	if err := oldDB.Find(&sessions).Error; err != nil {
		return err
	}

	for _, session := range sessions {
		log.Log(context.Background(), "migrating session", "session", session.DownstreamDPoPJKT)
		err := newDB.DB.Save(&session).Error
		if err != nil {
			return err
		}
	}

	var notifications []Notification
	if err := oldDB.Find(&notifications).Error; err != nil {
		time.Sleep(1 * time.Second)
		return err
	}

	for _, notification := range notifications {
		log.Log(context.Background(), "migrating notification", "notification", notification)
		err := newDB.DB.Save(&notification).Error
		if err != nil {
			return err
		}
	}

	var repos []model.Repo
	if err := oldDB.Find(&repos).Error; err != nil {
		time.Sleep(1 * time.Second)
		return err
	}

	for _, repo := range repos {
		newRepo := Repo{
			DID:       repo.DID,
			IndexedAt: time.Now().UTC(),
		}
		log.Log(context.Background(), "migrating repo", "repo", repo)
		err := newDB.DB.Save(&newRepo).Error
		if err != nil {
			return err
		}
	}

	return nil
}
