package spxrpc

import (
	"context"
	"strings"
	"testing"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/util"
	"github.com/bluesky-social/indigo/xrpc"
	metrics "github.com/slok/go-http-metrics/metrics/prometheus"
	"github.com/slok/go-http-metrics/middleware"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/devenv"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/streamplace"
)

// TestModerationCreateBlock_WithPermission tests that a moderator with "ban" permission can create blocks
func TestModerationCreateBlock_WithPermission(t *testing.T) {
	env := setupModerationTestEnv(t)
	defer env.Cleanup()

	// Create delegation with "ban" permission
	createDelegation(t, env, env.streamer, env.moderator, []string{"ban", "hide"})

	// Call API as moderator to ban user
	input := &streamplace.ModerationCreateBlock_Input{
		Streamer: env.streamer.DID,
		Subject:  env.user.DID,
	}
	output := &streamplace.ModerationCreateBlock_Output{}

	err := env.moderator.XRPC.Do(env.ctx, xrpc.Procedure, "application/json",
		"place.stream.moderation.createBlock", nil, input, output)
	require.NoError(t, err, "moderator with permission should be able to create block")
	require.NotEmpty(t, output.Uri, "block URI should be returned")
	require.NotEmpty(t, output.Cid, "block CID should be returned")

	// Verify block was created in streamer's repo
	blockExists := waitForCondition(t, env.ctx, func() bool {
		rkey := extractRKeyFromURI(output.Uri)
		block, err := env.model.GetBlock(env.ctx, rkey)
		if err != nil || block == nil {
			return false
		}
		return block.SubjectDID == env.user.DID
	})
	require.True(t, blockExists, "block should exist in database")

	// Verify audit log
	auditLog := getLatestAuditLog(t, env.model, env.ctx, env.streamer.DID, env.moderator.DID)
	require.NotNil(t, auditLog, "audit log should exist")
	require.Equal(t, "createBlock", auditLog.Action)
	require.True(t, auditLog.Success, "audit log should show success")
	require.Equal(t, env.user.DID, auditLog.TargetDID)
	require.Equal(t, output.Uri, auditLog.ResultURI)
}

// TestModerationCreateBlock_WithoutPermission tests that a moderator without "ban" permission is denied
func TestModerationCreateBlock_WithoutPermission(t *testing.T) {
	env := setupModerationTestEnv(t)
	defer env.Cleanup()

	// Create delegation WITHOUT "ban" permission (only "hide")
	createDelegation(t, env, env.streamer, env.moderator, []string{"hide"})

	// Attempt to call API as moderator to ban user
	input := &streamplace.ModerationCreateBlock_Input{
		Streamer: env.streamer.DID,
		Subject:  env.user.DID,
	}
	output := &streamplace.ModerationCreateBlock_Output{}

	err := env.moderator.XRPC.Do(env.ctx, xrpc.Procedure, "application/json",
		"place.stream.moderation.createBlock", nil, input, output)
	require.Error(t, err, "moderator without 'ban' permission should be denied")
	require.Contains(t, err.Error(), "403", "should return 403 Forbidden")

	// Verify audit log shows failure
	time.Sleep(100 * time.Millisecond) // Give time for audit log to be written
	auditLog := getLatestAuditLog(t, env.model, env.ctx, env.streamer.DID, env.moderator.DID)
	if auditLog != nil {
		require.Equal(t, "createBlock", auditLog.Action)
		require.False(t, auditLog.Success, "audit log should show failure")
		require.Contains(t, auditLog.ErrorMsg, "permission", "error should mention permission")
	}
}

// TestModerationCreateBlock_NoPermissionAtAll tests that a non-moderator is denied
func TestModerationCreateBlock_NoPermissionAtAll(t *testing.T) {
	env := setupModerationTestEnv(t)
	defer env.Cleanup()

	// No delegation created - moderator has no permissions

	// Attempt to call API as moderator to ban user
	input := &streamplace.ModerationCreateBlock_Input{
		Streamer: env.streamer.DID,
		Subject:  env.user.DID,
	}
	output := &streamplace.ModerationCreateBlock_Output{}

	err := env.moderator.XRPC.Do(env.ctx, xrpc.Procedure, "application/json",
		"place.stream.moderation.createBlock", nil, input, output)
	require.Error(t, err, "non-moderator should be denied")
	require.Contains(t, err.Error(), "403", "should return 403 Forbidden")
}

// TestModerationCreateBlock_StreamerSelfModeration tests that streamers can always moderate their own content
func TestModerationCreateBlock_StreamerSelfModeration(t *testing.T) {
	env := setupModerationTestEnv(t)
	defer env.Cleanup()

	// No delegation needed - streamer moderating their own content

	// Call API as streamer to ban user
	input := &streamplace.ModerationCreateBlock_Input{
		Streamer: env.streamer.DID,
		Subject:  env.user.DID,
	}
	output := &streamplace.ModerationCreateBlock_Output{}

	err := env.streamer.XRPC.Do(env.ctx, xrpc.Procedure, "application/json",
		"place.stream.moderation.createBlock", nil, input, output)
	require.NoError(t, err, "streamer should always be able to moderate their own content")
	require.NotEmpty(t, output.Uri)
}

// TestModerationCreateGate_WithPermission tests that a moderator with "hide" permission can create gates
func TestModerationCreateGate_WithPermission(t *testing.T) {
	env := setupModerationTestEnv(t)
	defer env.Cleanup()

	// Create a chat message to hide
	messageURI := createChatMessage(t, env, env.user, env.streamer.DID, "Test message")

	// Create delegation with "hide" permission
	createDelegation(t, env, env.streamer, env.moderator, []string{"hide"})

	// Call API as moderator to hide message
	input := &streamplace.ModerationCreateGate_Input{
		Streamer:   env.streamer.DID,
		MessageUri: messageURI,
	}
	output := &streamplace.ModerationCreateGate_Output{}

	err := env.moderator.XRPC.Do(env.ctx, xrpc.Procedure, "application/json",
		"place.stream.moderation.createGate", nil, input, output)
	require.NoError(t, err, "moderator with 'hide' permission should be able to create gate")
	require.NotEmpty(t, output.Uri)

	// Verify gate was created
	gateExists := waitForCondition(t, env.ctx, func() bool {
		rkey := extractRKeyFromURI(output.Uri)
		gate, err := env.model.GetGate(env.ctx, rkey)
		if err != nil || gate == nil {
			return false
		}
		return gate.HiddenMessage == messageURI
	})
	require.True(t, gateExists, "gate should exist in database")

	// Verify audit log
	auditLog := getLatestAuditLog(t, env.model, env.ctx, env.streamer.DID, env.moderator.DID)
	require.NotNil(t, auditLog, "audit log should exist")
	require.Equal(t, "createGate", auditLog.Action)
	require.True(t, auditLog.Success, "audit log should show success")
	require.Equal(t, messageURI, auditLog.TargetURI)
	require.Equal(t, output.Uri, auditLog.ResultURI)
}

// TestModerationCreateGate_WithoutPermission tests that a moderator without "hide" permission is denied
func TestModerationCreateGate_WithoutPermission(t *testing.T) {
	env := setupModerationTestEnv(t)
	defer env.Cleanup()

	// Create a chat message
	messageURI := createChatMessage(t, env, env.user, env.streamer.DID, "Test message")

	// Create delegation WITHOUT "hide" permission (only "ban")
	createDelegation(t, env, env.streamer, env.moderator, []string{"ban"})

	// Attempt to call API as moderator to hide message
	input := &streamplace.ModerationCreateGate_Input{
		Streamer:   env.streamer.DID,
		MessageUri: messageURI,
	}
	output := &streamplace.ModerationCreateGate_Output{}

	err := env.moderator.XRPC.Do(env.ctx, xrpc.Procedure, "application/json",
		"place.stream.moderation.createGate", nil, input, output)
	require.Error(t, err, "moderator without 'hide' permission should be denied")
	require.Contains(t, err.Error(), "403", "should return 403 Forbidden")
}

// TestModerationDeleteBlock tests deleting a block
func TestModerationDeleteBlock(t *testing.T) {
	env := setupModerationTestEnv(t)
	defer env.Cleanup()

	// Create delegation with "ban" permission
	createDelegation(t, env, env.streamer, env.moderator, []string{"ban"})

	// First create a block as moderator
	blockInput := &streamplace.ModerationCreateBlock_Input{
		Streamer: env.streamer.DID,
		Subject:  env.user.DID,
	}
	blockOutput := &streamplace.ModerationCreateBlock_Output{}
	err := env.moderator.XRPC.Do(env.ctx, xrpc.Procedure, "application/json",
		"place.stream.moderation.createBlock", nil, blockInput, blockOutput)
	require.NoError(t, err)

	// Wait for block to be ingested
	waitForCondition(t, env.ctx, func() bool {
		rkey := extractRKeyFromURI(blockOutput.Uri)
		block, _ := env.model.GetBlock(env.ctx, rkey)
		return block != nil
	})

	// Now delete the block
	deleteInput := &streamplace.ModerationDeleteBlock_Input{
		Streamer: env.streamer.DID,
		BlockUri: blockOutput.Uri,
	}
	deleteOutput := &streamplace.ModerationDeleteBlock_Output{}

	err = env.moderator.XRPC.Do(env.ctx, xrpc.Procedure, "application/json",
		"place.stream.moderation.deleteBlock", nil, deleteInput, deleteOutput)
	require.NoError(t, err, "moderator should be able to delete block")

	// Verify block was deleted
	blockDeleted := waitForCondition(t, env.ctx, func() bool {
		rkey := extractRKeyFromURI(blockOutput.Uri)
		block, _ := env.model.GetBlock(env.ctx, rkey)
		return block == nil
	})
	require.True(t, blockDeleted, "block should be deleted from database")

	// Verify audit log for delete operation
	auditLog := getLatestAuditLog(t, env.model, env.ctx, env.streamer.DID, env.moderator.DID)
	require.NotNil(t, auditLog, "audit log should exist")
	require.Equal(t, "deleteBlock", auditLog.Action)
	require.True(t, auditLog.Success, "audit log should show success")
	require.Equal(t, blockOutput.Uri, auditLog.TargetURI)
}

// TestModerationInvalidInput tests input validation
func TestModerationInvalidInput(t *testing.T) {
	env := setupModerationTestEnv(t)
	defer env.Cleanup()

	createDelegation(t, env, env.streamer, env.moderator, []string{"ban", "hide"})

	t.Run("invalid_streamer_did", func(t *testing.T) {
		input := &streamplace.ModerationCreateBlock_Input{
			Streamer: "not-a-valid-did",
			Subject:  env.user.DID,
		}
		output := &streamplace.ModerationCreateBlock_Output{}

		err := env.moderator.XRPC.Do(env.ctx, xrpc.Procedure, "application/json",
			"place.stream.moderation.createBlock", nil, input, output)
		require.Error(t, err)
		require.Contains(t, err.Error(), "400", "should return 400 Bad Request")
	})

	t.Run("invalid_subject_did", func(t *testing.T) {
		input := &streamplace.ModerationCreateBlock_Input{
			Streamer: env.streamer.DID,
			Subject:  "not-a-valid-did",
		}
		output := &streamplace.ModerationCreateBlock_Output{}

		err := env.moderator.XRPC.Do(env.ctx, xrpc.Procedure, "application/json",
			"place.stream.moderation.createBlock", nil, input, output)
		require.Error(t, err)
		require.Contains(t, err.Error(), "400", "should return 400 Bad Request")
	})

	t.Run("invalid_message_uri", func(t *testing.T) {
		input := &streamplace.ModerationCreateGate_Input{
			Streamer:   env.streamer.DID,
			MessageUri: "not-a-valid-uri",
		}
		output := &streamplace.ModerationCreateGate_Output{}

		err := env.moderator.XRPC.Do(env.ctx, xrpc.Procedure, "application/json",
			"place.stream.moderation.createGate", nil, input, output)
		require.Error(t, err)
		require.Contains(t, err.Error(), "400", "should return 400 Bad Request")
	})
}

// TestModerationUpdateLivestream_WithPermission tests that a moderator with "livestream.manage" permission can update livestreams
func TestModerationUpdateLivestream_WithPermission(t *testing.T) {
	env := setupModerationTestEnv(t)
	defer env.Cleanup()

	// Create a livestream record as streamer
	livestreamURI := createLivestream(t, env, env.streamer, "Original Title")

	// Create delegation with "livestream.manage" permission
	createDelegation(t, env, env.streamer, env.moderator, []string{"livestream.manage"})

	// Call API as moderator to update livestream
	newTitle := "Updated Title"
	input := &streamplace.ModerationUpdateLivestream_Input{
		Streamer:      env.streamer.DID,
		LivestreamUri: livestreamURI,
		Title:         &newTitle,
	}
	output := &streamplace.ModerationUpdateLivestream_Output{}

	err := env.moderator.XRPC.Do(env.ctx, xrpc.Procedure, "application/json",
		"place.stream.moderation.updateLivestream", nil, input, output)
	require.NoError(t, err, "moderator with 'livestream.manage' permission should be able to update livestream")
	require.NotEmpty(t, output.Uri)
	require.NotEmpty(t, output.Cid)

	// Verify audit log
	auditLog := getLatestAuditLog(t, env.model, env.ctx, env.streamer.DID, env.moderator.DID)
	require.NotNil(t, auditLog, "audit log should exist")
	require.Equal(t, "updateLivestream", auditLog.Action)
	require.True(t, auditLog.Success, "audit log should show success")
	require.Equal(t, livestreamURI, auditLog.TargetURI)
}

// TestModerationUpdateLivestream_WithoutPermission tests that a moderator without "livestream.manage" permission is denied
func TestModerationUpdateLivestream_WithoutPermission(t *testing.T) {
	env := setupModerationTestEnv(t)
	defer env.Cleanup()

	// Create a livestream record as streamer
	livestreamURI := createLivestream(t, env, env.streamer, "Original Title")

	// Create delegation WITHOUT "livestream.manage" permission (only "ban")
	createDelegation(t, env, env.streamer, env.moderator, []string{"ban"})

	// Attempt to call API as moderator to update livestream
	newTitle := "Updated Title"
	input := &streamplace.ModerationUpdateLivestream_Input{
		Streamer:      env.streamer.DID,
		LivestreamUri: livestreamURI,
		Title:         &newTitle,
	}
	output := &streamplace.ModerationUpdateLivestream_Output{}

	err := env.moderator.XRPC.Do(env.ctx, xrpc.Procedure, "application/json",
		"place.stream.moderation.updateLivestream", nil, input, output)
	require.Error(t, err, "moderator without 'livestream.manage' permission should be denied")
	require.Contains(t, err.Error(), "403", "should return 403 Forbidden")

	// Verify audit log shows failure
	time.Sleep(100 * time.Millisecond) // Give time for audit log to be written
	auditLog := getLatestAuditLog(t, env.model, env.ctx, env.streamer.DID, env.moderator.DID)
	if auditLog != nil {
		require.Equal(t, "updateLivestream", auditLog.Action)
		require.False(t, auditLog.Success, "audit log should show failure")
		require.Contains(t, auditLog.ErrorMsg, "permission", "error should mention permission")
	}
}

// TestModerationUpdateLivestream_InputValidation tests input validation
func TestModerationUpdateLivestream_InputValidation(t *testing.T) {
	env := setupModerationTestEnv(t)
	defer env.Cleanup()

	// Grant permission
	createDelegation(t, env, env.streamer, env.moderator, []string{"livestream.manage"})

	testCases := []struct {
		name          string
		livestreamUri string
		streamerDID   string
		title         *string
		expectError   bool
		errorCode     string
	}{
		{
			name:          "empty livestream URI",
			livestreamUri: "",
			streamerDID:   env.streamer.DID,
			title:         stringPtr("title"),
			expectError:   true,
			errorCode:     "400",
		},
		{
			name:          "invalid livestream URI",
			livestreamUri: "not-a-valid-uri",
			streamerDID:   env.streamer.DID,
			title:         stringPtr("title"),
			expectError:   true,
			errorCode:     "400",
		},
		{
			name:          "empty streamer DID",
			livestreamUri: "at://did:plc:xyz/place.stream.livestream/123",
			streamerDID:   "",
			title:         stringPtr("title"),
			expectError:   true,
			errorCode:     "400",
		},
		{
			name:          "invalid streamer DID",
			livestreamUri: "at://did:plc:xyz/place.stream.livestream/123",
			streamerDID:   "not-a-valid-did",
			title:         stringPtr("title"),
			expectError:   true,
			errorCode:     "400",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := &streamplace.ModerationUpdateLivestream_Input{
				Streamer:      tc.streamerDID,
				LivestreamUri: tc.livestreamUri,
				Title:         tc.title,
			}
			output := &streamplace.ModerationUpdateLivestream_Output{}

			err := env.moderator.XRPC.Do(env.ctx, xrpc.Procedure, "application/json",
				"place.stream.moderation.updateLivestream", nil, input, output)

			if tc.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errorCode, "should return "+tc.errorCode)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestModerationUpdateLivestream_StreamerSelfModeration tests that streamers can update their own livestreams
func TestModerationUpdateLivestream_StreamerSelfModeration(t *testing.T) {
	env := setupModerationTestEnv(t)
	defer env.Cleanup()

	// Create a livestream record as streamer
	livestreamURI := createLivestream(t, env, env.streamer, "Original Title")

	// No delegation needed - streamer updating their own content

	// Call API as streamer to update livestream
	newTitle := "Updated by Streamer"
	input := &streamplace.ModerationUpdateLivestream_Input{
		Streamer:      env.streamer.DID,
		LivestreamUri: livestreamURI,
		Title:         &newTitle,
	}
	output := &streamplace.ModerationUpdateLivestream_Output{}

	err := env.streamer.XRPC.Do(env.ctx, xrpc.Procedure, "application/json",
		"place.stream.moderation.updateLivestream", nil, input, output)
	require.NoError(t, err, "streamer should always be able to update their own livestream")
	require.NotEmpty(t, output.Uri)
	require.NotEmpty(t, output.Cid)
}

// Helper types and functions

// stringPtr returns a pointer to a string
func stringPtr(s string) *string {
	return &s
}

type moderationTestEnv struct {
	ctx        context.Context
	cancel     context.CancelFunc
	dev        *devenv.DevEnv
	model      model.Model
	statefulDB *statedb.StatefulDB
	server     *Server
	atsync     *atproto.ATProtoSynchronizer
	streamer   *devenv.DevEnvAccount
	moderator  *devenv.DevEnvAccount
	user       *devenv.DevEnvAccount
}

func (env *moderationTestEnv) Cleanup() {
	env.cancel()
}

func setupModerationTestEnv(t *testing.T) *moderationTestEnv {
	dev := devenv.WithDevEnv(t)

	cli := &config.CLI{
		BroadcasterHost: "example.com",
		DBURL:           ":memory:",
		RelayHost:       strings.ReplaceAll(dev.PDSURL, "http://", "ws://"),
		PLCURL:          dev.PLCURL,
		DataDir:         t.TempDir(),
	}

	b := bus.NewBus()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)

	state, err := statedb.MakeDB(context.Background(), cli, nil, mod)
	require.NoError(t, err)

	atsync := &atproto.ATProtoSynchronizer{
		CLI:        cli,
		StatefulDB: state,
		Model:      mod,
		Bus:        b,
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Start firehose in background
	go func() {
		_ = atsync.StartFirehose(ctx)
	}()

	// Create OATProxy for OAuth middleware
	op := oatproxy.New(&oatproxy.Config{
		Host:               "test.example.com",
		CreateOAuthSession: state.CreateOAuthSession,
		UpdateOAuthSession: state.UpdateOAuthSession,
		GetOAuthSession:    state.LoadOAuthSession,
		Lock:               state.GetNamedLock,
		Scope:              "atproto transition:generic",
		Public:             true,
	})

	// Create server with handlers
	mdlw := middleware.New(middleware.Config{
		Recorder: metrics.NewRecorder(metrics.Config{}),
	})
	server, err := NewServer(ctx, cli, mod, state, op, mdlw, atsync, b)
	require.NoError(t, err)

	// Create test accounts
	streamer := dev.CreateAccount(t)
	moderator := dev.CreateAccount(t)
	user := dev.CreateAccount(t)

	// Register sessions with statefulDB so OAuth middleware works
	registerSession(t, state, streamer)
	registerSession(t, state, moderator)
	registerSession(t, state, user)

	return &moderationTestEnv{
		ctx:        ctx,
		cancel:     cancel,
		dev:        dev,
		model:      mod,
		statefulDB: state,
		server:     server,
		atsync:     atsync,
		streamer:   streamer,
		moderator:  moderator,
		user:       user,
	}
}

func registerSession(t *testing.T, state *statedb.StatefulDB, account *devenv.DevEnvAccount) {
	// Store the session in statefulDB so the OAuth middleware can find it
	// This is a simplified version - in production, sessions are stored via the OAuth flow
	// TODO: The session may need additional fields (tokens, etc.) to be fully functional
	// This minimal session allows the test to compile but may need enhancement for actual OAuth flows
	session := &oatproxy.OAuthSession{
		DID: account.DID,
	}
	// Use DID as session ID for testing - in production this would be downstream_dpop_jkt
	err := state.CreateOAuthSession(account.DID, session)
	require.NoError(t, err)
}

func createDelegation(t *testing.T, env *moderationTestEnv, streamer, moderator *devenv.DevEnvAccount, permissions []string) {
	delegationRecord := &streamplace.ModerationPermission{
		LexiconTypeID: "place.stream.moderation.permission",
		Moderator:     moderator.DID,
		Permissions:   permissions,
		CreatedAt:     time.Now().Format(util.ISO8601),
	}

	_, err := comatproto.RepoCreateRecord(env.ctx, streamer.XRPC, &comatproto.RepoCreateRecord_Input{
		Collection: constants.PLACE_STREAM_MODERATION_PERMISSION,
		Repo:       streamer.DID,
		Record:     &lexutil.LexiconTypeDecoder{Val: delegationRecord},
	})
	require.NoError(t, err)

	// Wait for firehose ingestion
	waitForCondition(t, env.ctx, func() bool {
		delegation, err := env.model.GetModerationDelegation(env.ctx, streamer.DID, moderator.DID)
		return err == nil && delegation != nil
	})
}

func createChatMessage(t *testing.T, env *moderationTestEnv, author *devenv.DevEnvAccount, streamerDID, text string) string {
	msg := &streamplace.ChatMessage{
		LexiconTypeID: "place.stream.chat.message",
		Text:          text,
		CreatedAt:     time.Now().Format(util.ISO8601),
		Streamer:      streamerDID,
	}

	rec, err := comatproto.RepoCreateRecord(env.ctx, author.XRPC, &comatproto.RepoCreateRecord_Input{
		Collection: constants.PLACE_STREAM_CHAT_MESSAGE,
		Repo:       author.DID,
		Record:     &lexutil.LexiconTypeDecoder{Val: msg},
	})
	require.NoError(t, err)

	return rec.Uri
}

func createLivestream(t *testing.T, env *moderationTestEnv, streamer *devenv.DevEnvAccount, title string) string {
	livestream := &streamplace.Livestream{
		LexiconTypeID: "place.stream.livestream",
		Title:         title,
		CreatedAt:     time.Now().Format(util.ISO8601),
	}

	rec, err := comatproto.RepoCreateRecord(env.ctx, streamer.XRPC, &comatproto.RepoCreateRecord_Input{
		Collection: constants.PLACE_STREAM_LIVESTREAM,
		Repo:       streamer.DID,
		Record:     &lexutil.LexiconTypeDecoder{Val: livestream},
	})
	require.NoError(t, err)

	return rec.Uri
}

func waitForCondition(t *testing.T, ctx context.Context, condition func() bool) bool {
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return false
		case <-ticker.C:
			if condition() {
				return true
			}
		}
	}
}

func extractRKeyFromURI(uri string) string {
	// at://did:plc:xxx/collection/rkey -> rkey
	parts := strings.Split(uri, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func getLatestAuditLog(t *testing.T, m model.Model, ctx context.Context, streamerDID, moderatorDID string) *model.ModerationAuditLog {
	// Get recent audit logs for the streamer
	logs, err := m.GetAuditLogs(ctx, streamerDID, 10, nil)
	if err != nil {
		t.Fatalf("failed to get audit logs: %v", err)
	}

	// Filter by moderator DID and return the most recent
	for _, log := range logs {
		if log.ModeratorDID == moderatorDID {
			return log
		}
	}

	return nil
}
