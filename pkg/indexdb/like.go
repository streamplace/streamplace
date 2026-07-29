package indexdb

import (
	"context"
	"errors"
	"fmt"
	"time"

	glex "github.com/streamplace/glex/runtime"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/placestream"
)

type Like struct {
	// URI is the primary key: it's unique per like record (repo + rkey). CID
	// is NOT — two users liking the same subject at the same createdAt produce
	// byte-identical records and thus the same CID, which would collide.
	URI       string     `json:"uri"            gorm:"primaryKey;column:uri"`
	CID       string     `json:"cid"            gorm:"column:cid"`
	Subject   string     `json:"subject"        gorm:"column:subject;index:idx_likes_subject"`
	RepoDID   string     `json:"repoDID"        gorm:"column:repo_did"`
	Repo      *Repo      `json:"repo,omitempty" gorm:"foreignKey:DID;references:RepoDID"`
	IndexedAt *time.Time `json:"indexedAt"      gorm:"column:indexed_at"`
	CreatedAt time.Time  `json:"createdAt"      gorm:"column:created_at"`
}

func (l *Like) ToLikeView() (placestream.GetLikes_LikeView, error) {
	// The likes table doesn't store record bytes; synthesize the record from
	// the indexed columns.
	rec := &placestream.Like{Subject: l.Subject, CreatedAt: l.CreatedAt.Format(time.RFC3339)}
	return placestream.GetLikes_LikeView{
		Uri:    l.URI,
		Cid:    l.CID,
		Record: &glex.LexiconTypeDecoder{Val: rec},
		Author: appbsky.ActorDefs_ProfileViewBasic{
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

func (m *DBModel) CreateLike(ctx context.Context, like *Like) error {
	return m.DB.Create(like).Error
}

func (m *DBModel) DeleteLike(ctx context.Context, uri string) error {
	tx := m.DB.Where("uri = ?", uri).Delete(&Like{})
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (m *DBModel) GetLike(uri string) (*Like, error) {
	var like Like
	err := m.DB.Preload("Repo").Where("uri = ?", uri).First(&like).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &like, nil
}

func (m *DBModel) GetLikeBySubjectAndUser(ctx context.Context, subject string, repoDID string) (*Like, error) {
	var like Like
	err := m.DB.WithContext(ctx).Preload("Repo").Where("subject = ? AND repo_did = ?", subject, repoDID).First(&like).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &like, nil
}

func (m *DBModel) GetLikesForSubject(ctx context.Context, subject string, limit int, cursor *time.Time) ([]placestream.GetLikes_LikeView, int64, *time.Time, error) {
	count, err := m.GetLikeCount(ctx, subject)
	if err != nil {
		return nil, 0, nil, err
	}
	dblikes := []Like{}
	query := m.DB.
		Preload("Repo").
		Where("subject = ?", subject)
	if cursor != nil {
		query = query.Where("likes.created_at < ?", *cursor)
	}
	err = query.
		Limit(limit + 1).
		Order("likes.created_at DESC").
		Find(&dblikes).Error
	if err != nil {
		return nil, 0, nil, fmt.Errorf("error retrieving likes: %w", err)
	}
	var nextCursor *time.Time
	if len(dblikes) > limit {
		nextCursor = &dblikes[limit-1].CreatedAt
		dblikes = dblikes[:limit]
	}
	views := []placestream.GetLikes_LikeView{}
	for _, l := range dblikes {
		view, err := l.ToLikeView()
		if err != nil {
			return nil, 0, nil, err
		}
		views = append(views, view)
	}
	return views, count, nextCursor, nil
}

func (m *DBModel) GetLikeCount(ctx context.Context, subject string) (int64, error) {
	var count int64
	err := m.DB.Model(&Like{}).Where("subject = ?", subject).Count(&count).Error
	return count, err
}
