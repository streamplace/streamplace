package model

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

type FeedPost struct {
	CID          string    `json:"cid" gorm:"primaryKey;column:cid"`
	URI          string    `json:"uri"`
	FeedPost     *[]byte   `json:"feedPost"`
	RepoDID      string    `json:"repoDID"              gorm:"column:repo_did"`
	Repo         *Repo     `json:"repo,omitempty"       gorm:"foreignKey:DID;references:RepoDID"`
	Type         string    `json:"type"                 gorm:"column:type"`
	ReplyRootCID *string   `json:"replyRootCID,omitempty" gorm:"column:reply_root_cid"`
	ReplyRoot    *FeedPost `json:"replyRoot,omitempty" gorm:"foreignKey:ReplyRootCID;references:cid"`
}

func (m *DBModel) CreateFeedPost(ctx context.Context, post *FeedPost) error {
	return m.DB.Create(post).Error
}

func (m *DBModel) ListFeedPosts() ([]FeedPost, error) {
	posts := []FeedPost{}
	err := m.DB.Find(&posts).Error
	if err != nil {
		return nil, fmt.Errorf("error retrieving chat posts: %w", err)
	}
	return posts, nil
}

func (m *DBModel) GetFeedPost(cid string) (*FeedPost, error) {
	post := FeedPost{}
	err := m.DB.Where("CID = ?", cid).First(&post).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error retrieving feed post: %w", err)
	}
	return &post, nil
}
