package model

import (
	"context"
	"errors"
	"fmt"

	lexutil "github.com/bluesky-social/indigo/lex/util"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/streamplace"
)

type ChatProfile struct {
	RepoDID string `json:"repoDID"        gorm:"primarykey;column:repo_did"`
	Repo    *Repo  `json:"repo,omitempty" gorm:"foreignKey:DID;references:RepoDID"`
	Record  *[]byte
}

func (m *ChatProfile) ToStreamplaceChatProfile() (*streamplace.ChatProfile, error) {
	rec, err := lexutil.CborDecodeValue(*m.Record)
	if err != nil {
		return nil, fmt.Errorf("error decoding feed post: %w", err)
	}
	scp, ok := rec.(*streamplace.ChatProfile)
	if !ok {
		return nil, fmt.Errorf("invalid chat profile")
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

func ColorToHex(color *streamplace.ChatProfile_Color) string {
	if color == nil {
		return "#f8baca"
	}
	hex := fmt.Sprintf("#%02x%02x%02x", color.Red, color.Green, color.Blue)
	return hex
}
