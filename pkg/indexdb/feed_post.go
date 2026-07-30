package indexdb

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	glex "github.com/streamplace/glex/runtime"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/spid"
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

func (fp *FeedPost) ToPostView() (appbsky.FeedDefs_PostView, error) {
	rec, err := glex.CborDecodeValue(*fp.FeedPost)
	if err != nil {
		return appbsky.FeedDefs_PostView{}, fmt.Errorf("error decoding feed post: %w", err)
	}
	postView := appbsky.FeedDefs_PostView{
		LexiconTypeID: "app.bsky.feed.defs#postView",
		Cid:           fp.CID,
		Uri:           fp.URI,
		Author: appbsky.ActorDefs_ProfileViewBasic{
			Did: fp.RepoDID,
		},
		Record:    &glex.LexiconTypeDecoder{Val: rec},
		IndexedAt: fp.IndexedAt.UTC().Format(time.RFC3339Nano),
	}
	if fp.Repo != nil {
		postView.Author.Handle = fp.Repo.Handle
	}
	return postView, nil
}

// UpsertFeedPost indexes an app.bsky.feed.post record. typ is the
// indexer's annotation for the post's role in our feeds ("livestream",
// "reply") — it's caller knowledge, not part of the record. Reply
// linkage, CID, CBOR blob, and timestamps are derived here from the
// record and AT-URI.
func (m *DBModel) UpsertFeedPost(ctx context.Context, aturi syntax.ATURI, rec *appbsky.FeedPost, typ string) error {
	repoDID, err := aturi.Authority().AsDID()
	if err != nil {
		return fmt.Errorf("invalid ATURI authority: %w", err)
	}
	cid, err := spid.GetCID(rec)
	if err != nil {
		return fmt.Errorf("get feed post CID: %w", err)
	}
	createdAt, err := time.Parse(time.RFC3339, rec.CreatedAt)
	if err != nil {
		return fmt.Errorf("invalid feed post createdAt %q: %w", rec.CreatedAt, err)
	}
	var buf bytes.Buffer
	if err := rec.MarshalCBOR(&buf); err != nil {
		return fmt.Errorf("marshal feed post record: %w", err)
	}
	blob := buf.Bytes()
	now := time.Now().UTC()
	post := &FeedPost{
		URI:       aturi.String(),
		CID:       cid.String(),
		CreatedAt: createdAt,
		FeedPost:  &blob,
		RepoDID:   repoDID.String(),
		Type:      typ,
		IndexedAt: &now,
	}
	if rec.Reply != nil && rec.Reply.Root.Uri != "" {
		post.ReplyRootURI = &rec.Reply.Root.Uri
		if rootDID, err := syntax.ATURI(rec.Reply.Root.Uri).Authority().AsDID(); err == nil {
			did := rootDID.String()
			post.ReplyRootRepoDID = &did
		}
	}
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
		bskyPost, err := post.ToPostView()
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

	view, err := posts[0].ToPostView()
	if err != nil {
		return appbsky.FeedDefs_PostView{}, fmt.Errorf("error converting feed post to bsky post view: %w", err)
	}

	return view, nil
}
