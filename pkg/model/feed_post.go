package model

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/atproto/data"
	"gorm.io/gorm"
)

type FeedPost struct {
	CID              string    `json:"cid" gorm:"primaryKey;column:cid"`
	URI              string    `json:"uri"`
	CreatedAt        time.Time `json:"createdAt" gorm:"column:created_at;index:recent_replies"`
	FeedPost         *[]byte   `json:"feedPost"`
	RepoDID          string    `json:"repoDID"              gorm:"column:repo_did"`
	Repo             *Repo     `json:"repo,omitempty"       gorm:"foreignKey:DID;references:RepoDID"`
	Type             string    `json:"type"                 gorm:"column:type"`
	ReplyRootCID     *string   `json:"replyRootCID,omitempty" gorm:"column:reply_root_cid"`
	ReplyRoot        *FeedPost `json:"replyRoot,omitempty" gorm:"foreignKey:cid;references:ReplyRootCID"`
	ReplyRootRepoDID *string   `json:"replyRootRepoDID,omitempty" gorm:"column:reply_root_repo_did;index:recent_replies"`
	ReplyRootRepo    *Repo     `json:"replyRootRepo,omitempty" gorm:"foreignKey:DID;references:ReplyRootRepoDID"`
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

func (m *DBModel) GetReplies(repoDID string) ([]FeedPost, error) {
	posts := []FeedPost{}
	err := m.DB.
		Preload("Repo").
		Where("reply_root_repo_did = ? AND type = ?", repoDID, "reply").
		Limit(100).
		Order("created_at DESC").
		Find(&posts).Error
	if err != nil {
		return nil, fmt.Errorf("error retrieving replies: %w", err)
	}

	return posts, nil
}

type StreamplaceFeedPostLivestream struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

type StreamplaceFeedPost struct {
	bsky.FeedPost
	Livestream *StreamplaceFeedPostLivestream `json:"place.stream.livestream,omitempty"`
}

func (m *DBModel) GetLatestLivestream(repoDID string) (map[string]any, error) {
	posts := []FeedPost{}
	err := m.DB.
		Preload("Repo").
		Where("type = ?", "livestream").
		Limit(1).
		Order("created_at DESC").
		Find(&posts).Error
	if err != nil {
		return nil, fmt.Errorf("error retrieving livestream: %w", err)
	}

	if len(posts) == 0 {
		return nil, nil
	}

	d, err := data.UnmarshalCBOR(*posts[0].FeedPost)
	if err != nil {
		slog.Warn("failed to parse record CBOR")
		return nil, fmt.Errorf("error decoding livestream: %w", err)
	}

	return d, nil
}
