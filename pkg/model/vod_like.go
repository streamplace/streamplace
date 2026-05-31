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

type VodLike struct {
	CID       string     `json:"cid"            gorm:"primaryKey;column:cid"`
	URI       string     `json:"uri"            gorm:"column:uri"`
	Subject   string     `json:"subject"        gorm:"column:subject;index:idx_vod_likes_subject"`
	RepoDID   string     `json:"repoDID"        gorm:"column:repo_did"`
	Repo      *Repo      `json:"repo,omitempty" gorm:"foreignKey:DID;references:RepoDID"`
	IndexedAt *time.Time `json:"indexedAt"      gorm:"column:indexed_at"`
	CreatedAt time.Time  `json:"createdAt"      gorm:"column:created_at"`
}

func (l *VodLike) ToStreamplaceLikeView() (*streamplace.VodGetLikes_LikeView, error) {
	rec, err := lexutil.CborDecodeValue([]byte{})
	if err != nil {
		rec = &streamplace.Like{Subject: l.Subject, CreatedAt: l.CreatedAt.Format(time.RFC3339)}
	}
	return &streamplace.VodGetLikes_LikeView{
		Uri:    l.URI,
		Cid:    l.CID,
		Record: &lexutil.LexiconTypeDecoder{Val: rec},
		Author: &bsky.ActorDefs_ProfileViewBasic{
			Did: l.RepoDID,
			Handle: func() string {
				if l.Repo != nil {
					return l.Repo.Handle
				}
				return ""
			}(),
		},
		IndexedAt: l.IndexedAt.UTC().Format(time.RFC3339Nano),
	}, nil
}

func (m *DBModel) CreateVodLike(ctx context.Context, like *VodLike) error {
	return m.DB.Create(like).Error
}

func (m *DBModel) DeleteVodLike(ctx context.Context, uri string) error {
	tx := m.DB.Where("uri = ?", uri).Delete(&VodLike{})
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (m *DBModel) GetVodLike(uri string) (*VodLike, error) {
	var like VodLike
	err := m.DB.Preload("Repo").Where("uri = ?", uri).First(&like).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &like, nil
}

func (m *DBModel) GetVodLikeBySubjectAndUser(ctx context.Context, subject string, repoDID string) (*VodLike, error) {
	var like VodLike
	err := m.DB.Preload("Repo").Where("subject = ? AND repo_did = ?", subject, repoDID).First(&like).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &like, nil
}

func (m *DBModel) GetLikesForSubject(ctx context.Context, subject string, limit int, cursor *time.Time) ([]*streamplace.VodGetLikes_LikeView, int64, *time.Time, error) {
	count, err := m.GetLikeCount(ctx, subject)
	if err != nil {
		return nil, 0, nil, err
	}
	dblikes := []VodLike{}
	query := m.DB.
		Preload("Repo").
		Where("subject = ?", subject)
	if cursor != nil {
		query = query.Where("vod_likes.created_at < ?", *cursor)
	}
	err = query.
		Limit(limit + 1).
		Order("vod_likes.created_at DESC").
		Find(&dblikes).Error
	if err != nil {
		return nil, 0, nil, fmt.Errorf("error retrieving likes: %w", err)
	}
	var nextCursor *time.Time
	if len(dblikes) > limit {
		nextCursor = &dblikes[limit-1].CreatedAt
		dblikes = dblikes[:limit]
	}
	views := []*streamplace.VodGetLikes_LikeView{}
	for _, l := range dblikes {
		view, err := l.ToStreamplaceLikeView()
		if err != nil {
			return nil, 0, nil, err
		}
		views = append(views, view)
	}
	return views, count, nextCursor, nil
}

func (m *DBModel) GetLikeCount(ctx context.Context, subject string) (int64, error) {
	var count int64
	err := m.DB.Model(&VodLike{}).Where("subject = ?", subject).Count(&count).Error
	return count, err
}
