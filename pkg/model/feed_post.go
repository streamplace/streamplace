package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"stream.place/streamplace/pkg/appbsky"
	glexrt "github.com/streamplace/glex/runtime"
	"gorm.io/gorm"
)

type FeedPost struct {
	URI              string     `json:"uri" gorm:"primaryKey;column:uri"`
	CID              string     `json:"cid" gorm:"column:cid"`
	CreatedAt        time.Time  `json:"createdAt" gorm:"column:created_at;index:recent_replies"`
	FeedPost         *[]byte    `json:"feedPost" gorm:"column:feed_post"`
	RepoDID          string     `json:"repoDID"              gorm:"column:repo_did"`
	Repo             *Repo      `json:"repo,omitempty"       gorm:"foreignKey:DID;references:RepoDID"`
	Type             string     `json:"type"                 gorm:"column:type"`
	ReplyRootURI     *string    `json:"replyRootURI,omitempty" gorm:"column:reply_root_uri"`
	ReplyRoot        *FeedPost  `json:"replyRoot,omitempty" gorm:"foreignKey:uri;references:ReplyRootURI"`
	ReplyRootRepoDID *string    `json:"replyRootRepoDID,omitempty" gorm:"column:reply_root_repo_did;index:recent_replies"`
	ReplyRootRepo    *Repo      `json:"replyRootRepo,omitempty" gorm:"foreignKey:DID;references:ReplyRootRepoDID"`
	IndexedAt        *time.Time `json:"indexedAt,omitempty" gorm:"column:indexed_at"`
}

func (fp *FeedPost) ToBskyPostView() (appbsky.FeedDefs_PostView, error) {
	rec, err := glexrt.CborDecodeValue(*fp.FeedPost)
	if err != nil {
		return appbsky.FeedDefs_PostView{}, fmt.Errorf("error decoding feed post: %w", err)
	}
	postView := appbsky.FeedDefs_PostView{
		LexiconTypeID: "app.appbsky.feed.defs#postView",
		Cid:           fp.CID,
		Uri:           fp.URI,
		Author: appbsky.ActorDefs_ProfileViewBasic{
			Did: fp.RepoDID,
		},
		Record:    &glexrt.LexiconTypeDecoder{Val: rec},
		IndexedAt: fp.IndexedAt.UTC().Format(time.RFC3339Nano),
	}
	if fp.Repo != nil {
		postView.Author.Handle = fp.Repo.Handle
	}
	return postView, nil
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

func (m *DBModel) GetFeedPost(uri string) (*FeedPost, error) {
	post := FeedPost{}
	err := m.DB.Where("uri = ?", uri).First(&post).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error retrieving feed post: %w", err)
	}
	return &post, nil
}

func (m *DBModel) GetReplies(repoDID string) ([]appbsky.FeedDefs_PostView, error) {
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
	bskyPosts := []appbsky.FeedDefs_PostView{}
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

func (m *DBModel) GetLatestLivestream(repoDID string) (appbsky.FeedDefs_PostView, error) {
	posts := []FeedPost{}
	err := m.DB.
		Preload("Repo").
		Where("type = ?", "livestream").
		Where("repo_did = ?", repoDID).
		Limit(1).
		Order("created_at DESC").
		Find(&posts).Error
	if err != nil {
		return appbsky.FeedDefs_PostView{}, fmt.Errorf("error retrieving livestream: %w", err)
	}

	if len(posts) == 0 {
		return appbsky.FeedDefs_PostView{}, nil
	}

	view, err := posts[0].ToBskyPostView()
	if err != nil {
		return appbsky.FeedDefs_PostView{}, fmt.Errorf("error converting feed post to bsky post view: %w", err)
	}

	return view, nil
}
