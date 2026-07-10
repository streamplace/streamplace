package model

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"stream.place/streamplace/pkg/appbsky"
	"github.com/bluesky-social/indigo/atproto/syntax"
	glexrt "github.com/streamplace/glex/runtime"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/spid"
	"stream.place/streamplace/pkg/placestream"
)

// Video is the indexed view of a place.stream.video record. Every
// non-identity field — title, duration, source, etc — lives inside
// the CBOR Record blob; callers decode it via ToRecord (or rely on
// the getters that return placestream.Video directly). Only
// indexed fields and the URI/CID identity earn their own column.
type Video struct {
	URI       string    `gorm:"primaryKey;column:uri"`
	CID       string    `gorm:"column:cid"`
	RepoDID   string    `gorm:"column:repo_did;index:idx_videos_repo_indexed,priority:1"`
	Record    []byte    `gorm:"column:record"`
	IndexedAt time.Time `gorm:"column:indexed_at;index:idx_videos_repo_indexed,priority:2"`
}

// ToRecord decodes the stored CBOR into the typed lexicon struct.
func (v *Video) ToRecord() (placestream.Video, error) {
	rec, err := glexrt.CborDecodeValue(v.Record)
	if err != nil {
		return placestream.Video{}, fmt.Errorf("decode video record: %w", err)
	}
	video, ok := rec.(placestream.Video)
	if !ok {
		return placestream.Video{}, fmt.Errorf("video record decoded as %T, expected placestream.Video", rec)
	}
	return video, nil
}

func (m *DBModel) UpsertVideo(ctx context.Context, rec placestream.Video, aturi syntax.ATURI) error {
	repoDID, err := aturi.Authority().AsDID()
	if err != nil {
		return fmt.Errorf("invalid ATURI authority: %w", err)
	}
	cid, err := spid.GetCID(&rec)
	if err != nil {
		return fmt.Errorf("get video CID: %w", err)
	}
	var buf bytes.Buffer
	if err := rec.MarshalCBOR(&buf); err != nil {
		return fmt.Errorf("marshal video record: %w", err)
	}
	v := &Video{
		URI:       aturi.String(),
		CID:       cid.String(),
		RepoDID:   repoDID.String(),
		Record:    buf.Bytes(),
		IndexedAt: aqtime.FromTime(time.Now().UTC()).Time().UTC(),
	}
	return m.DB.WithContext(ctx).Save(v).Error
}

func (m *DBModel) DeleteVideo(ctx context.Context, uri string) error {
	return m.DB.WithContext(ctx).Where("uri = ?", uri).Delete(&Video{}).Error
}

func (m *DBModel) GetVideoByURI(ctx context.Context, uri string) (placestream.Video, error) {
	var v Video
	err := m.DB.WithContext(ctx).Where("uri = ?", uri).First(&v).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return placestream.Video{}, nil
	}
	if err != nil {
		return placestream.Video{}, fmt.Errorf("get video by uri: %w", err)
	}
	return v.ToRecord()
}

// GetVideoView is the hydrated read path for place.stream.media.getVideo:
// the video record, its author (DID + handle), and a summary of every
// indexed place.stream.media.viewCount record for the video. Returns
// (nil, nil) when no video matches the URI so callers can surface a
// 404 without inspecting any model types.
func (m *DBModel) GetVideoView(ctx context.Context, uri string) (placestream.MediaGetVideo_VideoView, error) {
	var row Video
	err := m.DB.WithContext(ctx).Where("uri = ?", uri).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return placestream.MediaGetVideo_VideoView{}, nil
	}
	if err != nil {
		return placestream.MediaGetVideo_VideoView{}, fmt.Errorf("get video by uri: %w", err)
	}
	rec, err := row.ToRecord()
	if err != nil {
		return placestream.MediaGetVideo_VideoView{}, err
	}

	author := appbsky.ActorDefs_ProfileViewBasic{Did: row.RepoDID}
	repo, err := m.GetRepo(row.RepoDID)
	if err != nil {
		return placestream.MediaGetVideo_VideoView{}, fmt.Errorf("hydrate author repo: %w", err)
	}
	if repo != nil {
		author.Handle = repo.Handle
	}

	summary, err := m.viewCountSummary(ctx, uri)
	if err != nil {
		return placestream.MediaGetVideo_VideoView{}, err
	}

	likeCount, err := m.GetLikeCount(ctx, uri)
	if err != nil {
		return placestream.MediaGetVideo_VideoView{}, fmt.Errorf("get like count: %w", err)
	}

	tracks := []placestream.MediaTrack_TrackView{}
	if rec.Source.MediaDefs_SourceTracks != nil {
		for _, track := range rec.Source.MediaDefs_SourceTracks.Tracks {
			t, err := m.GetMediaTrackByURI(ctx, track.Uri)
			if err != nil {
				return placestream.MediaGetVideo_VideoView{}, fmt.Errorf("get media track by uri: %w", err)
			}
			if false {
				continue
			}
			cid, err := spid.GetCID(&t)
			if err != nil {
				return placestream.MediaGetVideo_VideoView{}, fmt.Errorf("get cid: %w", err)
			}
			tracks = append(tracks, placestream.MediaTrack_TrackView{
				Record: &glexrt.LexiconTypeDecoder{Val: t},
				Uri:    track.Uri,
				Cid:    cid.String(),
				Author: author,
			})
		}
	}

	return placestream.MediaGetVideo_VideoView{
		Uri:        row.URI,
		Cid:        row.CID,
		Author:     author,
		Record:     &glexrt.LexiconTypeDecoder{Val: rec},
		ViewCounts: summary,
		LikeCount:  likeCount,
		Tracks:     tracks,
	}, nil
}

// GetLatestVideosForRepo returns the most recent N video rows by a
// given repo. Model rows (not decoded records) so the caller has
// URI/CID identity for each entry; call ToRecord on a row to decode.
func (m *DBModel) GetLatestVideosForRepo(ctx context.Context, repoDID string, limit int) ([]*Video, error) {
	if limit <= 0 {
		limit = 25
	}
	var out []*Video
	err := m.DB.WithContext(ctx).
		Where("repo_did = ?", repoDID).
		Order("indexed_at DESC").
		Limit(limit).
		Find(&out).Error
	if err != nil {
		return nil, fmt.Errorf("list videos for repo: %w", err)
	}
	return out, nil
}
