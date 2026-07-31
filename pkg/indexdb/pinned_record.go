package indexdb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
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

// UpsertPinnedRecord indexes a place.stream.chat.pinnedRecord record,
// deriving the row (CID, timestamps, expiry) from the record and AT-URI.
// pinnedBy defaults to the record's own repo when unset, matching the
// pre-refactor indexer.
func (m *DBModel) UpsertPinnedRecord(ctx context.Context, aturi syntax.ATURI, rec placestream.ChatPinnedRecord) error {
	repoDID, cid, _, err := recordParts(aturi, &rec)
	if err != nil {
		return err
	}
	createdAt, err := time.Parse(time.RFC3339, rec.CreatedAt)
	if err != nil {
		return fmt.Errorf("invalid pinnedRecord createdAt %q: %w", rec.CreatedAt, err)
	}
	var expiresAt *time.Time
	if rec.ExpiresAt != nil {
		exp, err := time.Parse(time.RFC3339, *rec.ExpiresAt)
		if err != nil {
			return fmt.Errorf("invalid pinnedRecord expiresAt %q: %w", *rec.ExpiresAt, err)
		}
		expiresAt = &exp
	}
	pinnedBy := repoDID
	if rec.PinnedBy != nil {
		pinnedBy = *rec.PinnedBy
	}
	now := time.Now().UTC()
	pin := &PinnedRecord{
		Uri:           aturi.String(),
		RepoDID:       repoDID,
		PinnedMessage: rec.PinnedMessage,
		PinnedBy:      pinnedBy,
		IndexedAt:     &now,
		CID:           cid,
		CreatedAt:     createdAt,
		ExpiresAt:     expiresAt,
	}
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
