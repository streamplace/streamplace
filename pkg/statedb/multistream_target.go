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

func (state *StatefulDB) ListMultistreamTargets(repoDID string) ([]*streamplace.MultistreamDefs_TargetView, error) {
	var targets []MultistreamTarget
	err := state.DB.Where("repo_did = ?", repoDID).Find(&targets).Error
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

		result[i] = &streamplace.MultistreamDefs_TargetView{
			Uri:    target.URI,
			Cid:    target.CID,
			Record: &util.LexiconTypeDecoder{Val: &multistreamTarget},
		}
	}

	return result, nil
}

func (state *StatefulDB) UpdateMultistreamTarget(uri string, input *streamplace.MultistreamCreateTarget_Input) (*streamplace.MultistreamDefs_TargetView, error) {
	return nil, nil
}

func (state *StatefulDB) DeleteMultistreamTarget(uri string) error {
	return nil
}
