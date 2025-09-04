package statedb

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type Config struct {
	Key       string    `gorm:"column:key;primarykey"`
	Value     []byte    `gorm:"column:value"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (state *StatefulDB) GetConfig(key string) (*Config, error) {
	var config Config
	if err := state.DB.Where("key = ?", key).First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

func (state *StatefulDB) PutConfig(key string, value []byte) error {
	config := Config{
		Key:   key,
		Value: value,
	}
	return state.DB.Save(&config).Error
}
