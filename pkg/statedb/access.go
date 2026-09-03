package statedb

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"stream.place/streamplace/pkg/access"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spid"
)

// AccessGrant is a place.stream.access.grant record held in statedb until
// the atproto spaces implementation ships (see pkg/access). The CBOR record
// body is the migration target; the indexed columns are denormalised from it
// so the checker can load a snapshot with one query.
type AccessGrant struct {
	// URI is the at:// space URI of the record (access.GrantURI).
	URI          string `gorm:"column:uri;primaryKey"`
	AuthorityDID string `gorm:"column:authority_did;index:idx_access_grant_lookup,priority:1"`
	SubjectDID   string `gorm:"column:subject_did;index:idx_access_grant_lookup,priority:2"`
	Role         string `gorm:"column:role;index:idx_access_grant_lookup,priority:3"`
	// AuthorDID is the admin that created the grant (the record's author).
	AuthorDID string `gorm:"column:author_did"`
	// Data is the CBOR-encoded place.stream.access.grant record body.
	Data []byte `gorm:"column:data;type:bytes"`
	// CID is the sha256-of-CBOR CIDv1 of Data.
	CID       string    `gorm:"column:cid"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (AccessGrant) TableName() string {
	return "access_grants"
}

// Record decodes the CBOR body.
func (g *AccessGrant) Record() (*placestream.AccessGrant, error) {
	var rec placestream.AccessGrant
	if err := rec.UnmarshalCBOR(bytes.NewReader(g.Data)); err != nil {
		return nil, fmt.Errorf("unmarshal access grant: %w", err)
	}
	return &rec, nil
}

// ListAccessGrants returns every grant in authority's space, oldest first.
func (state *StatefulDB) ListAccessGrants(ctx context.Context, authority string) ([]AccessGrant, error) {
	var grants []AccessGrant
	err := state.DB.WithContext(ctx).
		Where("authority_did = ?", authority).
		Order("created_at ASC, uri ASC").
		Find(&grants).Error
	if err != nil {
		return nil, fmt.Errorf("list access grants: %w", err)
	}
	return grants, nil
}

// FindAccessGrant returns the grant of role to subject, or nil when none.
func (state *StatefulDB) FindAccessGrant(ctx context.Context, authority, subject, role string) (*AccessGrant, error) {
	var g AccessGrant
	err := state.DB.WithContext(ctx).
		Where("authority_did = ? AND subject_did = ? AND role = ?", authority, subject, role).
		Order("created_at ASC").
		First(&g).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find access grant: %w", err)
	}
	return &g, nil
}

// GetAccessGrant returns the grant at uri, or gorm.ErrRecordNotFound.
func (state *StatefulDB) GetAccessGrant(ctx context.Context, uri string) (*AccessGrant, error) {
	var g AccessGrant
	if err := state.DB.WithContext(ctx).Where("uri = ?", uri).First(&g).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

// CreateAccessGrant writes a new grant record authored by author into
// authority's space. The rkey is a fresh TID.
func (state *StatefulDB) CreateAccessGrant(ctx context.Context, authority, author string, rec *placestream.AccessGrant) (*AccessGrant, error) {
	buf := bytes.NewBuffer(nil)
	if err := rec.MarshalCBOR(buf); err != nil {
		return nil, fmt.Errorf("marshal access grant: %w", err)
	}
	c, err := spid.GetCIDFromBytes(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("compute access grant CID: %w", err)
	}
	createdAt, err := time.Parse(time.RFC3339, rec.CreatedAt)
	if err != nil {
		createdAt = time.Now()
	}
	g := &AccessGrant{
		URI:          access.GrantURI(authority, author, spid.TIDClock.Next().String()),
		AuthorityDID: authority,
		SubjectDID:   rec.Subject,
		Role:         rec.Role,
		AuthorDID:    author,
		Data:         buf.Bytes(),
		CID:          c.String(),
		CreatedAt:    createdAt,
	}
	if err := state.DB.WithContext(ctx).Create(g).Error; err != nil {
		return nil, fmt.Errorf("create access grant: %w", err)
	}
	return g, nil
}

// DeleteAccessGrant removes the grant at uri. Returns gorm.ErrRecordNotFound
// when nothing matched.
func (state *StatefulDB) DeleteAccessGrant(ctx context.Context, uri string) error {
	res := state.DB.WithContext(ctx).Where("uri = ?", uri).Delete(&AccessGrant{})
	if res.Error != nil {
		return fmt.Errorf("delete access grant: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func accessPolicyKey(authority string) string {
	return "access-policy:" + authority
}

// GetAccessPolicy returns authority's place.stream.access.policy record, or
// nil when the node has never written one.
func (state *StatefulDB) GetAccessPolicy(ctx context.Context, authority string) (*placestream.AccessPolicy, error) {
	conf, err := state.GetConfig(accessPolicyKey(authority))
	if err != nil {
		return nil, fmt.Errorf("get access policy: %w", err)
	}
	if conf == nil {
		return nil, nil
	}
	var rec placestream.AccessPolicy
	if err := rec.UnmarshalCBOR(bytes.NewReader(conf.Value)); err != nil {
		return nil, fmt.Errorf("unmarshal access policy: %w", err)
	}
	return &rec, nil
}

// PutAccessPolicy stores authority's policy record.
func (state *StatefulDB) PutAccessPolicy(ctx context.Context, authority string, rec *placestream.AccessPolicy) error {
	buf := bytes.NewBuffer(nil)
	if err := rec.MarshalCBOR(buf); err != nil {
		return fmt.Errorf("marshal access policy: %w", err)
	}
	if err := state.PutConfig(accessPolicyKey(authority), buf.Bytes()); err != nil {
		return fmt.Errorf("put access policy: %w", err)
	}
	return nil
}

// EnsureAccessCookieKey returns the HMAC key that signs viewer cookies,
// generating and storing one on first use. Shared through statedb so every
// node in a station accepts each other's cookies.
func (state *StatefulDB) EnsureAccessCookieKey(ctx context.Context) ([]byte, error) {
	const key = "access-cookie-key"
	conf, err := state.GetConfig(key)
	if err != nil {
		return nil, fmt.Errorf("get access cookie key: %w", err)
	}
	if conf != nil && len(conf.Value) >= 32 {
		return conf.Value, nil
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate access cookie key: %w", err)
	}
	if err := state.PutConfig(key, secret); err != nil {
		return nil, fmt.Errorf("store access cookie key: %w", err)
	}
	return secret, nil
}
