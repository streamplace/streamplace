package indexdb

import (
	"context"
	"errors"
	"fmt"

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

func (m *DBModel) CreateChatProfile(ctx context.Context, profile *ChatProfile) error {
	err := m.DB.Save(profile).Error
	if err != nil {
		return err
	}
	return nil
}

func (m *DBModel) GetChatProfile(ctx context.Context, repoDID string) (*ChatProfile, error) {
	var profile ChatProfile
	err := m.DB.Where("repo_did = ?", repoDID).First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func ColorToHex(color *placestream.ChatProfile_Color) string {
	if color == nil {
		return "#f8baca"
	}
	hex := fmt.Sprintf("#%02x%02x%02x", color.Red, color.Green, color.Blue)
	return hex
}
