package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Teleport struct {
	URI             string    `json:"uri" gorm:"primaryKey;column:uri"`
	CID             string    `json:"cid" gorm:"column:cid"`
	StartsAt        time.Time `json:"startsAt" gorm:"column:starts_at;index:idx_repo_starts,priority:2"`
	DurationSeconds *int64    `json:"durationSeconds" gorm:"column:duration_seconds"`
	Teleport        *[]byte   `json:"teleport"`
	RepoDID         string    `json:"repoDID" gorm:"column:repo_did;index:idx_repo_starts,priority:1"`
	TargetDID       string    `json:"targetDID" gorm:"column:target_did;index:idx_target_did"`
	Repo            *Repo     `json:"repo,omitempty" gorm:"foreignKey:DID;references:RepoDID"`
	Target          *Repo     `json:"target,omitempty" gorm:"foreignKey:DID;references:TargetDID"`
}

func (m *DBModel) CreateTeleport(ctx context.Context, tp *Teleport) error {
	return m.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "uri"}},
		DoUpdates: clause.AssignmentColumns([]string{"cid", "starts_at", "duration_seconds", "teleport", "repo_did", "target_did"}),
	}).Create(tp).Error
}

func (m *DBModel) GetLatestTeleportForRepo(repoDID string) (*Teleport, error) {
	var teleport Teleport
	err := m.DB.
		Preload("Repo").
		Preload("Target").
		Where("repo_did = ?", repoDID).
		Order("starts_at DESC").
		First(&teleport).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error retrieving latest teleport: %w", err)
	}
	return &teleport, nil
}

func (m *DBModel) GetActiveTeleportsForRepo(repoDID string) ([]Teleport, error) {
	now := time.Now()
	var teleports []Teleport
	err := m.DB.
		Preload("Repo").
		Preload("Target").
		Where("repo_did = ?", repoDID).
		Where("starts_at <= ?", now).
		Where("(duration_seconds IS NULL OR DATE_ADD(starts_at, INTERVAL duration_seconds SECOND) > ?)", now).
		Order("starts_at DESC").
		Find(&teleports).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error retrieving active teleports: %w", err)
	}
	return teleports, nil
}
