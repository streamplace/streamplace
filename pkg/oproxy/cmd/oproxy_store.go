package main

import (
	"errors"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/oproxy"
)

type Store struct {
	DB *gorm.DB
}

func NewStore(dbPath string) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return &Store{DB: db}, nil
}

func (s *Store) CreateOAuthSession(id string, session *oproxy.OAuthSession) error {
	return s.DB.Create(session).Error
}

func (s *Store) LoadOAuthSession(id string) (*oproxy.OAuthSession, error) {
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
