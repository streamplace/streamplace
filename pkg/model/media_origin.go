package model

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	glexrt "github.com/streamplace/glex/runtime"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spid"
)

// MediaOrigin is the indexed view of a place.stream.media.origin
// record: a server's attestation that it holds the blob at the given
// CID. Many origin rows can point at the same Blob (one per server);
// the playback path queries by (Blob, ServerDID) to assemble the
// candidate set of nodes to fetch from. Size/MimeType and any other
// blob metadata stay in the CBOR Record blob.
type MediaOrigin struct {
	URI       string    `gorm:"primaryKey;column:uri"`
	CID       string    `gorm:"column:cid"`
	ServerDID string    `gorm:"column:server_did;index:idx_origins_blob_server,priority:2"`
	Blob      string    `gorm:"column:blob;index:idx_origins_blob_server,priority:1"`
	Record    []byte    `gorm:"column:record"`
	IndexedAt time.Time `gorm:"column:indexed_at"`
}

// ToRecord decodes the stored CBOR into the typed lexicon struct.
func (o *MediaOrigin) ToRecord() (placestream.MediaOrigin, error) {
	rec, err := glexrt.CborDecodeValue(o.Record)
	if err != nil {
		return placestream.MediaOrigin{}, fmt.Errorf("decode media origin record: %w", err)
	}
	origin, ok := rec.(placestream.MediaOrigin)
	if !ok {
		return placestream.MediaOrigin{}, fmt.Errorf("media origin record decoded as %T, expected placestream.MediaOrigin", rec)
	}
	return origin, nil
}

func (m *DBModel) UpsertMediaOrigin(ctx context.Context, rec placestream.MediaOrigin, aturi syntax.ATURI) error {
	serverDID, err := aturi.Authority().AsDID()
	if err != nil {
		return fmt.Errorf("invalid ATURI authority: %w", err)
	}
	cid, err := spid.GetCID(&rec)
	if err != nil {
		return fmt.Errorf("get media origin CID: %w", err)
	}
	var buf bytes.Buffer
	if err := rec.MarshalCBOR(&buf); err != nil {
		return fmt.Errorf("marshal media origin record: %w", err)
	}
	o := &MediaOrigin{
		URI:       aturi.String(),
		CID:       cid.String(),
		ServerDID: serverDID.String(),
		Blob:      rec.Blob,
		Record:    buf.Bytes(),
		IndexedAt: aqtime.FromTime(time.Now().UTC()).Time().UTC(),
	}
	return m.DB.WithContext(ctx).Save(o).Error
}

func (m *DBModel) DeleteMediaOrigin(ctx context.Context, uri string) error {
	return m.DB.WithContext(ctx).Where("uri = ?", uri).Delete(&MediaOrigin{}).Error
}

func (m *DBModel) GetMediaOriginByURI(ctx context.Context, uri string) (placestream.MediaOrigin, error) {
	var o MediaOrigin
	err := m.DB.WithContext(ctx).Where("uri = ?", uri).First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return placestream.MediaOrigin{}, nil
	}
	if err != nil {
		return placestream.MediaOrigin{}, fmt.Errorf("get media origin by uri: %w", err)
	}
	return o.ToRecord()
}

// GetMediaOriginsByBlob returns every server attestation for the
// given blob CID, newest first. Returns model rows so the caller
// has ServerDID for source selection without a CBOR decode per row.
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
