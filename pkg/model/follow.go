package model

import (
	"context"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/api/bsky"
	"stream.place/streamplace/pkg/aqtime"
)

type Follow struct {
	UserDID    string `gorm:"primaryKey;index:user_idx;column:user_did"`
	SubjectDID string `gorm:"primaryKey;index:subject_idx;column:subject_did"`
	RKey       string `gorm:"index;column:rkey"`
	CreatedAt  time.Time
}

func (m *DBModel) CreateFollow(ctx context.Context, userDID, rkey string, follow *bsky.GraphFollow) error {
	at, err := aqtime.FromString(follow.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to parse follow createdAt: %w", err)
	}
	return m.DB.Save(&Follow{
		UserDID:    userDID,
		SubjectDID: follow.Subject,
		RKey:       rkey,
		CreatedAt:  at.Time(),
	}).Error
}

func (m *DBModel) DeleteFollow(ctx context.Context, userDID, rkey string) error {
	res := m.DB.Where("user_did = ? AND rkey = ?", userDID, rkey).Delete(&Follow{})
	if res.Error != nil {
		return fmt.Errorf("failed to delete follow: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("no follow found for userDID %s and rkey %s", userDID, rkey)
	}
	return nil
}

func (m *DBModel) GetUserFollowing(ctx context.Context, userDID string) ([]Follow, error) {
	var follows []Follow
	return follows, m.DB.Where("user_did = ?", userDID).Find(&follows).Error
}

func (m *DBModel) GetUserFollowers(ctx context.Context, userDID string) ([]Follow, error) {
	var follows []Follow
	return follows, m.DB.Where("subject_did = ?", userDID).Find(&follows).Error
}
