package model

import (
	"context"
	"errors"
	"fmt"

	lexutil "github.com/bluesky-social/indigo/lex/util"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/streamplace"
)

type BioPage struct {
	RepoDID string `json:"repoDID"        gorm:"primarykey;column:repo_did"`
	Repo    *Repo  `json:"repo,omitempty" gorm:"foreignKey:DID;references:RepoDID"`
	Record  *[]byte
}

func (m *BioPage) ToStreamplaceBioPage() (*streamplace.BioPage, error) {
	if m == nil || m.Record == nil {
		return nil, fmt.Errorf("bio page is nil")
	}
	rec, err := lexutil.CborDecodeValue(*m.Record)
	if err != nil {
		return nil, fmt.Errorf("error decoding bio page: %w", err)
	}
	bp, ok := rec.(*streamplace.BioPage)
	if !ok {
		return nil, fmt.Errorf("invalid bio page")
	}
	return bp, nil
}

func (m *DBModel) CreateBioPage(ctx context.Context, bioPage *BioPage) error {
	err := m.DB.Save(bioPage).Error
	if err != nil {
		return err
	}
	return nil
}

func (m *DBModel) GetBioPage(ctx context.Context, repoDID string) (*BioPage, error) {
	var bioPage BioPage
	err := m.DB.Where("repo_did = ?", repoDID).First(&bioPage).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &bioPage, nil
}
