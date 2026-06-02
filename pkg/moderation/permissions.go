package moderation

import (
	"context"
	"fmt"
	"time"

	"stream.place/streamplace/pkg/streamplace"
)

type delegationGetter interface {
	GetModerationDelegations(ctx context.Context, streamerDID, moderatorDID string) ([]*streamplace.ModerationDefs_PermissionView, error)
}

// Permission scope constants
const (
	PermissionBan              = "ban"
	PermissionHide             = "hide"
	PermissionLivestreamManage = "livestream.manage"
	PermissionMessagePin       = "message.pin"
	PermissionVodCommentHide   = "vod.comment.hide"
)

// ActionPermissions maps moderation actions to required permissions
var ActionPermissions = map[string]string{
	"createBlock":      PermissionBan,
	"deleteBlock":      PermissionBan,
	"createGate":       PermissionHide,
	"deleteGate":       PermissionHide,
	"createVodGate":    PermissionVodCommentHide,
	"deleteVodGate":    PermissionVodCommentHide,
	"updateLivestream": PermissionLivestreamManage,
	"createPin":        PermissionMessagePin,
	"deletePin":        PermissionMessagePin,
}

// PermissionChecker validates moderation permissions
type PermissionChecker struct {
	model delegationGetter
}

// NewPermissionChecker creates a new permission checker
func NewPermissionChecker(m delegationGetter) *PermissionChecker {
	return &PermissionChecker{model: m}
}

// CheckPermission validates that a moderator has permission to perform an action
// Returns an error if the moderator lacks the required permission
func (pc *PermissionChecker) CheckPermission(ctx context.Context, moderatorDID, streamerDID, action string) error {
	// Get required permission for this action (validate action first)
	requiredPermission, ok := ActionPermissions[action]
	if !ok {
		return fmt.Errorf("unknown action: %s", action)
	}

	// Streamers always have permission for their own content
	if moderatorDID == streamerDID {
		return nil
	}

	// Check if moderator has the required permission
	hasPermission, err := pc.HasPermission(ctx, moderatorDID, streamerDID, requiredPermission)
	if err != nil {
		return fmt.Errorf("failed to check permissions: %w", err)
	}

	if !hasPermission {
		return fmt.Errorf("moderator %s does not have permission '%s' for streamer %s", moderatorDID, requiredPermission, streamerDID)
	}

	return nil
}

// HasPermission checks if a moderator has a specific permission for a streamer.
// It merges permissions from ALL delegation records for the moderator.
func (pc *PermissionChecker) HasPermission(ctx context.Context, moderatorDID, streamerDID, permission string) (bool, error) {
	// Streamers always have all permissions for their own content
	if moderatorDID == streamerDID {
		return true, nil
	}

	// Look up ALL delegation records for this moderator
	delegations, err := pc.model.GetModerationDelegations(ctx, streamerDID, moderatorDID)
	if err != nil {
		return false, fmt.Errorf("failed to get moderation delegations: %w", err)
	}

	if len(delegations) == 0 {
		return false, nil
	}

	// Check all delegation records and merge their permissions
	for _, delegationView := range delegations {
		// Extract the actual permission record from the view
		permRecord, ok := delegationView.Record.Val.(*streamplace.ModerationPermission)
		if !ok {
			return false, fmt.Errorf("failed to cast record to ModerationPermission")
		}

		// Skip expired delegations
		if permRecord.ExpirationTime != nil {
			expirationTime, err := time.Parse(time.RFC3339, *permRecord.ExpirationTime)
			if err != nil {
				return false, fmt.Errorf("failed to parse expiration time: %w", err)
			}
			if time.Now().After(expirationTime) {
				continue
			}
		}

		// Check if this delegation has the required permission
		for _, p := range permRecord.Permissions {
			if p == permission {
				return true, nil
			}
		}
	}

	return false, nil
}
