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

type ModerationDelegation struct {
	RKey         string    `gorm:"primaryKey;column:rkey"`
	CID          string    `gorm:"column:cid"`
	RepoDID      string    `json:"repoDID" gorm:"column:repo_did;index:idx_repo_moderator,priority:1"`
	Repo         *Repo     `json:"repo,omitempty" gorm:"foreignKey:DID;references:RepoDID"`
	ModeratorDID string    `gorm:"column:moderator_did;index:idx_repo_moderator,priority:2;index:idx_moderator"`
	Record       []byte    `gorm:"column:record"` // Full CBOR record
	CreatedAt    time.Time `gorm:"column:created_at"`
	IndexedAt    time.Time `gorm:"column:indexed_at"`
}

func (md *ModerationDelegation) ToPermissionView() (placestream.ModerationDefs_PermissionView, error) {
	rec, err := glexrt.CborDecodeValue(md.Record)
	if err != nil {
		return placestream.ModerationDefs_PermissionView{}, fmt.Errorf("error decoding moderation permission: %w", err)
	}

	uri := fmt.Sprintf("at://%s/place.stream.moderation.permission/%s", md.RepoDID, md.RKey)

	view := placestream.ModerationDefs_PermissionView{
		Author: appbsky.ActorDefs_ProfileViewBasic{
			Did: md.RepoDID,
		},
		Cid:    md.CID,
		Record: &glexrt.LexiconTypeDecoder{Val: rec},
		Uri:    uri,
	}

	if md.Repo != nil {
		view.Author.Handle = md.Repo.Handle
	}

	return view, nil
}

func (m *DBModel) CreateModerationDelegation(ctx context.Context, rec placestream.ModerationPermission, aturi syntax.ATURI) error {
	repoDID, err := aturi.Authority().AsDID()
	if err != nil {
		return fmt.Errorf("invalid ATURI authority: %w", err)
	}
	cid, err := spid.GetCID(&rec)
	if err != nil {
		return fmt.Errorf("failed to get CID: %w", err)
	}
	rkey := aturi.RecordKey().String()

	buf := bytes.Buffer{}
	err = rec.MarshalCBOR(&buf)
	if err != nil {
		return fmt.Errorf("failed to marshal moderation permission: %w", err)
	}

	now := aqtime.FromTime(time.Now().UTC())

	delegation := &ModerationDelegation{
		RKey:         rkey,
		CID:          cid.String(),
		RepoDID:      repoDID.String(),
		ModeratorDID: rec.Moderator,
		Record:       buf.Bytes(),
		CreatedAt:    now.Time().UTC(),
		IndexedAt:    now.Time().UTC(),
	}

	return m.DB.WithContext(ctx).Create(delegation).Error
}

func (m *DBModel) DeleteModerationDelegation(ctx context.Context, rkey string) error {
	return m.DB.WithContext(ctx).Where("rkey = ?", rkey).Delete(&ModerationDelegation{}).Error
}

func (m *DBModel) GetModerationDelegation(ctx context.Context, streamerDID, moderatorDID string) (placestream.ModerationDefs_PermissionView, error) {
	var delegation ModerationDelegation
	err := m.DB.WithContext(ctx).Preload("Repo").
		Where("repo_did = ? AND moderator_did = ?", streamerDID, moderatorDID).
		Order("created_at DESC").
		First(&delegation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return placestream.ModerationDefs_PermissionView{}, nil
	}
	if err != nil {
		return placestream.ModerationDefs_PermissionView{}, err
	}
	return delegation.ToPermissionView()
}

// GetModerationDelegations returns ALL delegation records for a moderator from a specific streamer.
// This allows multiple separate permission records (e.g., one for "ban", one for "hide") to be merged.
func (m *DBModel) GetModerationDelegations(ctx context.Context, streamerDID, moderatorDID string) ([]placestream.ModerationDefs_PermissionView, error) {
	var delegations []*ModerationDelegation
	err := m.DB.WithContext(ctx).Preload("Repo").
		Where("repo_did = ? AND moderator_did = ?", streamerDID, moderatorDID).
		Find(&delegations).Error
	if err != nil {
		return nil, err
	}

	views := make([]placestream.ModerationDefs_PermissionView, len(delegations))
	for i, d := range delegations {
		view, err := d.ToPermissionView()
		if err != nil {
			return nil, err
		}
		views[i] = view
	}
	return views, nil
}

func (m *DBModel) GetModeratorDelegations(ctx context.Context, moderatorDID string) ([]placestream.ModerationDefs_PermissionView, error) {
	var delegations []*ModerationDelegation
	err := m.DB.WithContext(ctx).Preload("Repo").
		Where("moderator_did = ?", moderatorDID).
		Find(&delegations).Error
	if err != nil {
		return nil, err
	}

	views := make([]placestream.ModerationDefs_PermissionView, len(delegations))
	for i, d := range delegations {
		view, err := d.ToPermissionView()
		if err != nil {
			return nil, err
		}
		views[i] = view
	}
	return views, nil
}

func (m *DBModel) GetStreamerModerators(ctx context.Context, streamerDID string) ([]placestream.ModerationDefs_PermissionView, error) {
	var delegations []*ModerationDelegation
	err := m.DB.WithContext(ctx).Preload("Repo").
		Where("repo_did = ?", streamerDID).
		Find(&delegations).Error
	if err != nil {
		return nil, err
	}

	views := make([]placestream.ModerationDefs_PermissionView, len(delegations))
	for i, d := range delegations {
		view, err := d.ToPermissionView()
		if err != nil {
			return nil, err
		}
		views[i] = view
	}
	return views, nil
}
