package localdb

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// ViewLogSalt is the per-UTC-day HMAC salt used to anonymize IP
// addresses in the view-log pipeline. Same date returns the same salt,
// so two requests from the same IP within a day share a hash; across
// days the hash changes because the salt rotates. Cleaning up old rows
// makes historical logs unrecoverable.
type ViewLogSalt struct {
	Date string `gorm:"primaryKey;column:date"`
	Salt []byte `gorm:"column:salt"`
}

func (m *LocalDatabase) GetViewLogSalt(date string) ([]byte, error) {
	var row ViewLogSalt
	err := m.DB.Where("date = ?", date).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get view log salt: %w", err)
	}
	return row.Salt, nil
}

func (m *LocalDatabase) PutViewLogSalt(date string, salt []byte) error {
	return m.DB.Save(&ViewLogSalt{Date: date, Salt: salt}).Error
}

func (m *LocalDatabase) DeleteViewLogSaltsBefore(date string) error {
	return m.DB.Where("date < ?", date).Delete(&ViewLogSalt{}).Error
}
