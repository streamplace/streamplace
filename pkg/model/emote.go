package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type EmotePack struct {
	URI          string    `gorm:"primaryKey;column:uri"`
	CID          string    `gorm:"column:cid"`
	RepoDID      string    `gorm:"column:repo_did;index"`
	RKey         string    `gorm:"column:rkey"`
	Name         string    `gorm:"column:name"`
	OpenInMyChat bool      `gorm:"column:open_in_my_chat;default:false"`
	Record       []byte    `gorm:"column:record"`
	IndexedAt    time.Time `gorm:"column:indexed_at"`
}

type EmotePackDelegation struct {
	URI          string    `gorm:"primaryKey;column:uri"`
	CID          string    `gorm:"column:cid"`
	RepoDID      string    `gorm:"column:repo_did;index"`
	RKey         string    `gorm:"column:rkey"`
	PackURI      string    `gorm:"column:pack_uri;index"`
	RecipientDID string    `gorm:"column:recipient_did;index"`
	// JSON-encoded []string of allowed emote URIs; null means all emotes in the pack.
	AllowedEmotes []byte    `gorm:"column:allowed_emotes"`
	Record        []byte    `gorm:"column:record"`
	IndexedAt     time.Time `gorm:"column:indexed_at"`
}

// DelegatedPack pairs a pack with the delegation record that grants access to it.
type DelegatedPack struct {
	Pack       *EmotePack
	Delegation *EmotePackDelegation
}

// AllowedEmoteSet returns a set of allowed emote URIs, or nil if all are allowed.
func (d *EmotePackDelegation) AllowedEmoteSet() (map[string]bool, error) {
	if d.AllowedEmotes == nil {
		return nil, nil
	}
	var uris []string
	if err := json.Unmarshal(d.AllowedEmotes, &uris); err != nil {
		return nil, fmt.Errorf("failed to parse allowed emotes: %w", err)
	}
	set := make(map[string]bool, len(uris))
	for _, u := range uris {
		set[u] = true
	}
	return set, nil
}

type EmoteItem struct {
	URI           string    `gorm:"primaryKey;column:uri"`
	CID           string    `gorm:"column:cid"`
	RepoDID       string    `gorm:"column:repo_did;index"`
	RKey          string    `gorm:"column:rkey"`
	PackURI       string    `gorm:"column:pack_uri;index"`
	Name          string    `gorm:"column:name"`
	ImageCID      string    `gorm:"column:image_cid"`
	ImageMimeType string    `gorm:"column:image_mime_type"`
	Alt           string    `gorm:"column:alt"`
	CreatorDID    string    `gorm:"column:creator_did"`
	Record        []byte    `gorm:"column:record"`
	IndexedAt     time.Time `gorm:"column:indexed_at"`
}

func (m *DBModel) UpsertEmotePack(ctx context.Context, pack *EmotePack) error {
	return m.DB.Save(pack).Error
}

func (m *DBModel) GetEmotePackByURI(ctx context.Context, uri string) (*EmotePack, error) {
	var pack EmotePack
	err := m.DB.Where("uri = ?", uri).First(&pack).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get emote pack: %w", err)
	}
	return &pack, nil
}

func (m *DBModel) GetEmotePacksByDID(ctx context.Context, did string) ([]*EmotePack, error) {
	var packs []*EmotePack
	err := m.DB.Where("repo_did = ?", did).Find(&packs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get emote packs: %w", err)
	}
	return packs, nil
}

func (m *DBModel) GetAllEmotePacks(ctx context.Context) ([]*EmotePack, error) {
	var packs []*EmotePack
	err := m.DB.Order("indexed_at asc").Find(&packs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get all emote packs: %w", err)
	}
	return packs, nil
}

func (m *DBModel) UpsertEmoteItem(ctx context.Context, item *EmoteItem) error {
	return m.DB.Save(item).Error
}

func (m *DBModel) GetEmoteItemsByPack(ctx context.Context, packURI string) ([]*EmoteItem, error) {
	var items []*EmoteItem
	err := m.DB.Where("pack_uri = ?", packURI).Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get emote items: %w", err)
	}
	return items, nil
}

func (m *DBModel) GetEmoteItemByURI(ctx context.Context, uri string) (*EmoteItem, error) {
	var item EmoteItem
	err := m.DB.Where("uri = ?", uri).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get emote item: %w", err)
	}
	return &item, nil
}

func (m *DBModel) DeleteEmoteItem(ctx context.Context, uri string) error {
	return m.DB.Where("uri = ?", uri).Delete(&EmoteItem{}).Error
}

func (m *DBModel) DeleteEmotePack(ctx context.Context, uri string) error {
	if err := m.DB.Where("pack_uri = ?", uri).Delete(&EmoteItem{}).Error; err != nil {
		return fmt.Errorf("failed to delete emote items for pack: %w", err)
	}
	return m.DB.Where("uri = ?", uri).Delete(&EmotePack{}).Error
}

func (m *DBModel) GetStreamerOpenPacks(ctx context.Context, streamerDID string) ([]*EmotePack, error) {
	var packs []*EmotePack
	err := m.DB.Where("repo_did = ? AND open_in_my_chat = ?", streamerDID, true).Find(&packs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get open packs for streamer: %w", err)
	}
	return packs, nil
}

func (m *DBModel) UpsertEmotePackDelegation(ctx context.Context, d *EmotePackDelegation) error {
	return m.DB.Save(d).Error
}

func (m *DBModel) DeleteEmotePackDelegation(ctx context.Context, uri string) error {
	return m.DB.Where("uri = ?", uri).Delete(&EmotePackDelegation{}).Error
}

func (m *DBModel) GetDelegatedPacksForUser(ctx context.Context, recipientDID string) ([]*DelegatedPack, error) {
	var delegations []*EmotePackDelegation
	err := m.DB.Where("recipient_did = ?", recipientDID).Find(&delegations).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get pack delegations: %w", err)
	}

	result := make([]*DelegatedPack, 0, len(delegations))
	for _, d := range delegations {
		pack, err := m.GetEmotePackByURI(ctx, d.PackURI)
		if err != nil {
			return nil, fmt.Errorf("failed to get pack %s: %w", d.PackURI, err)
		}
		if pack == nil {
			continue
		}
		result = append(result, &DelegatedPack{Pack: pack, Delegation: d})
	}
	return result, nil
}
