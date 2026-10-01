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
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spid"
)

// CaptionTranscript is the indexed view of a place.stream.caption.transcript
// record: one chunk of the captions of a livestream or video. The columns are
// what captions are looked up and grouped by; the words themselves stay in the
// CBOR Record blob and are reached via ToRecord.
type CaptionTranscript struct {
	URI        string     `gorm:"primaryKey;column:uri"`
	CID        string     `gorm:"column:cid"`
	RepoDID    string     `gorm:"column:repo_did;index:idx_caption_transcripts_repo_subject,priority:1"`
	SubjectURI string     `gorm:"column:subject_uri;index:idx_caption_transcripts_subject,priority:1;index:idx_caption_transcripts_repo_subject,priority:2"`
	SubjectCID string     `gorm:"column:subject_cid"`
	Language   string     `gorm:"column:language"`
	Kind       string     `gorm:"column:kind"`
	Source     string     `gorm:"column:source"`
	StartMs    int64      `gorm:"column:start_ms;index:idx_caption_transcripts_subject,priority:2"`
	MediaStart *time.Time `gorm:"column:media_start"`
	Record     []byte     `gorm:"column:record"`
	IndexedAt  time.Time  `gorm:"column:indexed_at"`
}

// ToRecord decodes the stored CBOR into the typed lexicon struct.
func (t *CaptionTranscript) ToRecord() (placestream.CaptionTranscript, error) {
	var rec placestream.CaptionTranscript
	if err := glex.DecodeCBOR(t.Record, &rec); err != nil {
		return placestream.CaptionTranscript{}, fmt.Errorf("decode caption transcript record: %w", err)
	}
	return rec, nil
}

// CaptionKindOrDefault is the kind of a transcript record, which is captions
// when the record doesn't say.
func CaptionKindOrDefault(kind *string) string {
	if kind == nil || *kind == "" {
		return "captions"
	}
	return *kind
}

func (m *DBModel) UpsertCaptionTranscript(ctx context.Context, rec placestream.CaptionTranscript, aturi syntax.ATURI) error {
	repoDID, err := aturi.Authority().AsDID()
	if err != nil {
		return fmt.Errorf("invalid ATURI authority: %w", err)
	}
	cid, err := spid.GetCID(&rec)
	if err != nil {
		return fmt.Errorf("get caption transcript CID: %w", err)
	}
	var buf bytes.Buffer
	if err := rec.MarshalCBOR(&buf); err != nil {
		return fmt.Errorf("marshal caption transcript record: %w", err)
	}
	row := &CaptionTranscript{
		URI:        aturi.String(),
		CID:        cid.String(),
		RepoDID:    repoDID.String(),
		SubjectURI: rec.Subject.Uri,
		SubjectCID: rec.Subject.Cid,
		Language:   rec.Language,
		Kind:       CaptionKindOrDefault(rec.Kind),
		Source:     rec.Source,
		StartMs:    rec.StartMs,
		Record:     buf.Bytes(),
		IndexedAt:  aqtime.FromTime(time.Now().UTC()).Time().UTC(),
	}
	if rec.MediaStart != nil {
		ts, err := syntax.ParseDatetime(*rec.MediaStart)
		if err != nil {
			return fmt.Errorf("invalid caption transcript mediaStart: %w", err)
		}
		t := ts.Time().UTC()
		row.MediaStart = &t
	}
	return m.DB.WithContext(ctx).Save(row).Error
}

func (m *DBModel) DeleteCaptionTranscript(ctx context.Context, uri string) error {
	return m.DB.WithContext(ctx).Where("uri = ?", uri).Delete(&CaptionTranscript{}).Error
}

func (m *DBModel) GetCaptionTranscriptByURI(ctx context.Context, uri string) (*CaptionTranscript, error) {
	var row CaptionTranscript
	err := m.DB.WithContext(ctx).Where("uri = ?", uri).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get caption transcript by uri: %w", err)
	}
	return &row, nil
}

// GetCaptionTranscriptsBySubject returns every indexed chunk of captions whose
// subject is the given livestream or video, from all repos, in order of start
// within each repo.
func (m *DBModel) GetCaptionTranscriptsBySubject(ctx context.Context, subjectURI string) ([]*CaptionTranscript, error) {
	var rows []*CaptionTranscript
	err := m.DB.WithContext(ctx).
		Where("subject_uri = ?", subjectURI).
		Order("repo_did ASC, start_ms ASC, uri ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list caption transcripts for subject: %w", err)
	}
	return rows, nil
}

// GetCaptionTranscriptsForRepoSubject is GetCaptionTranscriptsBySubject
// restricted to one author, for replacing a person's own captions.
func (m *DBModel) GetCaptionTranscriptsForRepoSubject(ctx context.Context, repoDID, subjectURI string) ([]*CaptionTranscript, error) {
	var rows []*CaptionTranscript
	err := m.DB.WithContext(ctx).
		Where("repo_did = ? AND subject_uri = ?", repoDID, subjectURI).
		Order("start_ms ASC, uri ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list caption transcripts for repo and subject: %w", err)
	}
	return rows, nil
}
