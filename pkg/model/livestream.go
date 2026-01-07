package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/api/bsky"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"stream.place/streamplace/pkg/streamplace"
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
	// upsert livestream record, actually
	return m.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "uri"}},
		DoUpdates: clause.AssignmentColumns([]string{"cid", "created_at", "livestream", "repo_did", "post_cid", "post_uri"}),
	}).Create(ls).Error
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

// GetLatestLivestreams returns the most recent livestreams, given a limit and a cursor
// Only gets livestreams with a valid segment no less than 30 seconds old
func (m *DBModel) GetLatestLivestreams(limit int, before *time.Time) ([]Livestream, error) {
	var recentLivestreams []Livestream
	thirtySecondsAgo := time.Now().Add(-30 * time.Second)

	// get latest segment for the repo DID
	latestRecentSegmentsSubQuery := m.DB.Table("segments").
		Select("repo_did, MAX(start_time) as latest_segment_start_time").
		Where("(repo_did, start_time) IN (?)",
			m.DB.Table("segments").
				Select("repo_did, MAX(start_time)").
				Group("repo_did")).
		Where("start_time > ?", thirtySecondsAgo.UTC()).
		Group("repo_did")

	rankedLivestreamsSubQuery := m.DB.Table("livestreams").
		Select("livestreams.*, ROW_NUMBER() OVER(PARTITION BY livestreams.repo_did ORDER BY livestreams.created_at DESC) as rn").
		Joins("JOIN repos ON livestreams.repo_did = repos.did")

	mainQuery := m.DB.Table("(?) as ranked_livestreams", rankedLivestreamsSubQuery).
		Joins("JOIN (?) as latest_segments ON ranked_livestreams.repo_did = latest_segments.repo_did", latestRecentSegmentsSubQuery).
		Select("ranked_livestreams.*, latest_segments.latest_segment_start_time").
		Where("ranked_livestreams.rn = 1")

	if before != nil {
		mainQuery = mainQuery.Where("livestreams.created_at < ?", *before)
	}

	mainQuery = mainQuery.Order("ranked_livestreams.created_at DESC").
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
