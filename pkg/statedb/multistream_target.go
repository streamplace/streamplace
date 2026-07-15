package statedb

import (
	"bytes"
	"fmt"
	"time"

	"github.com/bluesky-social/indigo/util"
	glex "github.com/streamplace/glex/runtime"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spid"
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

func (state *StatefulDB) CreateMultistreamTarget(input placestream.MultistreamCreateTarget_Input, repoDID string) (placestream.MultistreamDefs_TargetView, error) {
	// Check total targets limit
	var totalCount int64
	err := state.DB.Model(&MultistreamTarget{}).Where("repo_did = ?", repoDID).Count(&totalCount).Error
	if err != nil {
		return placestream.MultistreamDefs_TargetView{}, fmt.Errorf("failed to count existing targets: %w", err)
	}
	if totalCount >= MAX_MULTISTREAM_TARGETS {
		return placestream.MultistreamDefs_TargetView{}, fmt.Errorf("maximum number of multistream targets (%d) reached", MAX_MULTISTREAM_TARGETS)
	}

	// Check active targets limit if this target is active
	if input.MultistreamTarget.Active {
		var activeCount int64
		err := state.DB.Model(&MultistreamTarget{}).Where("repo_did = ? AND active = ?", repoDID, true).Count(&activeCount).Error
		if err != nil {
			return placestream.MultistreamDefs_TargetView{}, fmt.Errorf("failed to count active targets: %w", err)
		}
		if activeCount >= MAX_ACTIVE_MULTISTREAM_TARGETS {
			return placestream.MultistreamDefs_TargetView{}, fmt.Errorf("maximum number of active multistream targets (%d) reached", MAX_ACTIVE_MULTISTREAM_TARGETS)
		}
	}

	// this URI is, of course, a LIE
	tid := spid.TIDClock.Next()
	uri := fmt.Sprintf("at://%s/place.stream.multistream.target/%s", repoDID, tid.String())

	cid, err := spid.GetCID(&input.MultistreamTarget)
	if err != nil {
		return placestream.MultistreamDefs_TargetView{}, fmt.Errorf("failed to get CID: %w", err)
	}

	buf := bytes.Buffer{}
	err = input.MultistreamTarget.MarshalCBOR(&buf)
	if err != nil {
		return placestream.MultistreamDefs_TargetView{}, fmt.Errorf("failed to marshal multistream target: %w", err)
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
		return placestream.MultistreamDefs_TargetView{}, err
	}
	return placestream.MultistreamDefs_TargetView{
		Uri:    uri,
		Cid:    cid.String(),
		Record: &glex.LexiconTypeDecoder{Val: &input.MultistreamTarget},
	}, nil
}

func (state *StatefulDB) GetMultistreamTarget(uri string) (placestream.MultistreamDefs_TargetView, error) {
	return placestream.MultistreamDefs_TargetView{}, nil
}

type TargetWithEvent struct {
	MultistreamTarget
	LatestEventID        *string    `gorm:"column:latest_event_id"`
	LatestEventStatus    *string    `gorm:"column:latest_event_status"`
	LatestEventMessage   *string    `gorm:"column:latest_event_message"`
	LatestEventCreatedAt *time.Time `gorm:"column:latest_event_created_at"`
}

func (state *StatefulDB) ListMultistreamTargets(repoDID string, limit int, offset int, active *bool) ([]placestream.MultistreamDefs_TargetView, error) {

	var targets []TargetWithEvent
	query := state.DB.Table("multistream_targets").
		Select("multistream_targets.*, me.id as latest_event_id, me.status as latest_event_status, me.message as latest_event_message, me.created_at as latest_event_created_at").
		Joins(`LEFT JOIN multistream_events me ON multistream_targets.uri = me.target_uri 
		       AND me.created_at = (SELECT MAX(created_at) FROM multistream_events WHERE target_uri = multistream_targets.uri)`).
		Where("repo_did = ?", repoDID)

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

	result := make([]placestream.MultistreamDefs_TargetView, len(targets))
	for i, target := range targets {
		var multistreamTarget placestream.MultistreamTarget
		err = multistreamTarget.UnmarshalCBOR(bytes.NewReader(target.MultistreamTarget.MultistreamTarget))
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal multistream target: %w", err)
		}
		cid, err := spid.GetCID(&multistreamTarget)
		if err != nil {
			return nil, fmt.Errorf("failed to get CID: %w", err)
		}

		targetView := placestream.MultistreamDefs_TargetView{
			Uri:    target.URI,
			Cid:    cid.String(),
			Record: &glex.LexiconTypeDecoder{Val: &multistreamTarget},
		}

		// Add the latest event if it exists
		if target.LatestEventID != nil {
			event := placestream.MultistreamDefs_Event{
				Status:    *target.LatestEventStatus,
				Message:   *target.LatestEventMessage,
				CreatedAt: target.LatestEventCreatedAt.Format(util.ISO8601),
			}
			targetView.LatestEvent = &event
		}

		result[i] = targetView
	}

	return result, nil
}

func (state *StatefulDB) UpdateMultistreamTarget(uri string, input placestream.MultistreamPutTarget_Input) (placestream.MultistreamDefs_TargetView, error) {
	// Get the current target to check repo ownership and current active status
	var currentTarget MultistreamTarget
	err := state.DB.Where("uri = ?", uri).First(&currentTarget).Error
	if err != nil {
		return placestream.MultistreamDefs_TargetView{}, fmt.Errorf("multistream target not found")
	}

	// If updating to active and wasn't previously active, check active targets limit
	if input.MultistreamTarget.Active && !currentTarget.Active {
		var activeCount int64
		err := state.DB.Model(&MultistreamTarget{}).Where("repo_did = ? AND active = ?", currentTarget.RepoDID, true).Count(&activeCount).Error
		if err != nil {
			return placestream.MultistreamDefs_TargetView{}, fmt.Errorf("failed to count active targets: %w", err)
		}
		if activeCount >= MAX_ACTIVE_MULTISTREAM_TARGETS {
			return placestream.MultistreamDefs_TargetView{}, fmt.Errorf("maximum number of active multistream targets (%d) reached", MAX_ACTIVE_MULTISTREAM_TARGETS)
		}
	}

	// Get CID for the updated target
	cid, err := spid.GetCID(&input.MultistreamTarget)
	if err != nil {
		return placestream.MultistreamDefs_TargetView{}, fmt.Errorf("failed to get CID: %w", err)
	}

	// Marshal the target data
	buf := bytes.Buffer{}
	err = input.MultistreamTarget.MarshalCBOR(&buf)
	if err != nil {
		return placestream.MultistreamDefs_TargetView{}, fmt.Errorf("failed to marshal multistream target: %w", err)
	}

	// Update the database record
	updates := map[string]interface{}{
		"cid":    cid.String(),
		"record": buf.Bytes(),
		"active": input.MultistreamTarget.Active,
	}

	result := state.DB.Model(&MultistreamTarget{}).Where("uri = ?", uri).Updates(updates)
	if result.Error != nil {
		return placestream.MultistreamDefs_TargetView{}, fmt.Errorf("failed to update multistream target: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return placestream.MultistreamDefs_TargetView{}, fmt.Errorf("multistream target not found")
	}

	return placestream.MultistreamDefs_TargetView{
		Uri:    uri,
		Cid:    cid.String(),
		Record: &glex.LexiconTypeDecoder{Val: &input.MultistreamTarget},
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
