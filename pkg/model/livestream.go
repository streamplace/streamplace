package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	glexrt "github.com/streamplace/glex/runtime"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/moderation"
	"stream.place/streamplace/pkg/placestream"
)

type Livestream struct {
	URI        string    `json:"uri" gorm:"primaryKey;column:uri"`
	CID        string    `json:"cid" gorm:"column:cid"`
	CreatedAt  time.Time `json:"createdAt" gorm:"column:created_at;index:idx_repo_created,priority:2"`
	Livestream *[]byte   `json:"livestream"`
	RepoDID    string    `json:"repoDID" gorm:"column:repo_did;index:idx_repo_created,priority:1"`
	Repo       *Repo     `json:"repo,omitempty" gorm:"foreignKey:DID;references:RepoDID"`
	Post       *FeedPost `json:"post,omitempty" gorm:"foreignKey:CID;references:PostCID"`
	PostCID    string    `json:"postCID" gorm:"column:post_cid"`
	PostURI    string    `json:"postURI" gorm:"column:post_uri;index:idx_post_uri"`
}

func (ls *Livestream) ToLivestreamView() (placestream.Livestream_LivestreamView, error) {
	if ls == nil || ls.Livestream == nil {
		return placestream.Livestream_LivestreamView{}, fmt.Errorf("livestream record is nil")
	}
	rec, err := glexrt.CborDecodeValue(*ls.Livestream)
	if err != nil {
		return placestream.Livestream_LivestreamView{}, fmt.Errorf("error decoding feed post: %w", err)
	}
	if typedRec, ok := rec.(placestream.Livestream); ok {
		typedRec.Tags = moderation.FilterTags(typedRec.Tags)
	}
	postView := placestream.Livestream_LivestreamView{
		LexiconTypeID: "place.stream.livestream#livestreamView",
		Cid:           ls.CID,
		Uri:           ls.URI,
		Author: appbsky.ActorDefs_ProfileViewBasic{
			Did:    ls.RepoDID,
			Handle: ls.Repo.Handle,
		},
		Record:    &glexrt.LexiconTypeDecoder{Val: rec},
		IndexedAt: time.Now().Format(time.RFC3339),
	}
	return postView, nil
}

func (m *DBModel) CreateLivestream(ctx context.Context, ls *Livestream) error {
	// upsert livestream record, actually
	return m.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "uri"}},
		DoUpdates: clause.AssignmentColumns([]string{"cid", "created_at", "livestream", "repo_did", "post_cid", "post_uri"}),
	}).Create(ls).Error
}

func (m *DBModel) GetLivestream(uri string) (*Livestream, error) {
	var livestream Livestream
	err := m.DB.
		Preload("Repo").
		Preload("Post").
		Where("uri = ?", uri).
		First(&livestream).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error retrieving livestream by uri: %w", err)
	}
	return &livestream, nil
}

// GetLatestLivestreamForRepo returns the most recent livestream for a given repo DID
func (m *DBModel) GetLatestLivestreamForRepo(repoDID string) (*Livestream, error) {
	var livestream Livestream
	err := m.DB.
		Preload("Repo").
		Preload("Post").
		Where("repo_did = ?", repoDID).
		Order("created_at DESC").
		First(&livestream).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error retrieving latest livestream: %w", err)
	}
	return &livestream, nil
}

func (m *DBModel) GetLivestreamByPostURI(postURI string) (*Livestream, error) {
	var livestream Livestream
	err := m.DB.
		Preload("Repo").
		Preload("Post").
		Where("post_uri = ?", postURI).
		First(&livestream).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error retrieving livestream by postURI: %w", err)
	}
	return &livestream, nil
}

// Get the latest livestreams for a given list of repo DIDs
func (m *DBModel) GetLatestLivestreams(limit int, before *time.Time, dids []string) ([]Livestream, error) {
	var recentLivestreams []Livestream
	now := time.Now().UTC()

	if len(dids) == 0 {
		return []Livestream{}, nil
	}

	// Subquery to get the most recent livestream for each repo_did
	subQuery := m.DB.
		Table("livestreams").
		Select("MAX(created_at) as max_created_at, repo_did").
		Where("repo_did IN ?", dids).
		Group("repo_did")

	mainQuery := m.DB.
		Table("livestreams").
		Select("livestreams.*").
		Joins("JOIN (?) as sq ON livestreams.repo_did = sq.repo_did AND livestreams.created_at = sq.max_created_at", subQuery).
		Where("livestreams.repo_did IN ?", dids).
		// exclude livestreams with !hide label on the record
		Where("NOT EXISTS (?)",
			m.DB.Table("labels").
				Select("1").
				Where("labels.uri = livestreams.uri").
				Where("labels.val = ?", "!hide").
				Where("labels.neg = ?", false).
				Where("(labels.exp IS NULL OR labels.exp > ?)", now),
		).
		// exclude livestreams with !hide label on the user
		Where("NOT EXISTS (?)",
			m.DB.Table("labels").
				Select("1").
				Where("labels.uri = livestreams.repo_did").
				Where("labels.val = ?", "!hide").
				Where("labels.neg = ?", false).
				Where("(labels.exp IS NULL OR labels.exp > ?)", now),
		)

	if before != nil {
		mainQuery = mainQuery.Where("livestreams.created_at < ?", *before)
	}

	mainQuery = mainQuery.
		Order("livestreams.created_at DESC").
		Limit(limit).
		Preload("Repo")

	err := mainQuery.Find(&recentLivestreams).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("error fetching recent livestreams: %w", err)
	}

	if len(recentLivestreams) == 0 {
		return nil, nil
	}

	return recentLivestreams, nil
}
