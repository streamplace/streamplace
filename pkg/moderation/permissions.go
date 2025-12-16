package moderation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"stream.place/streamplace/pkg/model"
)

// Permission scope constants
const (
	PermissionBan              = "ban"
	PermissionHide             = "hide"
	PermissionLivestreamManage = "livestream.manage"
)

// ActionPermissions maps moderation actions to required permissions
var ActionPermissions = map[string]string{
	"createBlock":      PermissionBan,
	"deleteBlock":      PermissionBan,
	"createGate":       PermissionHide,
	"deleteGate":       PermissionHide,
	"updateLivestream": PermissionLivestreamManage,
}

// PermissionChecker validates moderation permissions
type PermissionChecker struct {
	model model.Model
}

// NewPermissionChecker creates a new permission checker
func NewPermissionChecker(m model.Model) *PermissionChecker {
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

// HasPermission checks if a moderator has a specific permission for a streamer
func (pc *PermissionChecker) HasPermission(ctx context.Context, moderatorDID, streamerDID, permission string) (bool, error) {
	// Streamers always have all permissions for their own content
	if moderatorDID == streamerDID {
		return true, nil
	}

	// Look up delegation record
	delegation, err := pc.model.GetModerationDelegation(ctx, streamerDID, moderatorDID)
	if err != nil {
		return false, fmt.Errorf("failed to get moderation delegation: %w", err)
	}

	if delegation == nil {
		return false, nil
	}

	// Check if delegation has expired
	if delegation.ExpirationTime != nil && time.Now().After(*delegation.ExpirationTime) {
		return false, nil
	}

	// Parse permissions JSON array
	var permissions []string
	if err := json.Unmarshal(delegation.Permissions, &permissions); err != nil {
		return false, fmt.Errorf("failed to unmarshal permissions: %w", err)
	}

	// Check if moderator has the required permission
	for _, p := range permissions {
		if p == permission {
			return true, nil
		}
	}

	return false, nil
}
