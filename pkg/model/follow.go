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
	Rev        string `gorm:"index;column:rev"`
	CreatedAt  time.Time
}

func (m *DBModel) CreateFollow(ctx context.Context, userDID, rev string, follow *bsky.GraphFollow) error {
	at, err := aqtime.FromString(follow.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to parse follow createdAt: %w", err)
	}
	return m.DB.Save(&Follow{
		UserDID:    userDID,
		SubjectDID: follow.Subject,
		Rev:        rev,
		CreatedAt:  at.Time(),
	}).Error
}

func (m *DBModel) DeleteFollow(ctx context.Context, userDID, rev string) error {
	return m.DB.Where("user_did = ? AND rev = ?", userDID, rev).Delete(&Follow{}).Error
}

func (m *DBModel) GetUserFollowing(ctx context.Context, userDID string) ([]Follow, error) {
	var follows []Follow
	return follows, m.DB.Where("user_did = ?", userDID).Find(&follows).Error
}

func (m *DBModel) GetUserFollowers(ctx context.Context, userDID string) ([]Follow, error) {
	var follows []Follow
	return follows, m.DB.Where("subject_did = ?", userDID).Find(&follows).Error
}
