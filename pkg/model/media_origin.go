package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// MediaOrigin is the indexed view of a place.stream.media.origin
// record. Each row is an attestation by ServerDID (a streamplace node's
// DID) that it has the blob at the given CID and can serve it via
// XRPC. Many origin rows can point at the same Blob; the playback
// path uses them as a candidate set of nodes to fetch from.
//
// The rkey of the underlying record is conventionally the blob CID
// itself (origin records use key=any). We also store the rkey
// explicitly so the (server, blob) tuple has a stable foreign key
// shape even if that convention changes.
type MediaOrigin struct {
	URI       string    `gorm:"primaryKey;column:uri"`
	CID       string    `gorm:"column:cid"`
	ServerDID string    `gorm:"column:server_did;index:idx_origins_blob_server,priority:2"`
	RKey      string    `gorm:"column:rkey"`
	Blob      string    `gorm:"column:blob;index:idx_origins_blob_server,priority:1"`
	Size      int64     `gorm:"column:size"`
	MimeType  string    `gorm:"column:mime_type"`
	Record    []byte    `gorm:"column:record"`
	IndexedAt time.Time `gorm:"column:indexed_at"`
}

func (m *DBModel) UpsertMediaOrigin(ctx context.Context, o *MediaOrigin) error {
	return m.DB.WithContext(ctx).Save(o).Error
}

func (m *DBModel) DeleteMediaOrigin(ctx context.Context, uri string) error {
	return m.DB.WithContext(ctx).Where("uri = ?", uri).Delete(&MediaOrigin{}).Error
}

func (m *DBModel) GetMediaOriginByURI(ctx context.Context, uri string) (*MediaOrigin, error) {
	var o MediaOrigin
	err := m.DB.WithContext(ctx).Where("uri = ?", uri).First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get media origin by uri: %w", err)
	}
	return &o, nil
}

// GetMediaOriginsByBlob returns every server that has attested to
// hosting the given blob CID. Used by playback to discover candidate
// download sources. Returned in IndexedAt order (newest first) on
// the theory that recent attestations are more reliable.
func (m *DBModel) GetMediaOriginsByBlob(ctx context.Context, blob string) ([]*MediaOrigin, error) {
	var out []*MediaOrigin
	err := m.DB.WithContext(ctx).
		Where("blob = ?", blob).
		Order("indexed_at DESC").
		Find(&out).Error
	if err != nil {
		return nil, fmt.Errorf("list origins for blob: %w", err)
	}
	return out, nil
}
