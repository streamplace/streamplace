package model

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spid"
)

// ChatAccessRule is one place.stream.chat.access record: a streamer's rule
// about who may chat on their streams. Indexed from tracked repos; the
// subject decides which verifiers and labelers this node goes on to index.
type ChatAccessRule struct {
	URI         string `json:"uri"         gorm:"primaryKey;column:uri"`
	CID         string `json:"cid"         gorm:"column:cid"`
	RepoDID     string `json:"repoDID"     gorm:"column:repo_did;index:idx_chat_access_repo"`
	Action      string `json:"action"      gorm:"column:action"`
	SubjectType string `json:"subjectType" gorm:"column:subject_type"`
	// SubjectDID is the verifier for a verifier subject, the labeler for a
	// label subject.
	SubjectDID string    `json:"subjectDID"  gorm:"column:subject_did;index:idx_chat_access_subject"`
	LabelValue string    `json:"labelValue"  gorm:"column:label_value"`
	Record     []byte    `json:"-"           gorm:"column:record"`
	CreatedAt  time.Time `json:"createdAt"   gorm:"column:created_at"`
	IndexedAt  time.Time `json:"indexedAt"   gorm:"column:indexed_at"`
}

const (
	ChatAccessAllow = "allow"
	ChatAccessDeny  = "deny"

	ChatAccessSubjectVerifier = "verifier"
	ChatAccessSubjectLabel    = "label"
)

func (r *ChatAccessRule) recordCID() string { return r.CID }
func (r *ChatAccessRule) recordURI() string { return r.URI }

// ErrChatAccessSubjectUnknown marks a rule whose subject this node does not
// understand; the record is skipped, not failed, so newer subject types
// never break indexing.
var ErrChatAccessSubjectUnknown = fmt.Errorf("chat access rule has a subject this node does not understand")

// ChatAccessRuleFromRecord turns a record into its row, or
// ErrChatAccessSubjectUnknown.
func ChatAccessRuleFromRecord(rec *placestream.ChatAccess, aturi syntax.ATURI) (*ChatAccessRule, error) {
	repoDID, err := aturi.Authority().AsDID()
	if err != nil {
		return nil, fmt.Errorf("invalid ATURI authority: %w", err)
	}
	action := strings.TrimSpace(rec.Action)
	if action != ChatAccessAllow && action != ChatAccessDeny {
		return nil, fmt.Errorf("chat access rule has action %q", rec.Action)
	}
	row := &ChatAccessRule{
		URI:     aturi.String(),
		RepoDID: repoDID.String(),
		Action:  action,
	}
	switch {
	case rec.Subject.ChatAccess_Verifier != nil:
		did := strings.TrimSpace(rec.Subject.ChatAccess_Verifier.Did)
		if !strings.HasPrefix(did, "did:") {
			return nil, fmt.Errorf("chat access verifier %q is not a DID", did)
		}
		row.SubjectType = ChatAccessSubjectVerifier
		row.SubjectDID = did
	case rec.Subject.ChatAccess_Label != nil:
		labeler := strings.TrimSpace(rec.Subject.ChatAccess_Label.Labeler)
		value := strings.TrimSpace(rec.Subject.ChatAccess_Label.Value)
		if !strings.HasPrefix(labeler, "did:") || value == "" {
			return nil, fmt.Errorf("chat access label rule needs a labeler DID and a value")
		}
		row.SubjectType = ChatAccessSubjectLabel
		row.SubjectDID = labeler
		row.LabelValue = value
	default:
		return nil, ErrChatAccessSubjectUnknown
	}
	cid, err := spid.GetCID(rec)
	if err != nil {
		return nil, fmt.Errorf("failed to get CID: %w", err)
	}
	row.CID = cid.String()
	buf := bytes.Buffer{}
	if err := rec.MarshalCBOR(&buf); err != nil {
		return nil, fmt.Errorf("failed to marshal chat access rule: %w", err)
	}
	row.Record = buf.Bytes()
	if created, err := time.Parse(time.RFC3339, rec.CreatedAt); err == nil {
		row.CreatedAt = created.UTC()
	} else {
		row.CreatedAt = time.Now().UTC()
	}
	row.IndexedAt = time.Now().UTC()
	return row, nil
}

// CreateChatAccessRule indexes one rule, idempotently.
func (m *DBModel) CreateChatAccessRule(ctx context.Context, row *ChatAccessRule) error {
	return createOrVerify(ctx, m, row, map[string]any{"uri": row.URI})
}

// DeleteChatAccessRule removes a rule by record URI.
func (m *DBModel) DeleteChatAccessRule(ctx context.Context, uri string) error {
	return m.DB.WithContext(ctx).Where("uri = ?", uri).Delete(&ChatAccessRule{}).Error
}

// ListChatAccessRules returns a streamer's rules, oldest first.
func (m *DBModel) ListChatAccessRules(ctx context.Context, repoDID string) ([]ChatAccessRule, error) {
	var rows []ChatAccessRule
	err := m.DB.WithContext(ctx).Where("repo_did = ?", repoDID).Order("created_at ASC").Find(&rows).Error
	if err != nil && !isNotFound(err) {
		return nil, err
	}
	return rows, nil
}

// ChatAccessSubjects lists every distinct subject any indexed rule names:
// the verifiers to index and, per labeler, the label values to mirror.
func (m *DBModel) ChatAccessSubjects(ctx context.Context) (verifiers []string, labels map[string][]string, err error) {
	var rows []ChatAccessRule
	err = m.DB.WithContext(ctx).Select("subject_type", "subject_did", "label_value").Find(&rows).Error
	if err != nil && !isNotFound(err) {
		return nil, nil, err
	}
	labels = map[string][]string{}
	seenV := map[string]bool{}
	seenL := map[string]bool{}
	for _, r := range rows {
		switch r.SubjectType {
		case ChatAccessSubjectVerifier:
			if !seenV[r.SubjectDID] {
				seenV[r.SubjectDID] = true
				verifiers = append(verifiers, r.SubjectDID)
			}
		case ChatAccessSubjectLabel:
			key := r.SubjectDID + "\x00" + r.LabelValue
			if !seenL[key] {
				seenL[key] = true
				labels[r.SubjectDID] = append(labels[r.SubjectDID], r.LabelValue)
			}
		}
	}
	return verifiers, labels, nil
}
