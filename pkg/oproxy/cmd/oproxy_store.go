package main

import (
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
	slogGorm "github.com/orandin/slog-gorm"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/oproxy"
)

type Store struct {
	DB     *gorm.DB
	Logger *slog.Logger
}

func NewStore(dbPath string, logger *slog.Logger, verbose bool) (*Store, error) {
	gormLogger := slogGorm.New(
		slogGorm.WithHandler(tint.NewHandler(os.Stderr, &tint.Options{
			TimeFormat: time.RFC3339,
		})),
	)
	if verbose {
		gormLogger = slogGorm.New(
			slogGorm.WithHandler(tint.NewHandler(os.Stderr, &tint.Options{
				TimeFormat: time.RFC3339,
			})),
			slogGorm.WithTraceAll(),
		)
	}
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, err
	}
	db.AutoMigrate(&oproxy.OAuthSession{}, &Key{})
	return &Store{DB: db, Logger: logger}, nil
}

func (s *Store) CreateOAuthSession(id string, session *oproxy.OAuthSession) error {
	return s.DB.Create(session).Error
}

func (s *Store) GetOAuthSession(id string) (*oproxy.OAuthSession, error) {
	var session oproxy.OAuthSession
	if err := s.DB.Where("downstream_dpop_jkt = ?", id).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

func (s *Store) UpdateOAuthSession(id string, session *oproxy.OAuthSession) error {
	res := s.DB.Model(&oproxy.OAuthSession{}).Where("downstream_dpop_jkt = ?", id).Updates(session)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("no rows affected")
	}
	return nil
}
