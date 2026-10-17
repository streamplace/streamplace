package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/util"
	glex "github.com/streamplace/glex/runtime"
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
	LivestreamURI string     `gorm:"column:livestream_uri" json:"livestreamURI,omitempty"`
}

func (p *PinnedRecord) ToStreamplacePinnedRecord() (placestream.ChatPinnedRecord, error) {
	rec := placestream.ChatPinnedRecord{
		LexiconTypeID: "place.stream.chat.pinnedRecord",
		PinnedMessage: p.PinnedMessage,
		CreatedAt:     p.CreatedAt.UTC().Format(util.ISO8601),
	}
	if p.ExpiresAt != nil {
		s := p.ExpiresAt.UTC().Format(util.ISO8601)
		rec.ExpiresAt = &s
	}
	if p.LivestreamURI != "" {
		rec.Livestream = &p.LivestreamURI
	}
	return rec, nil
}

func (p *PinnedRecord) ToStreamplacePinnedRecordView() (placestream.ChatDefs_PinnedRecordView, error) {
	pr, err := p.ToStreamplacePinnedRecord()
	if err != nil {
		return placestream.ChatDefs_PinnedRecordView{}, err
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
	return createOrVerify(ctx, m, pin, map[string]any{"uri": pin.Uri})
}

func (m *DBModel) GetPinnedRecord(ctx context.Context, uri string) (*PinnedRecord, error) {
	var pin PinnedRecord
	err := m.DB.Preload("Repo").Where("uri = ?", uri).First(&pin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &pin, nil
}

func (m *DBModel) DeletePinnedRecord(ctx context.Context, uri string) error {
	return m.DB.Where("uri = ?", uri).Delete(&PinnedRecord{}).Error
}

func (m *DBModel) DeleteAllPinnedRecords(ctx context.Context, streamerDID string) error {
	return m.DB.Where("repo_did = ?", streamerDID).Delete(&PinnedRecord{}).Error
}

func (m *DBModel) GetActivePinnedRecord(ctx context.Context, streamerDID string) (*PinnedRecord, error) {
	var pin PinnedRecord
	err := m.DB.Preload("Repo").
		Where("repo_did = ?", streamerDID).
		Order("created_at DESC").
		First(&pin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// The newest pin supersedes older pins, including after it expires.
	if pin.LivestreamURI != "" {
		latest, err := m.GetLatestLivestreamForRepo(streamerDID)
		if err != nil {
			return nil, err
		}
		if latest == nil {
			return nil, nil
		}
		if latest.Livestream == nil {
			return nil, errors.New("livestream record is nil")
		}
		var rec placestream.Livestream
		if err := glex.DecodeCBOR(*latest.Livestream, &rec); err != nil {
			return nil, fmt.Errorf("decoding pinned broadcast: %w", err)
		}
		broadcast := latest.URI
		if rec.Broadcast != nil {
			broadcast = *rec.Broadcast
		}
		if rec.EndedAt != nil || broadcast != pin.LivestreamURI {
			return nil, nil
		}
	} else if pin.ExpiresAt != nil && !pin.ExpiresAt.After(time.Now()) {
		return nil, nil
	}
	return &pin, nil
}
