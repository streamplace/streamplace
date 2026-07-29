package indexdb

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
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spid"
)

// MediaViewCount is the indexed view of a place.stream.media.viewCount
// record: a single reporting node's report of view counts for one
// place.stream.video over a closed time window. Multiple records exist
// per video — one per (reporter, window) tuple — and the query layer
// sums across them to produce per-video totals.
//
// Records live in their reporter's server repo; we index every one
// the firehose hands us. Consumers cross-reference by VideoURI; the
// reporter identity is RepoDID. count / bytes / duration totals stay
// in the CBOR Record blob and get decoded at query time (small
// enough not to be worth denormalizing yet).
type MediaViewCount struct {
	URI       string    `gorm:"primaryKey;column:uri"`
	CID       string    `gorm:"column:cid"`
	RepoDID   string    `gorm:"column:repo_did"`
	VideoURI  string    `gorm:"column:video_uri;index"`
	Record    []byte    `gorm:"column:record"`
	IndexedAt time.Time `gorm:"column:indexed_at"`
}

// ToRecord decodes the stored CBOR into the typed lexicon struct.
func (v *MediaViewCount) ToRecord() (*placestream.MediaViewCount, error) {
	var vc placestream.MediaViewCount
	if err := glex.DecodeCBOR(v.Record, &vc); err != nil {
		return nil, fmt.Errorf("decode view-count record: %w", err)
	}
	return &vc, nil
}

func (m *DBModel) UpsertMediaViewCount(ctx context.Context, rec placestream.MediaViewCount, aturi syntax.ATURI) error {
	repoDID, err := aturi.Authority().AsDID()
	if err != nil {
		return fmt.Errorf("invalid ATURI authority: %w", err)
	}
	cid, err := spid.GetCID(&rec)
	if err != nil {
		return fmt.Errorf("get view-count CID: %w", err)
	}
	var buf bytes.Buffer
	if err := rec.MarshalCBOR(&buf); err != nil {
		return fmt.Errorf("marshal view-count record: %w", err)
	}
	row := &MediaViewCount{
		URI:       aturi.String(),
		CID:       cid.String(),
		RepoDID:   repoDID.String(),
		VideoURI:  rec.Video,
		Record:    buf.Bytes(),
		IndexedAt: aqtime.FromTime(time.Now().UTC()).Time().UTC(),
	}
	return m.DB.WithContext(ctx).Save(row).Error
}

func (m *DBModel) DeleteMediaViewCount(ctx context.Context, uri string) error {
	return m.DB.WithContext(ctx).Where("uri = ?", uri).Delete(&MediaViewCount{}).Error
}

func (m *DBModel) GetMediaViewCountByURI(ctx context.Context, uri string) (*placestream.MediaViewCount, error) {
	var row MediaViewCount
	err := m.DB.WithContext(ctx).Where("uri = ?", uri).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get view count by uri: %w", err)
	}
	return row.ToRecord()
}

// viewCountSummary sums every place.stream.media.viewCount record
// indexed for the given video — across reporters and across windows
// — into the lexicon-defined summary shape. Always returns a non-nil
// summary (with zeroes when no records exist) so the view shape stays
// consistent: consumers can render `count` / `bytes` / `durationMs`
// unconditionally without a nil check. Internal to the model
// package; consumers see only the hydrated VideoView from
// GetVideoView.
func (m *DBModel) viewCountSummary(ctx context.Context, videoURI string) (placestream.MediaGetVideo_ViewCountSummary, error) {
	out := placestream.MediaGetVideo_ViewCountSummary{}
	var rows []*MediaViewCount
	err := m.DB.WithContext(ctx).
		Where("video_uri = ?", videoURI).
		Find(&rows).Error
	if err != nil {
		return placestream.MediaGetVideo_ViewCountSummary{}, fmt.Errorf("list view counts for video: %w", err)
	}
	reporters := make(map[string]struct{})
	for _, row := range rows {
		rec, err := row.ToRecord()
		if err != nil {
			// Skip the corrupt row rather than aborting the summary —
			// one bad record shouldn't zero out a popular video.
			continue
		}
		out.Count += rec.Count
		for _, t := range rec.Tracks {
			out.Bytes += t.Bytes
			out.DurationMs += t.DurationMs
		}
		reporters[row.RepoDID] = struct{}{}
	}
	out.Reporters = int64(len(reporters))
	return out, nil
}
