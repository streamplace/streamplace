package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type EmotePack struct {
	URI       string    `gorm:"primaryKey;column:uri"`
	CID       string    `gorm:"column:cid"`
	RepoDID   string    `gorm:"column:repo_did;index"`
	RKey      string    `gorm:"column:rkey"`
	Name      string    `gorm:"column:name"`
	Record    []byte    `gorm:"column:record"`
	IndexedAt time.Time `gorm:"column:indexed_at"`
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
