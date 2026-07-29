package indexdb

import (
	"context"
	"errors"
	"fmt"

	glex "github.com/streamplace/glex/runtime"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/placestream"
)

type MetadataConfiguration struct {
	RepoDID string `json:"repoDID"        gorm:"primarykey;column:repo_did"`
	Repo    *Repo  `json:"repo,omitempty" gorm:"foreignKey:DID;references:RepoDID"`
	Record  *[]byte
}

func (m *MetadataConfiguration) ToRecord() (placestream.MetadataConfiguration, error) {
	var sdm placestream.MetadataConfiguration
	if err := glex.DecodeCBOR(*m.Record, &sdm); err != nil {
		return placestream.MetadataConfiguration{}, fmt.Errorf("error decoding metadata configuration: %w", err)
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

func (m *DBModel) GetMetadataConfiguration(ctx context.Context, repoDID string) (*placestream.MetadataConfiguration, error) {
	var metadata MetadataConfiguration
	err := m.DB.Where("repo_did = ?", repoDID).First(&metadata).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rec, err := metadata.ToRecord()
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (m *DBModel) DeleteMetadataConfiguration(ctx context.Context, repoDID string) error {
	err := m.DB.Where("repo_did = ?", repoDID).Delete(&MetadataConfiguration{}).Error
	if err != nil {
		return err
	}
	return nil
}
