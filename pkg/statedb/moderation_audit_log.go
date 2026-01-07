package statedb

import (
	"context"
	"time"
)

type ModerationAuditLog struct {
	ID           uint      `gorm:"primaryKey;autoIncrement"`
	StreamerDID  string    `gorm:"column:streamer_did;index:idx_streamer_time,priority:1"`
	ModeratorDID string    `gorm:"column:moderator_did;index:idx_audit_moderator"`
	Action       string    `gorm:"column:action"`     // "createBlock", "deleteBlock", "createGate", "deleteGate", "updateLivestream"
	TargetURI    string    `gorm:"column:target_uri"` // URI of affected resource
	TargetDID    string    `gorm:"column:target_did"` // DID of affected user (for blocks)
	ResultURI    string    `gorm:"column:result_uri"` // URI of created/deleted record
	Success      bool      `gorm:"column:success"`
	ErrorMsg     string    `gorm:"column:error_msg"`
	CreatedAt    time.Time `gorm:"column:created_at;index:idx_streamer_time,priority:2"`
}

func (ModerationAuditLog) TableName() string {
	return "moderation_audit_logs"
}

func (state *StatefulDB) CreateAuditLog(ctx context.Context, log *ModerationAuditLog) error {
	return state.DB.WithContext(ctx).Create(log).Error
}

func (state *StatefulDB) GetAuditLogs(ctx context.Context, streamerDID string, limit int, before *time.Time) ([]*ModerationAuditLog, error) {
	var logs []*ModerationAuditLog
	query := state.DB.WithContext(ctx).Where("streamer_did = ?", streamerDID).
		Order("created_at DESC").
		Limit(limit)

	if before != nil {
		query = query.Where("created_at < ?", *before)
	}

	err := query.Find(&logs).Error
	if err != nil {
		return nil, err
	}
	return logs, nil
}

func (state *StatefulDB) GetModeratorAuditLogs(ctx context.Context, moderatorDID string, limit int, before *time.Time) ([]*ModerationAuditLog, error) {
	var logs []*ModerationAuditLog
	query := state.DB.WithContext(ctx).Where("moderator_did = ?", moderatorDID).
		Order("created_at DESC").
		Limit(limit)

	if before != nil {
		query = query.Where("created_at < ?", *before)
	}

	err := query.Find(&logs).Error
	if err != nil {
		return nil, err
	}
	return logs, nil
}
