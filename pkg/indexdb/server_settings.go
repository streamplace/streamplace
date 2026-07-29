package indexdb

import (
	"context"
	"errors"
	"fmt"
	"time"

	glex "github.com/streamplace/glex/runtime"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/placestream"
)

// ServerSettings represents a user's settings for a particular Streamplace node
type ServerSettings struct {
	Server  string    `gorm:"primaryKey;column:server"`
	RepoDID string    `gorm:"primaryKey;column:repo_did"`
	Record  *[]byte   `gorm:"column:record"`
	Created time.Time `gorm:"column:created;not null"`
	Updated time.Time `gorm:"column:updated;not null"`
}

// TableName specifies the table name for the ServerSettings model
func (ServerSettings) TableName() string {
	return "server_settings"
}

// ToStreamplaceServerSettings converts the model to a streamplace ServerSettings
func (m *ServerSettings) ToStreamplaceServerSettings() (placestream.ServerSettings, error) {
	if m.Record == nil {
		return placestream.ServerSettings{}, fmt.Errorf("no record data")
	}
	var ss placestream.ServerSettings
	if err := glex.DecodeCBOR(*m.Record, &ss); err != nil {
		return placestream.ServerSettings{}, fmt.Errorf("error decoding server settings: %w", err)
	}
	return ss, nil
}

// UpdateServerSettings creates or updates a server settings record
func (m *DBModel) UpdateServerSettings(ctx context.Context, settings *ServerSettings) error {
	now := time.Now()
	if settings.Created.IsZero() {
		settings.Created = now
	}
	settings.Updated = now
	return m.DB.Save(settings).Error
}

// GetServerSettings retrieves server settings for a given server and repoDID
func (m *DBModel) GetServerSettings(ctx context.Context, server string, repoDID string) (*ServerSettings, error) {
	var settings ServerSettings
	err := m.DB.Where("server = ? AND repo_did = ?", server, repoDID).First(&settings).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

// DeleteServerSettings deletes server settings for a given server and repoDID
func (m *DBModel) DeleteServerSettings(ctx context.Context, server string, repoDID string) error {
	return m.DB.Where("server = ? AND repo_did = ?", server, repoDID).Delete(&ServerSettings{}).Error
}
