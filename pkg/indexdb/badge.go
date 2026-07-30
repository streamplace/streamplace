package indexdb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/placestream"
)

// BadgeDef stores an indexed badge definition (image, name, description, type).
type BadgeDef struct {
	URI           string    `gorm:"primaryKey;column:uri"`
	CID           string    `gorm:"column:cid"`
	RepoDID       string    `gorm:"column:repo_did;index"`
	RKey          string    `gorm:"column:rkey"`
	Name          string    `gorm:"column:name"`
	Description   string    `gorm:"column:description"`
	ImageCID      string    `gorm:"column:image_cid"`
	ImageMimeType string    `gorm:"column:image_mime_type"`
	BadgeType     string    `gorm:"column:badge_type;index"`
	Record        []byte    `gorm:"column:record"`
	IndexedAt     time.Time `gorm:"column:indexed_at"`
}

// BadgeIssuance records a badge grant from RepoDID (issuer) to RecipientDID.
// BadgeURI points to the BadgeDef record that describes the badge.
type BadgeIssuance struct {
	URI          string    `gorm:"primaryKey;column:uri"`
	CID          string    `gorm:"column:cid"`
	RepoDID      string    `gorm:"column:repo_did;index"`
	RKey         string    `gorm:"column:rkey"`
	RecipientDID string    `gorm:"column:recipient_did;index"`
	BadgeURI     string    `gorm:"column:badge_uri;index"`
	Record       []byte    `gorm:"column:record"`
	IndexedAt    time.Time `gorm:"column:indexed_at"`
}

// UpsertBadgeDef indexes a place.stream.badge.def record, deriving the
// row (identity, image blob extraction, CBOR) from the record and AT-URI.
func (m *DBModel) UpsertBadgeDef(ctx context.Context, rec placestream.BadgeDef, aturi syntax.ATURI) error {
	repoDID, cid, blob, err := recordParts(aturi, &rec)
	if err != nil {
		return err
	}
	def := &BadgeDef{
		URI:       aturi.String(),
		CID:       cid,
		RepoDID:   repoDID,
		RKey:      aturi.RecordKey().String(),
		Name:      rec.Name,
		BadgeType: rec.BadgeType,
		Record:    blob,
		IndexedAt: time.Now().UTC(),
	}
	if rec.Description != nil {
		def.Description = *rec.Description
	}
	if rec.Image != nil {
		def.ImageCID = rec.Image.Ref.String()
		def.ImageMimeType = rec.Image.MimeType
	}
	return m.DB.WithContext(ctx).Save(def).Error
}

func (m *DBModel) DeleteBadgeDef(ctx context.Context, uri string) error {
	return m.DB.WithContext(ctx).Where("uri = ?", uri).Delete(&BadgeDef{}).Error
}

func (m *DBModel) GetBadgeDefByURI(ctx context.Context, uri string) (*BadgeDef, error) {
	var def BadgeDef
	err := m.DB.WithContext(ctx).Where("uri = ?", uri).First(&def).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get badge def: %w", err)
	}
	return &def, nil
}

// UpsertBadgeIssuance indexes a place.stream.badge.issuance record,
// deriving the row (identity, recipient, badge link, CBOR) from the
// record and AT-URI.
func (m *DBModel) UpsertBadgeIssuance(ctx context.Context, rec placestream.BadgeIssuance, aturi syntax.ATURI) error {
	repoDID, cid, blob, err := recordParts(aturi, &rec)
	if err != nil {
		return err
	}
	return m.DB.WithContext(ctx).Save(&BadgeIssuance{
		URI:          aturi.String(),
		CID:          cid,
		RepoDID:      repoDID,
		RKey:         aturi.RecordKey().String(),
		RecipientDID: rec.Did,
		BadgeURI:     rec.Badge.Uri,
		Record:       blob,
		IndexedAt:    time.Now().UTC(),
	}).Error
}

func (m *DBModel) DeleteBadgeIssuance(ctx context.Context, uri string) error {
	return m.DB.WithContext(ctx).Where("uri = ?", uri).Delete(&BadgeIssuance{}).Error
}

func (m *DBModel) GetBadgeIssuanceByURI(ctx context.Context, uri string) (*BadgeIssuance, error) {
	var issuance BadgeIssuance
	err := m.DB.WithContext(ctx).Where("uri = ?", uri).First(&issuance).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get badge issuance: %w", err)
	}
	return &issuance, nil
}

func (m *DBModel) GetBadgeIssuancesForRecipient(ctx context.Context, recipientDID string) ([]*BadgeIssuance, error) {
	var issuances []*BadgeIssuance
	err := m.DB.WithContext(ctx).Where("recipient_did = ?", recipientDID).Find(&issuances).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get badge issuances: %w", err)
	}
	return issuances, nil
}
