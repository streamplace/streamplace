package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/api/bsky"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"gorm.io/gorm"
)

type FeedPost struct {
	CID              string     `json:"cid" gorm:"primaryKey;column:cid"`
	URI              string     `json:"uri"`
	CreatedAt        time.Time  `json:"createdAt" gorm:"column:created_at;index:recent_replies"`
	FeedPost         *[]byte    `json:"feedPost"`
	RepoDID          string     `json:"repoDID"              gorm:"column:repo_did"`
	Repo             *Repo      `json:"repo,omitempty"       gorm:"foreignKey:DID;references:RepoDID"`
	Type             string     `json:"type"                 gorm:"column:type"`
	ReplyRootCID     *string    `json:"replyRootCID,omitempty" gorm:"column:reply_root_cid"`
	ReplyRoot        *FeedPost  `json:"replyRoot,omitempty" gorm:"foreignKey:cid;references:ReplyRootCID"`
	ReplyRootRepoDID *string    `json:"replyRootRepoDID,omitempty" gorm:"column:reply_root_repo_did;index:recent_replies"`
	ReplyRootRepo    *Repo      `json:"replyRootRepo,omitempty" gorm:"foreignKey:DID;references:ReplyRootRepoDID"`
	IndexedAt        *time.Time `json:"indexedAt,omitempty" gorm:"column:indexed_at"`
}

func (fp *FeedPost) ToBskyPostView() (*bsky.FeedDefs_PostView, error) {
	rec, err := lexutil.CborDecodeValue(*fp.FeedPost)
	if err != nil {
		return nil, fmt.Errorf("error decoding feed post: %w", err)
	}
	postView := bsky.FeedDefs_PostView{
		LexiconTypeID: "app.bsky.feed.defs#postView",
		Cid:           fp.CID,
		Uri:           fp.URI,
		Author: &bsky.ActorDefs_ProfileViewBasic{
			Did:    fp.RepoDID,
			Handle: fp.Repo.Handle,
		},
		Record:    &lexutil.LexiconTypeDecoder{Val: rec},
		IndexedAt: time.Now().Format(time.RFC3339),
	}
	return &postView, nil
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
func (m *DBModel) ListFeedPostsByType(feedType string, limit int, after int64) ([]FeedPost, error) {
	if after == 0 {
		after = time.Now().Add(48 * time.Hour).UnixMilli()
	}
	time := time.UnixMilli(after)
	posts := []FeedPost{}
	// exclude scumb.ag for now (so my dev streams don't show up)
	err := m.DB.Where("type = ? AND created_at < ? AND repo_did != ?", feedType, time.UTC(), "did:plc:dkh4rwafdcda4ko7lewe43ml").
		Order("created_at DESC").
		Group("uri").
		Limit(limit).
		Find(&posts).Error
	if err != nil {
		return nil, fmt.Errorf("error retrieving feed posts: %w", err)
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

func (m *DBModel) GetReplies(repoDID string) ([]*bsky.FeedDefs_PostView, error) {
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
	bskyPosts := []*bsky.FeedDefs_PostView{}
	for _, post := range posts {
		bskyPost, err := post.ToBskyPostView()
		if err != nil {
			return nil, fmt.Errorf("error converting feed post to bsky post view: %w", err)
		}
		bskyPosts = append(bskyPosts, bskyPost)
	}
	return bskyPosts, nil
}

type StreamplaceFeedPostLivestream struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

func (m *DBModel) GetLatestLivestream(repoDID string) (*bsky.FeedDefs_PostView, error) {
	posts := []FeedPost{}
	err := m.DB.
		Preload("Repo").
		Where("type = ?", "livestream").
		Where("repo_did = ?", repoDID).
		Limit(1).
		Order("created_at DESC").
		Find(&posts).Error
	if err != nil {
		return nil, fmt.Errorf("error retrieving livestream: %w", err)
	}

	if len(posts) == 0 {
		return nil, nil
	}

	view, err := posts[0].ToBskyPostView()
	if err != nil {
		return nil, fmt.Errorf("error converting feed post to bsky post view: %w", err)
	}

	return view, nil
}
