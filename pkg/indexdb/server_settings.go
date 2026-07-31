package indexdb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
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

// ToRecord converts the model to a streamplace ServerSettings
func (m *ServerSettings) ToRecord() (placestream.ServerSettings, error) {
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
// UpsertServerSettings indexes a place.stream.server.settings record.
// The server column is the record's rkey (server settings are keyed by
// the server they apply to, carried in the record key); the repo DID
// comes from the AT-URI authority.
func (m *DBModel) UpsertServerSettings(ctx context.Context, aturi syntax.ATURI, rec placestream.ServerSettings) error {
	repoDID, _, blob, err := recordParts(aturi, &rec)
	if err != nil {
		return err
	}
	now := time.Now()
	return m.DB.Save(&ServerSettings{
		Server:  aturi.RecordKey().String(),
		RepoDID: repoDID,
		Record:  &blob,
		Created: now,
		Updated: now,
	}).Error
}

// GetServerSettings retrieves server settings for a given server and repoDID
func (m *DBModel) GetServerSettings(ctx context.Context, server string, repoDID string) (*placestream.ServerSettings, error) {
	var settings ServerSettings
	err := m.DB.Where("server = ? AND repo_did = ?", server, repoDID).First(&settings).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rec, err := settings.ToRecord()
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// DeleteServerSettings deletes server settings for a given server and repoDID
func (m *DBModel) DeleteServerSettings(ctx context.Context, server string, repoDID string) error {
	return m.DB.Where("server = ? AND repo_did = ?", server, repoDID).Delete(&ServerSettings{}).Error
}
