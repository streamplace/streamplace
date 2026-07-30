package indexdb

import (
	"context"
	"errors"
	"fmt"

	"github.com/bluesky-social/indigo/atproto/syntax"
	glex "github.com/streamplace/glex/runtime"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/placestream"
)

type ChatProfile struct {
	RepoDID string `json:"repoDID"        gorm:"primarykey;column:repo_did"`
	Repo    *Repo  `json:"repo,omitempty" gorm:"foreignKey:DID;references:RepoDID"`
	Record  *[]byte
}

func (m *ChatProfile) ToRecord() (placestream.ChatProfile, error) {
	if m == nil || m.Record == nil {
		return placestream.ChatProfile{}, fmt.Errorf("chat profile is nil")
	}
	var scp placestream.ChatProfile
	if err := glex.DecodeCBOR(*m.Record, &scp); err != nil {
		return placestream.ChatProfile{}, fmt.Errorf("error decoding chat profile: %w", err)
	}
	return scp, nil
}

// UpsertChatProfile indexes a place.stream.chat.profile record,
// deriving the stored CBOR blob from the record.
func (m *DBModel) UpsertChatProfile(ctx context.Context, rec placestream.ChatProfile, aturi syntax.ATURI) error {
	repoDID, _, blob, err := recordParts(aturi, &rec)
	if err != nil {
		return err
	}
	return m.DB.Save(&ChatProfile{
		RepoDID: repoDID,
		Record:  &blob,
	}).Error
}

func (m *DBModel) GetChatProfile(ctx context.Context, repoDID string) (*placestream.ChatProfile, error) {
	var profile ChatProfile
	err := m.DB.Where("repo_did = ?", repoDID).First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rec, err := profile.ToRecord()
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func ColorToHex(color *placestream.ChatProfile_Color) string {
	if color == nil {
		return "#f8baca"
	}
	hex := fmt.Sprintf("#%02x%02x%02x", color.Red, color.Green, color.Blue)
	return hex
}
