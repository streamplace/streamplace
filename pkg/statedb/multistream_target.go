package statedb

import (
	"bytes"
	"fmt"

	"github.com/bluesky-social/indigo/lex/util"
	"stream.place/streamplace/pkg/spid"
	"stream.place/streamplace/pkg/streamplace"
)

const MAX_MULTISTREAM_TARGETS = 100
const MAX_ACTIVE_MULTISTREAM_TARGETS = 5

type MultistreamTarget struct {
	URI               string `gorm:"column:uri;primarykey"`
	CID               string `gorm:"column:cid;not null"`
	Active            bool   `gorm:"column:active"`
	RepoDID           string `gorm:"column:repo_did;not null;index"`
	MultistreamTarget []byte `gorm:"column:record"`
}

func (m *MultistreamTarget) TableName() string {
	return "multistream_targets"
}

func (state *StatefulDB) CreateMultistreamTarget(input *streamplace.MultistreamCreateTarget_Input, repoDID string) (*streamplace.MultistreamDefs_TargetView, error) {
	// Check total targets limit
	var totalCount int64
	err := state.DB.Model(&MultistreamTarget{}).Where("repo_did = ?", repoDID).Count(&totalCount).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count existing targets: %w", err)
	}
	if totalCount >= MAX_MULTISTREAM_TARGETS {
		return nil, fmt.Errorf("maximum number of multistream targets (%d) reached", MAX_MULTISTREAM_TARGETS)
	}

	// Check active targets limit if this target is active
	if input.MultistreamTarget.Active {
		var activeCount int64
		err := state.DB.Model(&MultistreamTarget{}).Where("repo_did = ? AND active = ?", repoDID, true).Count(&activeCount).Error
		if err != nil {
			return nil, fmt.Errorf("failed to count active targets: %w", err)
		}
		if activeCount >= MAX_ACTIVE_MULTISTREAM_TARGETS {
			return nil, fmt.Errorf("maximum number of active multistream targets (%d) reached", MAX_ACTIVE_MULTISTREAM_TARGETS)
		}
	}

	// this URI is, of course, a LIE
	tid := spid.TIDClock.Next()
	uri := fmt.Sprintf("at://%s/place.stream.multistream.target/%s", repoDID, tid.String())

	cid, err := spid.GetCID(input.MultistreamTarget)
	if err != nil {
		return nil, fmt.Errorf("failed to get CID: %w", err)
	}

	buf := bytes.Buffer{}
	err = input.MultistreamTarget.MarshalCBOR(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal multistream target: %w", err)
	}

	dbTarget := &MultistreamTarget{
		URI:               uri,
		CID:               cid.String(),
		RepoDID:           repoDID,
		MultistreamTarget: buf.Bytes(),
		Active:            input.MultistreamTarget.Active,
	}
	err = state.DB.Create(dbTarget).Error
	if err != nil {
		return nil, err
	}
	return &streamplace.MultistreamDefs_TargetView{
		Uri:    uri,
		Cid:    cid.String(),
		Record: &util.LexiconTypeDecoder{Val: input.MultistreamTarget},
	}, nil
}

func (state *StatefulDB) GetMultistreamTarget(uri string) (*streamplace.MultistreamDefs_TargetView, error) {
	return nil, nil
}

func (state *StatefulDB) ListMultistreamTargets(repoDID string, limit int, offset int, active *bool) ([]*streamplace.MultistreamDefs_TargetView, error) {
	var targets []MultistreamTarget
	query := state.DB.Where("repo_did = ?", repoDID)

	if active != nil {
		query = query.Where("active = ?", *active)
	}

	err := query.Limit(limit).
		Offset(offset).
		Order("uri ASC").
		Find(&targets).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list multistream targets: %w", err)
	}

	result := make([]*streamplace.MultistreamDefs_TargetView, len(targets))
	for i, target := range targets {
		var multistreamTarget streamplace.MultistreamTarget
		err = multistreamTarget.UnmarshalCBOR(bytes.NewReader(target.MultistreamTarget))
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal multistream target: %w", err)
		}
		cid, err := spid.GetCID(&multistreamTarget)
		if err != nil {
			return nil, fmt.Errorf("failed to get CID: %w", err)
		}

		result[i] = &streamplace.MultistreamDefs_TargetView{
			Uri:    target.URI,
			Cid:    cid.String(),
			Record: &util.LexiconTypeDecoder{Val: &multistreamTarget},
		}
	}

	return result, nil
}

func (state *StatefulDB) UpdateMultistreamTarget(uri string, input *streamplace.MultistreamPutTarget_Input) (*streamplace.MultistreamDefs_TargetView, error) {
	if input.MultistreamTarget == nil {
		return nil, fmt.Errorf("multistream target is required")
	}

	// Get the current target to check repo ownership and current active status
	var currentTarget MultistreamTarget
	err := state.DB.Where("uri = ?", uri).First(&currentTarget).Error
	if err != nil {
		return nil, fmt.Errorf("multistream target not found")
	}

	// If updating to active and wasn't previously active, check active targets limit
	if input.MultistreamTarget.Active && !currentTarget.Active {
		var activeCount int64
		err := state.DB.Model(&MultistreamTarget{}).Where("repo_did = ? AND active = ?", currentTarget.RepoDID, true).Count(&activeCount).Error
		if err != nil {
			return nil, fmt.Errorf("failed to count active targets: %w", err)
		}
		if activeCount >= MAX_ACTIVE_MULTISTREAM_TARGETS {
			return nil, fmt.Errorf("maximum number of active multistream targets (%d) reached", MAX_ACTIVE_MULTISTREAM_TARGETS)
		}
	}

	// Get CID for the updated target
	cid, err := spid.GetCID(input.MultistreamTarget)
	if err != nil {
		return nil, fmt.Errorf("failed to get CID: %w", err)
	}

	// Marshal the target data
	buf := bytes.Buffer{}
	err = input.MultistreamTarget.MarshalCBOR(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal multistream target: %w", err)
	}

	// Update the database record
	updates := map[string]interface{}{
		"cid":    cid.String(),
		"record": buf.Bytes(),
		"active": input.MultistreamTarget.Active,
	}

	result := state.DB.Model(&MultistreamTarget{}).Where("uri = ?", uri).Updates(updates)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to update multistream target: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("multistream target not found")
	}

	return &streamplace.MultistreamDefs_TargetView{
		Uri:    uri,
		Cid:    cid.String(),
		Record: &util.LexiconTypeDecoder{Val: input.MultistreamTarget},
	}, nil
}

func (state *StatefulDB) DeleteMultistreamTarget(uri string) error {
	result := state.DB.Where("uri = ?", uri).Delete(&MultistreamTarget{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete multistream target: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("multistream target not found")
	}
	return nil
}
