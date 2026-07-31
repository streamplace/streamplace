package model

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	glex "github.com/streamplace/glex/runtime"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/constants"
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
	var origin placestream.MediaOrigin
	if err := glex.DecodeCBOR(o.Record, &origin); err != nil {
		return placestream.MediaOrigin{}, fmt.Errorf("decode media origin record: %w", err)
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

// UpsertOwnMediaOrigin indexes this node's own attestation that it holds a
// blob, building the record and AT-URI the way the publisher does: rkey is the
// blob CID by convention, and the authority is our ServerDID — the same
// (server_did, blob) key GetVideoList filters on.
//
// Publishing the record to the server repo and indexing it locally are separate
// steps, normally bridged by the record federating back to us over the firehose.
// That round-trip is a single point of failure, and when it drops an event the
// video plays fine by direct link but never appears in any listing, with nothing
// to reconcile it afterward. Callers that publish an origin should index it here
// too; both paths are idempotent, so whichever lands second is a no-op.
func (m *DBModel) UpsertOwnMediaOrigin(ctx context.Context, serverDID, blobCID string, size int64, mimeType string) error {
	aturi, err := syntax.ParseATURI(fmt.Sprintf(
		"at://%s/%s/%s", serverDID, constants.PLACE_STREAM_MEDIA_ORIGIN, blobCID,
	))
	if err != nil {
		return fmt.Errorf("build media origin uri: %w", err)
	}
	return m.UpsertMediaOrigin(ctx, placestream.MediaOrigin{
		LexiconTypeID: constants.PLACE_STREAM_MEDIA_ORIGIN,
		Blob:          blobCID,
		Size:          size,
		MimeType:      mimeType,
	}, aturi)
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
