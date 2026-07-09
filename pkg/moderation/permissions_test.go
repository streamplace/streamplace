package moderation

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/api/bsky"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/placestream"
)

func TestPermissionChecker_CheckPermission_StreamerSelfModeration(t *testing.T) {
	mod := newMockModel()
	pc := NewPermissionChecker(mod)

	streamerDID := "did:plc:streamer123"

	err := pc.CheckPermission(context.Background(), streamerDID, streamerDID, "createBlock")
	require.NoError(t, err, "streamer should have permission for self-moderation")

	err = pc.CheckPermission(context.Background(), streamerDID, streamerDID, "createGate")
	require.NoError(t, err, "streamer should have permission for self-moderation")

	err = pc.CheckPermission(context.Background(), streamerDID, streamerDID, "updateLivestream")
	require.NoError(t, err, "streamer should have permission for self-moderation")
}

func TestPermissionChecker_CheckPermission_WithCorrectPermission(t *testing.T) {
	mod := newMockModel()
	pc := NewPermissionChecker(mod)

	ctx := context.Background()
	streamerDID := "did:plc:streamer123"
	moderatorDID := "did:plc:moderator456"

	mod.addPermissionView(streamerDID, moderatorDID, []string{"ban", "hide"}, nil)

	err := pc.CheckPermission(ctx, moderatorDID, streamerDID, "createBlock")
	require.NoError(t, err, "moderator with 'ban' permission should be able to createBlock")

	err = pc.CheckPermission(ctx, moderatorDID, streamerDID, "createGate")
	require.NoError(t, err, "moderator with 'hide' permission should be able to createGate")
}

func TestPermissionChecker_CheckPermission_WithWrongPermission(t *testing.T) {
	mod := newMockModel()
	pc := NewPermissionChecker(mod)

	ctx := context.Background()
	streamerDID := "did:plc:streamer123"
	moderatorDID := "did:plc:moderator456"

	mod.addPermissionView(streamerDID, moderatorDID, []string{"hide"}, nil)

	err := pc.CheckPermission(ctx, moderatorDID, streamerDID, "createBlock")
	require.Error(t, err, "moderator with only 'hide' permission should not be able to createBlock")
	require.Contains(t, err.Error(), "does not have permission 'ban'")
}

func TestPermissionChecker_CheckPermission_WithoutAnyPermission(t *testing.T) {
	mod := newMockModel()
	pc := NewPermissionChecker(mod)

	ctx := context.Background()
	streamerDID := "did:plc:streamer123"
	moderatorDID := "did:plc:moderator456"

	err := pc.CheckPermission(ctx, moderatorDID, streamerDID, "createBlock")
	require.Error(t, err, "moderator without any delegation should be denied")
	require.Contains(t, err.Error(), "does not have permission")
}

func TestPermissionChecker_CheckPermission_UnknownAction(t *testing.T) {
	mod := newMockModel()
	pc := NewPermissionChecker(mod)

	streamerDID := "did:plc:streamer123"

	err := pc.CheckPermission(context.Background(), streamerDID, streamerDID, "unknownAction")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown action")
}

func TestPermissionChecker_HasPermission(t *testing.T) {
	mod := newMockModel()
	pc := NewPermissionChecker(mod)

	ctx := context.Background()
	streamerDID := "did:plc:streamer123"
	moderatorDID := "did:plc:moderator456"

	mod.addPermissionView(streamerDID, moderatorDID, []string{"ban", "hide"}, nil)

	has, err := pc.HasPermission(ctx, moderatorDID, streamerDID, PermissionBan)
	require.NoError(t, err)
	require.True(t, has, "should have 'ban' permission")

	has, err = pc.HasPermission(ctx, moderatorDID, streamerDID, PermissionHide)
	require.NoError(t, err)
	require.True(t, has, "should have 'hide' permission")

	has, err = pc.HasPermission(ctx, moderatorDID, streamerDID, PermissionLivestreamManage)
	require.NoError(t, err)
	require.False(t, has, "should not have 'livestream.manage' permission")
}

func TestActionPermissions_Mapping(t *testing.T) {
	require.Equal(t, PermissionBan, ActionPermissions["createBlock"])
	require.Equal(t, PermissionBan, ActionPermissions["deleteBlock"])
	require.Equal(t, PermissionHide, ActionPermissions["createGate"])
	require.Equal(t, PermissionHide, ActionPermissions["deleteGate"])
	require.Equal(t, PermissionLivestreamManage, ActionPermissions["updateLivestream"])
}

func TestPermissionChecker_HasPermission_MultipleSeparateRecords(t *testing.T) {
	mod := newMockModel()
	pc := NewPermissionChecker(mod)

	ctx := context.Background()
	streamerDID := "did:plc:streamer123"
	moderatorDID := "did:plc:moderator456"

	mod.addPermissionView(streamerDID, moderatorDID, []string{"ban"}, nil)
	mod.addPermissionView(streamerDID, moderatorDID, []string{"hide"}, nil)

	hasBan, err := pc.HasPermission(ctx, moderatorDID, streamerDID, PermissionBan)
	require.NoError(t, err)
	require.True(t, hasBan, "should have 'ban' permission from first record")

	hasHide, err := pc.HasPermission(ctx, moderatorDID, streamerDID, PermissionHide)
	require.NoError(t, err)
	require.True(t, hasHide, "should have 'hide' permission from second record")

	err = pc.CheckPermission(ctx, moderatorDID, streamerDID, "createBlock")
	require.NoError(t, err, "should allow createBlock with 'ban' permission from separate record")

	err = pc.CheckPermission(ctx, moderatorDID, streamerDID, "createGate")
	require.NoError(t, err, "should allow createGate with 'hide' permission from separate record")
}

func TestPermissionChecker_HasPermission_ExpiredDelegation(t *testing.T) {
	mod := newMockModel()
	pc := NewPermissionChecker(mod)

	ctx := context.Background()
	streamerDID := "did:plc:streamer123"
	moderatorDID := "did:plc:moderator456"

	expiredTime := time.Now().Add(-1 * time.Hour)
	mod.addPermissionView(streamerDID, moderatorDID, []string{"ban"}, &expiredTime)

	hasPermission, err := pc.HasPermission(ctx, moderatorDID, streamerDID, PermissionBan)
	require.NoError(t, err)
	require.False(t, hasPermission, "should deny permission for expired delegation")

	err = pc.CheckPermission(ctx, moderatorDID, streamerDID, "createBlock")
	require.Error(t, err, "should deny action for expired delegation")
	require.Contains(t, err.Error(), "does not have permission")
}

func TestPermissionChecker_HasPermission_NotYetExpired(t *testing.T) {
	mod := newMockModel()
	pc := NewPermissionChecker(mod)

	ctx := context.Background()
	streamerDID := "did:plc:streamer123"
	moderatorDID := "did:plc:moderator456"

	futureTime := time.Now().Add(1 * time.Hour)
	mod.addPermissionView(streamerDID, moderatorDID, []string{"ban", "hide"}, &futureTime)

	hasPermission, err := pc.HasPermission(ctx, moderatorDID, streamerDID, PermissionBan)
	require.NoError(t, err)
	require.True(t, hasPermission, "should allow permission for not-yet-expired delegation")

	err = pc.CheckPermission(ctx, moderatorDID, streamerDID, "createBlock")
	require.NoError(t, err, "should allow action for not-yet-expired delegation")
}

func TestPermissionChecker_HasPermission_NoExpiration(t *testing.T) {
	mod := newMockModel()
	pc := NewPermissionChecker(mod)

	ctx := context.Background()
	streamerDID := "did:plc:streamer123"
	moderatorDID := "did:plc:moderator456"

	mod.addPermissionView(streamerDID, moderatorDID, []string{"ban", "hide"}, nil)

	hasPermission, err := pc.HasPermission(ctx, moderatorDID, streamerDID, PermissionBan)
	require.NoError(t, err)
	require.True(t, hasPermission, "should allow permission for delegation with no expiration")

	err = pc.CheckPermission(ctx, moderatorDID, streamerDID, "createBlock")
	require.NoError(t, err, "should allow action for delegation with no expiration")
}

type mockModel struct {
	delegations map[string][]*streamplace.ModerationDefs_PermissionView
}

func newMockModel() *mockModel {
	return &mockModel{
		delegations: make(map[string][]*streamplace.ModerationDefs_PermissionView),
	}
}

func (m *mockModel) addPermissionView(streamerDID, moderatorDID string, permissions []string, expirationTime *time.Time) {
	key := streamerDID + "_" + moderatorDID

	var expTimeStr *string
	if expirationTime != nil {
		str := expirationTime.Format(time.RFC3339)
		expTimeStr = &str
	}

	permRecord := &streamplace.ModerationPermission{
		Moderator:      moderatorDID,
		Permissions:    permissions,
		ExpirationTime: expTimeStr,
	}

	view := &streamplace.ModerationDefs_PermissionView{
		Uri:    fmt.Sprintf("at://%s/place.stream.moderation.permission/test", streamerDID),
		Cid:    "bafytest",
		Author: &bsky.ActorDefs_ProfileViewBasic{Did: streamerDID},
		Record: &lexutil.LexiconTypeDecoder{Val: permRecord},
	}

	m.delegations[key] = append(m.delegations[key], view)
}

func (m *mockModel) GetModerationDelegations(ctx context.Context, streamerDID, moderatorDID string) ([]*streamplace.ModerationDefs_PermissionView, error) {
	key := streamerDID + "_" + moderatorDID
	delegations, exists := m.delegations[key]
	if !exists {
		return nil, nil
	}
	return delegations, nil
}
