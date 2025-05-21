package main

import (
	"encoding/json"
	"errors"
	"time"

	oauth_helpers "github.com/haileyok/atproto-oauth-golang/helpers"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"gorm.io/gorm"
)

type Key struct {
	ID        string `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Key       string
}

func (s *Store) GetKey(id string) (jwk.Key, error) {
	var key Key
	err := s.DB.Where("id = ?", id).First(&key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.GenerateKey(id)
	}
	return jwk.ParseKey([]byte(key.Key))
}

func (s *Store) GenerateKey(id string) (jwk.Key, error) {
	k, err := oauth_helpers.GenerateKey(nil)
	if err != nil {
		return nil, err
	}
	bs, err := json.Marshal(k)
	if err != nil {
		return nil, err
	}
	err = s.DB.Create(&Key{
		ID:  id,
		Key: string(bs),
	}).Error
	if err != nil {
		return nil, err
	}
	s.Logger.Info("generated key", "id", id)
	return k, nil
}
