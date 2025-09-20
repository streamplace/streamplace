package statedb

import (
	"time"

	"github.com/google/uuid"
)

type MultistreamEvent struct {
	ID        string    `gorm:"column:id;primarykey"`
	TargetURI string    `gorm:"column:target_uri;primarykey;index:idx_target_created,priority:1"`
	Message   string    `gorm:"column:message"`
	Status    string    `gorm:"column:status"`
	CreatedAt time.Time `gorm:"column:created_at;index:idx_target_created,priority:2"`
}

func (m *MultistreamEvent) TableName() string {
	return "multistream_events"
}

func (state *StatefulDB) CreateMultistreamEvent(targetURI, message, status string) error {
	uu, err := uuid.NewV7()
	if err != nil {
		return err
	}
	event := &MultistreamEvent{
		ID:        uu.String(),
		TargetURI: targetURI,
		Message:   message,
		Status:    status,
		CreatedAt: time.Now().UTC(),
	}
	return state.DB.Create(event).Error
}
