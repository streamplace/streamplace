package statedb

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// createSharedConfig returns the winner of concurrent first-use key generation.
func (state *StatefulDB) createSharedConfig(ctx context.Context, key string, value []byte) ([]byte, error) {
	row := Config{Key: key, Value: value}
	db := state.DB.WithContext(ctx)
	res := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if res.Error != nil || res.RowsAffected == 1 {
		return value, res.Error
	}
	if err := db.First(&row, "key = ?", key).Error; err != nil {
		return nil, err
	}
	return row.Value, nil
}
