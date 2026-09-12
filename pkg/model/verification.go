package model

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// Verification is one app.bsky.graph.verification record: a verifier (the
// repo it lives in) vouching for a subject. Indexed from every repo on the
// firehose, not only tracked ones, because verifiers are few and which of
// them a node trusts is a branding decision (verifierDids) made after the
// fact.
type Verification struct {
	URI         string    `json:"uri"         gorm:"primaryKey;column:uri"`
	CID         string    `json:"cid"         gorm:"column:cid"`
	IssuerDID   string    `json:"issuerDID"   gorm:"column:issuer_did;index:idx_verification_subject_issuer,priority:2"`
	SubjectDID  string    `json:"subjectDID"  gorm:"column:subject_did;index:idx_verification_subject_issuer,priority:1"`
	Handle      string    `json:"handle"      gorm:"column:handle"`
	DisplayName string    `json:"displayName" gorm:"column:display_name"`
	CreatedAt   time.Time `json:"createdAt"   gorm:"column:created_at"`
	IndexedAt   time.Time `json:"indexedAt"   gorm:"column:indexed_at"`
}

func (v *Verification) recordCID() string { return v.CID }
func (v *Verification) recordURI() string { return v.URI }

// CreateVerification indexes one verification record, idempotently.
func (m *DBModel) CreateVerification(ctx context.Context, v *Verification) error {
	if v.IndexedAt.IsZero() {
		v.IndexedAt = time.Now()
	}
	return createOrVerify(ctx, m, v, map[string]any{"uri": v.URI})
}

// DeleteVerification removes a verification by record URI.
func (m *DBModel) DeleteVerification(ctx context.Context, uri string) error {
	return m.DB.WithContext(ctx).Where("uri = ?", uri).Delete(&Verification{}).Error
}

// DeleteVerificationsByIssuer removes every verification issued by issuer
// (a mirrored labeler's rows, before a rescan under new label rules).
func (m *DBModel) DeleteVerificationsByIssuer(ctx context.Context, issuer string) error {
	return m.DB.WithContext(ctx).Where("issuer_did = ?", issuer).Delete(&Verification{}).Error
}

// VerificationsFor returns the verifications of the given subjects issued by
// any of the given issuers, keyed by subject DID.
func (m *DBModel) VerificationsFor(ctx context.Context, subjectDIDs []string, issuerDIDs []string) (map[string][]Verification, error) {
	out := map[string][]Verification{}
	if len(subjectDIDs) == 0 || len(issuerDIDs) == 0 {
		return out, nil
	}
	var rows []Verification
	err := m.DB.WithContext(ctx).
		Where("subject_did IN ? AND issuer_did IN ?", subjectDIDs, issuerDIDs).
		Order("created_at ASC").
		Find(&rows).Error
	if err != nil && !isNotFound(err) {
		return nil, err
	}
	for _, r := range rows {
		out[r.SubjectDID] = append(out[r.SubjectDID], r)
	}
	return out, nil
}

func isNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}
