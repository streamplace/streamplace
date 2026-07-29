package indexdb

import (
	"context"
	"errors"
	"time"

	"github.com/bluesky-social/indigo/util"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/placestream"
)

type PinnedRecord struct {
	Uri           string     `gorm:"primaryKey;column:uri"`
	CID           string     `gorm:"column:cid"`
	RepoDID       string     `json:"repoDID"              gorm:"column:repo_did"`
	Repo          *Repo      `json:"repo,omitempty"       gorm:"foreignKey:DID;references:RepoDID"`
	PinnedMessage string     `gorm:"column:pinned_message" json:"pinnedMessage"`
	PinnedBy      string     `gorm:"column:pinned_by"      json:"pinnedBy"`
	IndexedAt     *time.Time `gorm:"column:indexed_at"    json:"indexedAt"`
	ExpiresAt     *time.Time `gorm:"column:expires_at"    json:"expiresAt"`
	CreatedAt     time.Time  `gorm:"column:created_at"    json:"createdAt"`
}

func (p *PinnedRecord) ToRecord() (placestream.ChatPinnedRecord, error) {
	rec := placestream.ChatPinnedRecord{
		LexiconTypeID: "place.stream.chat.pinnedRecord",
		PinnedMessage: p.PinnedMessage,
		CreatedAt:     p.CreatedAt.UTC().Format(util.ISO8601),
	}
	if p.ExpiresAt != nil {
		s := p.ExpiresAt.UTC().Format(util.ISO8601)
		rec.ExpiresAt = &s
	}
	return rec, nil
}

func (p *PinnedRecord) ToPinnedRecordView() (placestream.ChatDefs_PinnedRecordView, error) {
	pr := placestream.ChatPinnedRecord{
		LexiconTypeID: "place.stream.chat.pinnedRecord",
		PinnedMessage: p.PinnedMessage,
		CreatedAt:     p.CreatedAt.UTC().Format(util.ISO8601),
	}
	if p.ExpiresAt != nil {
		s := p.ExpiresAt.UTC().Format(util.ISO8601)
		pr.ExpiresAt = &s
	}
	rec := placestream.ChatDefs_PinnedRecordView{
		LexiconTypeID: "place.stream.chat.defs#pinnedRecordView",
		Record:        pr,
		Cid:           p.CID,
		IndexedAt:     p.CreatedAt.UTC().Format(time.RFC3339Nano),
		// message, pinnedby not included, will fill in later
		Uri: p.Uri,
	}
	return rec, nil
}
func (m *DBModel) CreatePinnedRecord(ctx context.Context, pin *PinnedRecord) error {
	return m.DB.Create(pin).Error
}

func (m *DBModel) GetPinnedRecord(ctx context.Context, uri string) (*placestream.ChatDefs_PinnedRecordView, error) {
	var pin PinnedRecord
	err := m.DB.Preload("Repo").Where("uri = ?", uri).First(&pin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	view, err := pin.ToPinnedRecordView()
	if err != nil {
		return nil, err
	}
	return &view, nil
}

func (m *DBModel) DeletePinnedRecord(ctx context.Context, uri string) error {
	return m.DB.Where("uri = ?", uri).Delete(&PinnedRecord{}).Error
}

func (m *DBModel) DeleteAllPinnedRecords(ctx context.Context, streamerDID string) error {
	return m.DB.Where("repo_did = ?", streamerDID).Delete(&PinnedRecord{}).Error
}

func (m *DBModel) GetActivePinnedRecord(ctx context.Context, streamerDID string) (*placestream.ChatDefs_PinnedRecordView, error) {
	var pin PinnedRecord
	now := time.Now()
	err := m.DB.Preload("Repo").
		Where("repo_did = ? AND (expires_at IS NULL OR expires_at > ?)", streamerDID, now).
		Order("created_at DESC").
		First(&pin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	view, err := pin.ToPinnedRecordView()
	if err != nil {
		return nil, err
	}
	return &view, nil
}
