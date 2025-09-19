package statedb

import (
	"bytes"
	"fmt"

	"github.com/bluesky-social/indigo/lex/util"
	"stream.place/streamplace/pkg/spid"
	"stream.place/streamplace/pkg/streamplace"
)

type MultistreamTarget struct {
	URI               string `gorm:"column:uri;primarykey"`
	CID               string `gorm:"column:cid;not null"`
	RepoDID           string `gorm:"column:repo_did;not null;index"`
	MultistreamTarget []byte `gorm:"column:record"`
}

func (m *MultistreamTarget) TableName() string {
	return "multistream_targets"
}

func (state *StatefulDB) CreateMultistreamTarget(input *streamplace.MultistreamCreateTarget_Input, repoDID string) (*streamplace.MultistreamDefs_TargetView, error) {
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

func (state *StatefulDB) ListMultistreamTargets(repoDID string, limit int, offset int) ([]*streamplace.MultistreamDefs_TargetView, error) {
	var targets []MultistreamTarget
	err := state.DB.Where("repo_did = ?", repoDID).
		Limit(limit).
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
		"cid":                cid.String(),
		"multistream_target": buf.Bytes(),
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
