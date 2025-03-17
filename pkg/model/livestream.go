package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/api/bsky"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/streamplace"
)

type Livestream struct {
	CID        string    `json:"cid" gorm:"primaryKey;column:cid"`
	URI        string    `json:"uri"`
	CreatedAt  time.Time `json:"createdAt" gorm:"column:created_at;index:idx_repo_created,priority:2"`
	Livestream *[]byte   `json:"livestream"`
	RepoDID    string    `json:"repoDID" gorm:"column:repo_did;index:idx_repo_created,priority:1"`
	Repo       *Repo     `json:"repo,omitempty" gorm:"foreignKey:DID;references:RepoDID"`
	Post       *FeedPost `json:"post,omitempty" gorm:"foreignKey:CID;references:PostCID"`
	PostCID    string    `json:"postCID" gorm:"column:post_cid;index:idx_post_cid"`
	PostURI    string    `json:"postURI" gorm:"column:post_uri"`
}

func (ls *Livestream) ToLivestreamView() (*streamplace.Livestream_LivestreamView, error) {
	rec, err := lexutil.CborDecodeValue(*ls.Livestream)
	if err != nil {
		return nil, fmt.Errorf("error decoding feed post: %w", err)
	}
	postView := streamplace.Livestream_LivestreamView{
		LexiconTypeID: "place.stream.livestream#livestreamView",
		Cid:           ls.CID,
		Uri:           ls.URI,
		Author: &bsky.ActorDefs_ProfileViewBasic{
			Did:    ls.RepoDID,
			Handle: ls.Repo.Handle,
		},
		Record:    &lexutil.LexiconTypeDecoder{Val: rec},
		IndexedAt: time.Now().Format(time.RFC3339),
	}
	return &postView, nil
}

func (m *DBModel) CreateLivestream(ctx context.Context, ls *Livestream) error {
	return m.DB.Create(ls).Error
}

// GetLatestLivestreamForRepo returns the most recent livestream for a given repo DID
func (m *DBModel) GetLatestLivestreamForRepo(repoDID string) (*Livestream, error) {
	var livestream Livestream
	err := m.DB.
		Preload("Repo").
		Where("repo_did = ?", repoDID).
		Order("created_at DESC").
		First(&livestream).Error
	if err != nil {
		return nil, fmt.Errorf("error retrieving latest livestream: %w", err)
	}
	return &livestream, nil
}

func (m *DBModel) GetLivestreamByPostCID(postCID string) (*Livestream, error) {
	var livestream Livestream
	err := m.DB.
		Preload("Repo").
		Where("post_cid = ?", postCID).
		First(&livestream).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error retrieving livestream by postCID: %w", err)
	}
	return &livestream, nil
}
