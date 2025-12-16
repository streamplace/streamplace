package moderation

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/streamplace"
)

func TestPermissionChecker_CheckPermission_StreamerSelfModeration(t *testing.T) {
	mod := setupMockModel(t)
	pc := NewPermissionChecker(mod)

	streamerDID := "did:plc:streamer123"

	// Streamer should always have permission to moderate their own content
	err := pc.CheckPermission(context.Background(), streamerDID, streamerDID, "createBlock")
	require.NoError(t, err, "streamer should have permission for self-moderation")

	err = pc.CheckPermission(context.Background(), streamerDID, streamerDID, "createGate")
	require.NoError(t, err, "streamer should have permission for self-moderation")

	err = pc.CheckPermission(context.Background(), streamerDID, streamerDID, "updateLivestream")
	require.NoError(t, err, "streamer should have permission for self-moderation")
}

func TestPermissionChecker_CheckPermission_WithCorrectPermission(t *testing.T) {
	mod := setupMockModel(t)
	pc := NewPermissionChecker(mod)

	ctx := context.Background()
	streamerDID := "did:plc:streamer123"
	moderatorDID := "did:plc:moderator456"

	// Create delegation with "ban" and "hide" permissions
	delegation := &model.ModerationDelegation{
		RepoDID:      streamerDID,
		ModeratorDID: moderatorDID,
		Permissions:  mustMarshalJSON([]string{"ban", "hide"}),
		CreatedAt:    time.Now(),
	}
	mod.(*mockModel).addDelegation(delegation)

	// Should succeed with "ban" permission for createBlock action
	err := pc.CheckPermission(ctx, moderatorDID, streamerDID, "createBlock")
	require.NoError(t, err, "moderator with 'ban' permission should be able to createBlock")

	// Should succeed with "hide" permission for createGate action
	err = pc.CheckPermission(ctx, moderatorDID, streamerDID, "createGate")
	require.NoError(t, err, "moderator with 'hide' permission should be able to createGate")
}

func TestPermissionChecker_CheckPermission_WithWrongPermission(t *testing.T) {
	mod := setupMockModel(t)
	pc := NewPermissionChecker(mod)

	ctx := context.Background()
	streamerDID := "did:plc:streamer123"
	moderatorDID := "did:plc:moderator456"

	// Create delegation with only "hide" permission (no "ban")
	delegation := &model.ModerationDelegation{
		RepoDID:      streamerDID,
		ModeratorDID: moderatorDID,
		Permissions:  mustMarshalJSON([]string{"hide"}),
		CreatedAt:    time.Now(),
	}
	mod.(*mockModel).addDelegation(delegation)

	// Should fail - has "hide" but needs "ban" for createBlock
	err := pc.CheckPermission(ctx, moderatorDID, streamerDID, "createBlock")
	require.Error(t, err, "moderator with only 'hide' permission should not be able to createBlock")
	require.Contains(t, err.Error(), "does not have permission 'ban'")
}

func TestPermissionChecker_CheckPermission_WithoutAnyPermission(t *testing.T) {
	mod := setupMockModel(t)
	pc := NewPermissionChecker(mod)

	ctx := context.Background()
	streamerDID := "did:plc:streamer123"
	moderatorDID := "did:plc:moderator456"

	// No delegation exists

	// Should fail - no delegation
	err := pc.CheckPermission(ctx, moderatorDID, streamerDID, "createBlock")
	require.Error(t, err, "moderator without any delegation should be denied")
	require.Contains(t, err.Error(), "does not have permission")
}

func TestPermissionChecker_CheckPermission_UnknownAction(t *testing.T) {
	mod := setupMockModel(t)
	pc := NewPermissionChecker(mod)

	streamerDID := "did:plc:streamer123"

	// Should fail for unknown action
	err := pc.CheckPermission(context.Background(), streamerDID, streamerDID, "unknownAction")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown action")
}

func TestPermissionChecker_HasPermission(t *testing.T) {
	mod := setupMockModel(t)
	pc := NewPermissionChecker(mod)

	ctx := context.Background()
	streamerDID := "did:plc:streamer123"
	moderatorDID := "did:plc:moderator456"

	// Create delegation with specific permissions
	delegation := &model.ModerationDelegation{
		RepoDID:      streamerDID,
		ModeratorDID: moderatorDID,
		Permissions:  mustMarshalJSON([]string{"ban", "hide"}),
		CreatedAt:    time.Now(),
	}
	mod.(*mockModel).addDelegation(delegation)

	// Test each permission
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
	// Verify action-to-permission mappings are correct
	require.Equal(t, PermissionBan, ActionPermissions["createBlock"])
	require.Equal(t, PermissionBan, ActionPermissions["deleteBlock"])
	require.Equal(t, PermissionHide, ActionPermissions["createGate"])
	require.Equal(t, PermissionHide, ActionPermissions["deleteGate"])
	require.Equal(t, PermissionLivestreamManage, ActionPermissions["updateLivestream"])
}

func TestPermissionChecker_HasPermission_MultipleSeparateRecords(t *testing.T) {
	mod := setupMockModel(t)
	pc := NewPermissionChecker(mod)

	ctx := context.Background()
	streamerDID := "did:plc:streamer123"
	moderatorDID := "did:plc:moderator456"

	// Create TWO separate delegation records for the same moderator
	// One with "ban" permission, one with "hide" permission
	banDelegation := &model.ModerationDelegation{
		RKey:         "rkey1",
		RepoDID:      streamerDID,
		ModeratorDID: moderatorDID,
		Permissions:  mustMarshalJSON([]string{"ban"}),
		CreatedAt:    time.Now(),
	}
	mod.(*mockModel).addDelegation(banDelegation)

	hideDelegation := &model.ModerationDelegation{
		RKey:         "rkey2",
		RepoDID:      streamerDID,
		ModeratorDID: moderatorDID,
		Permissions:  mustMarshalJSON([]string{"hide"}),
		CreatedAt:    time.Now().Add(1 * time.Second), // Created slightly later
	}
	mod.(*mockModel).addDelegation(hideDelegation)

	// Should have BOTH permissions from the separate records
	hasBan, err := pc.HasPermission(ctx, moderatorDID, streamerDID, PermissionBan)
	require.NoError(t, err)
	require.True(t, hasBan, "should have 'ban' permission from first record")

	hasHide, err := pc.HasPermission(ctx, moderatorDID, streamerDID, PermissionHide)
	require.NoError(t, err)
	require.True(t, hasHide, "should have 'hide' permission from second record")

	// CheckPermission should succeed for both actions
	err = pc.CheckPermission(ctx, moderatorDID, streamerDID, "createBlock")
	require.NoError(t, err, "should allow createBlock with 'ban' permission from separate record")

	err = pc.CheckPermission(ctx, moderatorDID, streamerDID, "createGate")
	require.NoError(t, err, "should allow createGate with 'hide' permission from separate record")
}

func TestPermissionChecker_HasPermission_ExpiredDelegation(t *testing.T) {
	mod := setupMockModel(t)
	pc := NewPermissionChecker(mod)

	ctx := context.Background()
	streamerDID := "did:plc:streamer123"
	moderatorDID := "did:plc:moderator456"

	// Create delegation with expiration time in the past
	expiredTime := time.Now().Add(-1 * time.Hour)
	delegation := &model.ModerationDelegation{
		RepoDID:        streamerDID,
		ModeratorDID:   moderatorDID,
		Permissions:    mustMarshalJSON([]string{"ban"}),
		CreatedAt:      time.Now().Add(-2 * time.Hour),
		ExpirationTime: &expiredTime,
	}
	mod.(*mockModel).addDelegation(delegation)

	// Should return false for expired delegation
	hasPermission, err := pc.HasPermission(ctx, moderatorDID, streamerDID, PermissionBan)
	require.NoError(t, err)
	require.False(t, hasPermission, "should deny permission for expired delegation")

	// CheckPermission should also fail
	err = pc.CheckPermission(ctx, moderatorDID, streamerDID, "createBlock")
	require.Error(t, err, "should deny action for expired delegation")
	require.Contains(t, err.Error(), "does not have permission")
}

func TestPermissionChecker_HasPermission_NotYetExpired(t *testing.T) {
	mod := setupMockModel(t)
	pc := NewPermissionChecker(mod)

	ctx := context.Background()
	streamerDID := "did:plc:streamer123"
	moderatorDID := "did:plc:moderator456"

	// Create delegation with future expiration time (valid for 1 hour)
	futureTime := time.Now().Add(1 * time.Hour)
	delegation := &model.ModerationDelegation{
		RepoDID:        streamerDID,
		ModeratorDID:   moderatorDID,
		Permissions:    mustMarshalJSON([]string{"ban", "hide"}),
		CreatedAt:      time.Now(),
		ExpirationTime: &futureTime,
	}
	mod.(*mockModel).addDelegation(delegation)

	// Should return true for not-yet-expired delegation
	hasPermission, err := pc.HasPermission(ctx, moderatorDID, streamerDID, PermissionBan)
	require.NoError(t, err)
	require.True(t, hasPermission, "should allow permission for not-yet-expired delegation")

	// CheckPermission should succeed
	err = pc.CheckPermission(ctx, moderatorDID, streamerDID, "createBlock")
	require.NoError(t, err, "should allow action for not-yet-expired delegation")
}

func TestPermissionChecker_HasPermission_NoExpiration(t *testing.T) {
	mod := setupMockModel(t)
	pc := NewPermissionChecker(mod)

	ctx := context.Background()
	streamerDID := "did:plc:streamer123"
	moderatorDID := "did:plc:moderator456"

	// Create delegation with nil expiration time (never expires)
	delegation := &model.ModerationDelegation{
		RepoDID:        streamerDID,
		ModeratorDID:   moderatorDID,
		Permissions:    mustMarshalJSON([]string{"ban", "hide"}),
		CreatedAt:      time.Now(),
		ExpirationTime: nil, // Never expires
	}
	mod.(*mockModel).addDelegation(delegation)

	// Should return true for delegation with no expiration
	hasPermission, err := pc.HasPermission(ctx, moderatorDID, streamerDID, PermissionBan)
	require.NoError(t, err)
	require.True(t, hasPermission, "should allow permission for delegation with no expiration")

	// CheckPermission should succeed
	err = pc.CheckPermission(ctx, moderatorDID, streamerDID, "createBlock")
	require.NoError(t, err, "should allow action for delegation with no expiration")
}

// Mock implementation for testing

type mockModel struct {
	mockModelStubs
	// delegations stores multiple delegation records per streamer+moderator pair
	// Key format: "streamerDID_moderatorDID", value is a slice of delegations
	delegations map[string][]*model.ModerationDelegation
}

func setupMockModel(t *testing.T) model.Model {
	return &mockModel{
		delegations: make(map[string][]*model.ModerationDelegation),
	}
}

// addDelegation is a helper to add a delegation to the mock
func (m *mockModel) addDelegation(delegation *model.ModerationDelegation) {
	key := delegation.RepoDID + "_" + delegation.ModeratorDID
	m.delegations[key] = append(m.delegations[key], delegation)
}

func (m *mockModel) GetModerationDelegation(ctx context.Context, streamerDID, moderatorDID string) (*model.ModerationDelegation, error) {
	key := streamerDID + "_" + moderatorDID
	delegations, exists := m.delegations[key]
	if !exists || len(delegations) == 0 {
		return nil, nil
	}
	// Return the first one (for backward compatibility)
	return delegations[0], nil
}

func (m *mockModel) GetModerationDelegations(ctx context.Context, streamerDID, moderatorDID string) ([]*model.ModerationDelegation, error) {
	key := streamerDID + "_" + moderatorDID
	delegations, exists := m.delegations[key]
	if !exists {
		return nil, nil
	}
	return delegations, nil
}

func mustMarshalJSON(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

// Stub out all other required model.Model methods (not used in permission tests)
// Using interface embedding to avoid having to list all ~60 methods
type mockModelStubs struct{}

func (m *mockModelStubs) CreatePlayerEvent(event model.PlayerEventAPI) error { return nil }
func (m *mockModelStubs) ListPlayerEvents(playerID string) ([]model.PlayerEvent, error) {
	return nil, nil
}
func (m *mockModelStubs) PlayerReport(playerID string) (map[string]any, error) { return nil, nil }
func (m *mockModelStubs) ClearPlayerEvents() error                             { return nil }
func (m *mockModelStubs) CreateSegment(segment *model.Segment) error           { return nil }
func (m *mockModelStubs) MostRecentSegments() ([]model.Segment, error)         { return nil, nil }
func (m *mockModelStubs) LatestSegmentForUser(user string) (*model.Segment, error) {
	return nil, nil
}
func (m *mockModelStubs) LatestSegmentsForUser(user string, limit int, before *time.Time, after *time.Time) ([]model.Segment, error) {
	return nil, nil
}
func (m *mockModelStubs) CreateThumbnail(thumb *model.Thumbnail) error { return nil }
func (m *mockModelStubs) LatestThumbnailForUser(user string) (*model.Thumbnail, error) {
	return nil, nil
}
func (m *mockModelStubs) GetSegment(id string) (*model.Segment, error) { return nil, nil }
func (m *mockModelStubs) GetExpiredSegments(ctx context.Context) ([]model.Segment, error) {
	return nil, nil
}
func (m *mockModelStubs) DeleteSegment(ctx context.Context, id string) error { return nil }
func (m *mockModelStubs) StartSegmentCleaner(ctx context.Context) error      { return nil }
func (m *mockModelStubs) SegmentCleaner(ctx context.Context) error           { return nil }
func (m *mockModelStubs) GetIdentity(id string) (*model.Identity, error)     { return nil, nil }
func (m *mockModelStubs) UpdateIdentity(ident *model.Identity) error         { return nil }
func (m *mockModelStubs) GetRepo(did string) (*model.Repo, error)            { return nil, nil }
func (m *mockModelStubs) GetRepoByHandle(handle string) (*model.Repo, error) { return nil, nil }
func (m *mockModelStubs) GetRepoByHandleOrDID(arg string) (*model.Repo, error) {
	return nil, nil
}
func (m *mockModelStubs) GetRepoBySigningKey(signingKey string) (*model.Repo, error) {
	return nil, nil
}
func (m *mockModelStubs) GetAllRepos() ([]model.Repo, error)           { return nil, nil }
func (m *mockModelStubs) UpdateRepo(repo *model.Repo) error            { return nil }
func (m *mockModelStubs) UpdateSigningKey(key *model.SigningKey) error { return nil }
func (m *mockModelStubs) GetSigningKey(ctx context.Context, did, repoDID string) (*model.SigningKey, error) {
	return nil, nil
}
func (m *mockModelStubs) GetSigningKeyByRKey(ctx context.Context, rkey string) (*model.SigningKey, error) {
	return nil, nil
}
func (m *mockModelStubs) GetSigningKeysForRepo(repoDID string) ([]model.SigningKey, error) {
	return nil, nil
}
func (m *mockModelStubs) CreateFollow(ctx context.Context, userDID, rev string, follow *bsky.GraphFollow) error {
	return nil
}
func (m *mockModelStubs) GetUserFollowing(ctx context.Context, userDID string) ([]model.Follow, error) {
	return nil, nil
}
func (m *mockModelStubs) GetUserFollowers(ctx context.Context, userDID string) ([]model.Follow, error) {
	return nil, nil
}
func (m *mockModelStubs) GetUserFollowingUser(ctx context.Context, userDID, subjectDID string) (*model.Follow, error) {
	return nil, nil
}
func (m *mockModelStubs) DeleteFollow(ctx context.Context, userDID, rev string) error { return nil }
func (m *mockModelStubs) CreateFeedPost(ctx context.Context, post *model.FeedPost) error {
	return nil
}
func (m *mockModelStubs) ListFeedPosts() ([]model.FeedPost, error) { return nil, nil }
func (m *mockModelStubs) ListFeedPostsByType(feedType string, limit int, after int64) ([]model.FeedPost, error) {
	return nil, nil
}
func (m *mockModelStubs) GetFeedPost(uri string) (*model.FeedPost, error) { return nil, nil }
func (m *mockModelStubs) GetReplies(repoDID string) ([]*bsky.FeedDefs_PostView, error) {
	return nil, nil
}
func (m *mockModelStubs) CreateLivestream(ctx context.Context, ls *model.Livestream) error {
	return nil
}
func (m *mockModelStubs) GetLatestLivestreamForRepo(repoDID string) (*model.Livestream, error) {
	return nil, nil
}
func (m *mockModelStubs) GetLivestreamByPostURI(postURI string) (*model.Livestream, error) {
	return nil, nil
}
func (m *mockModelStubs) GetLatestLivestreams(limit int, before *time.Time) ([]model.Livestream, error) {
	return nil, nil
}
func (m *mockModelStubs) CreateBlock(ctx context.Context, block *model.Block) error { return nil }
func (m *mockModelStubs) GetBlock(ctx context.Context, rkey string) (*model.Block, error) {
	return nil, nil
}
func (m *mockModelStubs) GetUserBlock(ctx context.Context, userDID, subjectDID string) (*model.Block, error) {
	return nil, nil
}
func (m *mockModelStubs) DeleteBlock(ctx context.Context, rkey string) error { return nil }
func (m *mockModelStubs) CreateChatMessage(ctx context.Context, message *model.ChatMessage) error {
	return nil
}
func (m *mockModelStubs) MostRecentChatMessages(repoDID string) ([]*streamplace.ChatDefs_MessageView, error) {
	return nil, nil
}
func (m *mockModelStubs) GetChatMessage(uri string) (*model.ChatMessage, error) { return nil, nil }
func (m *mockModelStubs) DeleteChatMessage(ctx context.Context, uri string, deletedAt *time.Time) error {
	return nil
}
func (m *mockModelStubs) CreateGate(ctx context.Context, gate *model.Gate) error { return nil }
func (m *mockModelStubs) DeleteGate(ctx context.Context, rkey string) error      { return nil }
func (m *mockModelStubs) GetGate(ctx context.Context, rkey string) (*model.Gate, error) {
	return nil, nil
}
func (m *mockModelStubs) GetUserGates(ctx context.Context, userDID string) ([]*model.Gate, error) {
	return nil, nil
}
func (m *mockModelStubs) CreateChatProfile(ctx context.Context, profile *model.ChatProfile) error {
	return nil
}
func (m *mockModelStubs) GetChatProfile(ctx context.Context, repoDID string) (*model.ChatProfile, error) {
	return nil, nil
}
func (m *mockModelStubs) UpdateServerSettings(ctx context.Context, settings *model.ServerSettings) error {
	return nil
}
func (m *mockModelStubs) GetServerSettings(ctx context.Context, server string, repoDID string) (*model.ServerSettings, error) {
	return nil, nil
}
func (m *mockModelStubs) DeleteServerSettings(ctx context.Context, server string, repoDID string) error {
	return nil
}
func (m *mockModelStubs) CreateLabeler(did string) (*model.Labeler, error)   { return nil, nil }
func (m *mockModelStubs) GetLabeler(did string) (*model.Labeler, error)      { return nil, nil }
func (m *mockModelStubs) UpdateLabelerCursor(did string, cursor int64) error { return nil }
func (m *mockModelStubs) CreateLabel(label *model.Label) error               { return nil }
func (m *mockModelStubs) GetActiveLabels(uri string) ([]*comatproto.LabelDefs_Label, error) {
	return nil, nil
}
func (m *mockModelStubs) UpdateBroadcastOrigin(ctx context.Context, origin *streamplace.BroadcastOrigin, aturi syntax.ATURI) error {
	return nil
}
func (m *mockModelStubs) GetRecentBroadcastOrigins(ctx context.Context) ([]*streamplace.BroadcastDefs_BroadcastOriginView, error) {
	return nil, nil
}
func (m *mockModelStubs) CreateMetadataConfiguration(ctx context.Context, metadata *model.MetadataConfiguration) error {
	return nil
}
func (m *mockModelStubs) GetMetadataConfiguration(ctx context.Context, repoDID string) (*model.MetadataConfiguration, error) {
	return nil, nil
}
func (m *mockModelStubs) DeleteMetadataConfiguration(ctx context.Context, repoDID string) error {
	return nil
}
func (m *mockModelStubs) CreateModerationDelegation(ctx context.Context, delegation *model.ModerationDelegation) error {
	return nil
}
func (m *mockModelStubs) DeleteModerationDelegation(ctx context.Context, rkey string) error {
	return nil
}
func (m *mockModelStubs) GetModerationDelegations(ctx context.Context, streamerDID, moderatorDID string) ([]*model.ModerationDelegation, error) {
	return nil, nil
}
func (m *mockModelStubs) GetModeratorDelegations(ctx context.Context, moderatorDID string) ([]*model.ModerationDelegation, error) {
	return nil, nil
}
func (m *mockModelStubs) GetStreamerModerators(ctx context.Context, streamerDID string) ([]*model.ModerationDelegation, error) {
	return nil, nil
}
func (m *mockModelStubs) CreateAuditLog(ctx context.Context, log *model.ModerationAuditLog) error {
	return nil
}
func (m *mockModelStubs) GetAuditLogs(ctx context.Context, streamerDID string, limit int, before *time.Time) ([]*model.ModerationAuditLog, error) {
	return nil, nil
}
func (m *mockModelStubs) GetModeratorAuditLogs(ctx context.Context, moderatorDID string, limit int, before *time.Time) ([]*model.ModerationAuditLog, error) {
	return nil, nil
}
