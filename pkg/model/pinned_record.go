package model

import (
	"context"
	"errors"
	"time"

	glex "github.com/streamplace/glex/runtime"

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
	pr := placestream.ChatPinnedRecord{
		LexiconTypeID: "place.stream.chat.pinnedRecord",
		PinnedMessage: p.PinnedMessage,
		CreatedAt:     p.CreatedAt.UTC().Format(util.ISO8601),
	}
	if p.ExpiresAt != nil {
		s := p.ExpiresAt.UTC().Format(util.ISO8601)
		pr.ExpiresAt = &s
	}
	if p.LivestreamURI != "" {
		pr.Livestream = &p.LivestreamURI
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
	active, err := m.isPinActive(&pin)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, nil
	}
	return &pin, nil
}

// isPinActive checks whether a pin should currently be shown. The most recent
// pin record for a streamer is authoritative: if it is scoped to a livestream
// whose stream has ended, or expires (expiresAt in the past with no
// livestream), the streamer has no active pin — older records are not
// resurrected. A pin with neither livestream nor expiresAt is permanent.
func (m *DBModel) isPinActive(pin *PinnedRecord) (bool, error) {
	// A livestream-scoped pin is active only while the referenced stream
	// exists and has not ended; livestream takes precedence over expiresAt.
	if pin.LivestreamURI != "" {
		ls, err := m.GetLivestream(pin.LivestreamURI)
		if err != nil {
			return false, err
		}
		if ls == nil {
			return false, nil
		}
		if ls.Livestream != nil {
			var rec placestream.Livestream
			if err := glex.DecodeCBOR(*ls.Livestream, &rec); err == nil && rec.EndedAt != nil {
				return false, nil
			}
		}
		return true, nil
	}
	// Timed pins expire; pins with neither livestream nor expiresAt are permanent.
	if pin.ExpiresAt != nil && pin.ExpiresAt.Before(time.Now()) {
		return false, nil
	}
	return true, nil
}
