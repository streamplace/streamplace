package model

import (
	"context"
	"errors"
	"fmt"

	lexutil "github.com/bluesky-social/indigo/lex/util"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/streamplace"
)

type MetadataConfiguration struct {
	RepoDID string `json:"repoDID"        gorm:"primarykey;column:repo_did"`
	Repo    *Repo  `json:"repo,omitempty" gorm:"foreignKey:DID;references:RepoDID"`
	Record  *[]byte
}

func (m *MetadataConfiguration) ToStreamplaceMetadataConfiguration() (*streamplace.MetadataConfiguration, error) {
	rec, err := lexutil.CborDecodeValue(*m.Record)
	if err != nil {
		return nil, fmt.Errorf("error decoding feed post: %w", err)
	}
	sdm, ok := rec.(*streamplace.MetadataConfiguration)
	if !ok {
		return nil, fmt.Errorf("invalid metadata configuration")
	}
	return sdm, nil
}

func (m *DBModel) CreateMetadataConfiguration(ctx context.Context, metadata *MetadataConfiguration) error {
	err := m.DB.Save(metadata).Error
	if err != nil {
		return err
	}
	return nil
}

func (m *DBModel) GetMetadataConfiguration(ctx context.Context, repoDID string) (*MetadataConfiguration, error) {
	var metadata MetadataConfiguration
	err := m.DB.Where("repo_did = ?", repoDID).First(&metadata).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &metadata, nil
}

func (m *DBModel) DeleteMetadataConfiguration(ctx context.Context, repoDID string) error {
	err := m.DB.Where("repo_did = ?", repoDID).Delete(&MetadataConfiguration{}).Error
	if err != nil {
		return err
	}
	return nil
}
