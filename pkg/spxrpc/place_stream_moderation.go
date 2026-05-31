package spxrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/atproto/syntax"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/streamplace"
)

// handlePlaceStreamModerationCreateBlock creates a block (ban) on behalf of a streamer
func (s *Server) handlePlaceStreamModerationCreateBlock(ctx context.Context, input *streamplace.ModerationCreateBlock_Input) (*streamplace.ModerationCreateBlock_Output, error) {
	// Validate input
	if err := validateDID(input.Streamer); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid streamer DID: %v", err))
	}
	if err := validateDID(input.Subject); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid subject DID: %v", err))
	}

	// Get delegated moderation context (validates OAuth, permission, and returns client)
	modCtx, err := s.GetDelegatedModerationContext(ctx, input.Streamer, "createBlock")
	if err != nil {
		return nil, err
	}

	// Create block record in streamer's repo
	block := &bsky.GraphBlock{
		Subject:   input.Subject,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	createInput := comatproto.RepoCreateRecord_Input{
		Collection: constants.APP_BSKY_GRAPH_BLOCK,
		Record:     &lexutil.LexiconTypeDecoder{Val: block},
		Repo:       input.Streamer,
	}
	createOutput := comatproto.RepoCreateRecord_Output{}

	err = modCtx.StreamerClient.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.createRecord", map[string]any{}, createInput, &createOutput)
	if err != nil {
		log.Error(ctx, "failed to create block record", "err", err)
		if auditErr := s.logAudit(ctx, input.Streamer, modCtx.ModeratorDID, "createBlock", "", input.Subject, "", false, err.Error()); auditErr != nil {
			log.Error(ctx, "failed to create audit log", "error", auditErr)
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to create block: %v", err))
	}

	// Log successful audit entry
	if err := s.logAudit(ctx, input.Streamer, modCtx.ModeratorDID, "createBlock", "", input.Subject, createOutput.Uri, true, ""); err != nil {
		log.Error(ctx, "failed to create audit log", "error", err)
	}

	return &streamplace.ModerationCreateBlock_Output{
		Uri: createOutput.Uri,
		Cid: createOutput.Cid,
	}, nil
}

// handlePlaceStreamModerationDeleteBlock deletes a block (unban) on behalf of a streamer
func (s *Server) handlePlaceStreamModerationDeleteBlock(ctx context.Context, input *streamplace.ModerationDeleteBlock_Input) (*streamplace.ModerationDeleteBlock_Output, error) {
	// Validate input
	if err := validateDID(input.Streamer); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid streamer DID: %v", err))
	}
	if err := validateATURI(input.BlockUri); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid block URI: %v", err))
	}

	// Get delegated moderation context (validates OAuth, permission, and returns client)
	modCtx, err := s.GetDelegatedModerationContext(ctx, input.Streamer, "deleteBlock")
	if err != nil {
		return nil, err
	}

	// Parse blockUri to extract rkey
	// AT-URI format: at://did:plc:xxx/collection/rkey
	rkey, err := extractRKey(input.BlockUri)
	if err != nil {
		log.Error(ctx, "failed to extract rkey from blockUri", "uri", input.BlockUri, "err", err)
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid blockUri format")
	}

	// Delete block record from streamer's repo
	deleteInput := comatproto.RepoDeleteRecord_Input{
		Collection: constants.APP_BSKY_GRAPH_BLOCK,
		Rkey:       rkey,
		Repo:       input.Streamer,
	}
	deleteOutput := comatproto.RepoDeleteRecord_Output{}

	err = modCtx.StreamerClient.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.deleteRecord", map[string]any{}, deleteInput, &deleteOutput)
	if err != nil {
		log.Error(ctx, "failed to delete block record", "err", err)
		if auditErr := s.logAudit(ctx, input.Streamer, modCtx.ModeratorDID, "deleteBlock", input.BlockUri, "", "", false, err.Error()); auditErr != nil {
			log.Error(ctx, "failed to create audit log", "error", auditErr)
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to delete block: %v", err))
	}

	// Log successful audit entry
	if err := s.logAudit(ctx, input.Streamer, modCtx.ModeratorDID, "deleteBlock", input.BlockUri, "", "", true, ""); err != nil {
		log.Error(ctx, "failed to create audit log", "error", err)
	}

	return &streamplace.ModerationDeleteBlock_Output{}, nil
}

// handlePlaceStreamModerationCreateGate creates a gate (hide message) on behalf of a streamer
func (s *Server) handlePlaceStreamModerationCreateGate(ctx context.Context, input *streamplace.ModerationCreateGate_Input) (*streamplace.ModerationCreateGate_Output, error) {
	// Validate input
	if err := validateDID(input.Streamer); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid streamer DID: %v", err))
	}
	if err := validateATURI(input.MessageUri); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid message URI: %v", err))
	}

	// Get delegated moderation context (validates OAuth, permission, and returns client)
	modCtx, err := s.GetDelegatedModerationContext(ctx, input.Streamer, "createGate")
	if err != nil {
		return nil, err
	}

	// Create gate record in streamer's repo
	gate := &streamplace.ChatGate{
		HiddenMessage: input.MessageUri,
	}

	createInput := comatproto.RepoCreateRecord_Input{
		Collection: constants.PLACE_STREAM_CHAT_GATE,
		Record:     &lexutil.LexiconTypeDecoder{Val: gate},
		Repo:       input.Streamer,
	}
	createOutput := comatproto.RepoCreateRecord_Output{}

	err = modCtx.StreamerClient.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.createRecord", map[string]any{}, createInput, &createOutput)
	if err != nil {
		log.Error(ctx, "failed to create gate record", "err", err)
		if auditErr := s.logAudit(ctx, input.Streamer, modCtx.ModeratorDID, "createGate", input.MessageUri, "", "", false, err.Error()); auditErr != nil {
			log.Error(ctx, "failed to create audit log", "error", auditErr)
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to create gate: %v", err))
	}

	// Log successful audit entry
	if err := s.logAudit(ctx, input.Streamer, modCtx.ModeratorDID, "createGate", input.MessageUri, "", createOutput.Uri, true, ""); err != nil {
		log.Error(ctx, "failed to create audit log", "error", err)
	}

	return &streamplace.ModerationCreateGate_Output{
		Uri: createOutput.Uri,
		Cid: createOutput.Cid,
	}, nil
}

// handlePlaceStreamModerationDeleteGate deletes a gate (unhide message) on behalf of a streamer
func (s *Server) handlePlaceStreamModerationDeleteGate(ctx context.Context, input *streamplace.ModerationDeleteGate_Input) (*streamplace.ModerationDeleteGate_Output, error) {
	// Validate input
	if err := validateDID(input.Streamer); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid streamer DID: %v", err))
	}
	if err := validateATURI(input.GateUri); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid gate URI: %v", err))
	}

	// Get delegated moderation context (validates OAuth, permission, and returns client)
	modCtx, err := s.GetDelegatedModerationContext(ctx, input.Streamer, "deleteGate")
	if err != nil {
		return nil, err
	}

	// Parse gateUri to extract rkey
	rkey, err := extractRKey(input.GateUri)
	if err != nil {
		log.Error(ctx, "failed to extract rkey from gateUri", "uri", input.GateUri, "err", err)
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid gateUri format")
	}

	// Delete gate record from streamer's repo
	deleteInput := comatproto.RepoDeleteRecord_Input{
		Collection: constants.PLACE_STREAM_CHAT_GATE,
		Rkey:       rkey,
		Repo:       input.Streamer,
	}
	deleteOutput := comatproto.RepoDeleteRecord_Output{}

	err = modCtx.StreamerClient.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.deleteRecord", map[string]any{}, deleteInput, &deleteOutput)
	if err != nil {
		log.Error(ctx, "failed to delete gate record", "err", err)
		if auditErr := s.logAudit(ctx, input.Streamer, modCtx.ModeratorDID, "deleteGate", input.GateUri, "", "", false, err.Error()); auditErr != nil {
			log.Error(ctx, "failed to create audit log", "error", auditErr)
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to delete gate: %v", err))
	}

	// Log successful audit entry
	if err := s.logAudit(ctx, input.Streamer, modCtx.ModeratorDID, "deleteGate", input.GateUri, "", "", true, ""); err != nil {
		log.Error(ctx, "failed to create audit log", "error", err)
	}

	return &streamplace.ModerationDeleteGate_Output{}, nil
}

// handlePlaceStreamModerationUpdateLivestream updates livestream metadata on behalf of a streamer
func (s *Server) handlePlaceStreamModerationUpdateLivestream(ctx context.Context, input *streamplace.ModerationUpdateLivestream_Input) (*streamplace.ModerationUpdateLivestream_Output, error) {
	// Validate input
	if err := validateDID(input.Streamer); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid streamer DID: %v", err))
	}
	if err := validateATURI(input.LivestreamUri); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid livestream URI: %v", err))
	}

	// Get delegated moderation context (validates OAuth, permission, and returns client)
	modCtx, err := s.GetDelegatedModerationContext(ctx, input.Streamer, "updateLivestream")
	if err != nil {
		return nil, err
	}

	// Parse livestreamUri to extract rkey
	rkey, err := extractRKey(input.LivestreamUri)
	if err != nil {
		log.Error(ctx, "failed to extract rkey from livestreamUri", "uri", input.LivestreamUri, "err", err)
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid livestreamUri format")
	}

	// Get existing livestream record
	getInput := map[string]any{
		"repo":       input.Streamer,
		"collection": constants.PLACE_STREAM_LIVESTREAM,
		"rkey":       rkey,
	}
	getOutput := comatproto.RepoGetRecord_Output{}
	err = modCtx.StreamerClient.Do(ctx, xrpc.Query, "application/json", "com.atproto.repo.getRecord", getInput, nil, &getOutput)
	if err != nil {
		log.Error(ctx, "failed to get livestream record", "err", err)
		if auditErr := s.logAudit(ctx, input.Streamer, modCtx.ModeratorDID, "updateLivestream", input.LivestreamUri, "", "", false, fmt.Sprintf("failed to get record: %v", err)); auditErr != nil {
			log.Error(ctx, "failed to create audit log", "error", auditErr)
		}
		return nil, echo.NewHTTPError(http.StatusNotFound, "livestream record not found")
	}

	// Decode existing record
	if getOutput.Value == nil || getOutput.Value.Val == nil {
		log.Error(ctx, "livestream record value is nil")
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to decode livestream record")
	}

	// Convert the decoded value to our struct
	livestream := &streamplace.Livestream{}
	recordBytes, err := json.Marshal(getOutput.Value.Val)
	if err != nil {
		log.Error(ctx, "failed to marshal livestream record", "err", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to decode livestream record")
	}
	err = json.Unmarshal(recordBytes, livestream)
	if err != nil {
		log.Error(ctx, "failed to unmarshal livestream record", "err", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to decode livestream record")
	}

	// Create new record (don't edit existing - old records serve as "chapter markers")
	// Copy fields from existing record and update title
	if input.Title != nil {
		livestream.Title = *input.Title
	}

	// Ensure notificationSettings.pushNotification is false for mods
	if livestream.NotificationSettings == nil {
		livestream.NotificationSettings = &streamplace.Livestream_NotificationSettings{}
	}
	pushNotificationFalse := false
	livestream.NotificationSettings.PushNotification = &pushNotificationFalse

	// Update createdAt to current time for new record
	livestream.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	// Create new record instead of updating existing
	createInput := comatproto.RepoCreateRecord_Input{
		Collection: constants.PLACE_STREAM_LIVESTREAM,
		Record:     &lexutil.LexiconTypeDecoder{Val: livestream},
		Repo:       input.Streamer,
	}
	createOutput := comatproto.RepoCreateRecord_Output{}

	err = modCtx.StreamerClient.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.createRecord", map[string]any{}, createInput, &createOutput)
	if err != nil {
		log.Error(ctx, "failed to create livestream record", "err", err)
		if auditErr := s.logAudit(ctx, input.Streamer, modCtx.ModeratorDID, "updateLivestream", input.LivestreamUri, "", "", false, err.Error()); auditErr != nil {
			log.Error(ctx, "failed to create audit log", "error", auditErr)
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to create livestream: %v", err))
	}

	// Log successful audit entry
	if err := s.logAudit(ctx, input.Streamer, modCtx.ModeratorDID, "updateLivestream", input.LivestreamUri, "", createOutput.Uri, true, ""); err != nil {
		log.Error(ctx, "failed to create audit log", "error", err)
	}

	return &streamplace.ModerationUpdateLivestream_Output{
		Uri: createOutput.Uri,
		Cid: createOutput.Cid,
	}, nil
}

// Helper functions

// extractRKey extracts the rkey from an AT-URI (at://did:plc:xxx/collection/rkey)
func extractRKey(uri string) (string, error) {
	aturi, err := syntax.ParseATURI(uri)
	if err != nil {
		return "", fmt.Errorf("invalid AT-URI: %w", err)
	}
	return aturi.RecordKey().String(), nil
}

// validateDID checks if string is a valid AT Protocol DID format
func validateDID(did string) error {
	_, err := syntax.ParseDID(did)
	if err != nil {
		return fmt.Errorf("invalid DID format: %w", err)
	}
	return nil
}

// validateATURI checks if string is a valid AT-URI format
func validateATURI(uri string) error {
	_, err := syntax.ParseATURI(uri)
	if err != nil {
		return fmt.Errorf("invalid AT-URI format: %w", err)
	}
	return nil
}

// logAudit logs a moderation action to the audit log
func (s *Server) logAudit(ctx context.Context, streamerDID, moderatorDID, action, targetURI, targetDID, resultURI string, success bool, errorMsg string) error {
	auditLog := &statedb.ModerationAuditLog{
		StreamerDID:  streamerDID,
		ModeratorDID: moderatorDID,
		Action:       action,
		TargetURI:    targetURI,
		TargetDID:    targetDID,
		ResultURI:    resultURI,
		Success:      success,
		ErrorMsg:     errorMsg,
		CreatedAt:    time.Now(),
	}

	return s.statefulDB.CreateAuditLog(ctx, auditLog)
}

func (s *Server) handlePlaceStreamModerationCreatePin(ctx context.Context, input *streamplace.ModerationCreatePin_Input) (*streamplace.ModerationCreatePin_Output, error) {
	// Validate input
	if err := validateDID(input.Streamer); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid streamer DID: %v", err))
	}
	if err := validateATURI(input.MessageUri); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid messageUri: %v", err))
	}

	// Get delegated moderation context (validates OAuth, permission, and returns client)
	modCtx, err := s.GetDelegatedModerationContext(ctx, input.Streamer, "createPin")
	if err != nil {
		return nil, err
	}

	// Create the pinned record (old pins persist as history)
	pinnedRecord := &streamplace.ChatPinnedRecord{
		LexiconTypeID: "place.stream.chat.pinnedRecord",
		PinnedMessage: input.MessageUri,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:     input.ExpiresAt,
	}

	createInput := comatproto.RepoCreateRecord_Input{
		Collection: constants.PLACE_STREAM_CHAT_PINNED_RECORD,
		Record:     &lexutil.LexiconTypeDecoder{Val: pinnedRecord},
		Repo:       input.Streamer,
	}
	createOutput := comatproto.RepoCreateRecord_Output{}

	err = modCtx.StreamerClient.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.createRecord", map[string]any{}, createInput, &createOutput)
	if err != nil {
		log.Error(ctx, "failed to create pinned record", "err", err)
		if auditErr := s.logAudit(ctx, input.Streamer, modCtx.ModeratorDID, "createPin", input.MessageUri, "", "", false, err.Error()); auditErr != nil {
			log.Error(ctx, "failed to create audit log", "error", auditErr)
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to create pinned record: %v", err))
	}

	// Log successful audit entry
	if err := s.logAudit(ctx, input.Streamer, modCtx.ModeratorDID, "createPin", input.MessageUri, "", createOutput.Uri, true, ""); err != nil {
		log.Error(ctx, "failed to create audit log", "error", err)
	}

	return &streamplace.ModerationCreatePin_Output{
		Uri: createOutput.Uri,
		Cid: createOutput.Cid,
	}, nil
}

func (s *Server) handlePlaceStreamModerationDeletePin(ctx context.Context, input *streamplace.ModerationDeletePin_Input) (*streamplace.ModerationDeletePin_Output, error) {
	// Validate input
	if err := validateDID(input.Streamer); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid streamer DID: %v", err))
	}
	if err := validateATURI(input.PinUri); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid pinUri: %v", err))
	}

	// Get delegated moderation context (validates OAuth, permission, and returns client)
	modCtx, err := s.GetDelegatedModerationContext(ctx, input.Streamer, "deletePin")
	if err != nil {
		return nil, err
	}

	// Parse pinUri to extract rkey
	rkey, err := extractRKey(input.PinUri)
	if err != nil {
		log.Error(ctx, "failed to extract rkey from pinUri", "uri", input.PinUri, "err", err)
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid pinUri format")
	}

	// Delete pinned record from streamer's repo
	deleteInput := comatproto.RepoDeleteRecord_Input{
		Collection: constants.PLACE_STREAM_CHAT_PINNED_RECORD,
		Rkey:       rkey,
		Repo:       input.Streamer,
	}
	deleteOutput := comatproto.RepoDeleteRecord_Output{}

	err = modCtx.StreamerClient.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.deleteRecord", map[string]any{}, deleteInput, &deleteOutput)
	if err != nil {
		log.Error(ctx, "failed to delete pinned record", "err", err)
		if auditErr := s.logAudit(ctx, input.Streamer, modCtx.ModeratorDID, "deletePin", input.PinUri, "", "", false, err.Error()); auditErr != nil {
			log.Error(ctx, "failed to create audit log", "error", auditErr)
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to delete pinned record: %v", err))
	}

	// Log successful audit entry
	if err := s.logAudit(ctx, input.Streamer, modCtx.ModeratorDID, "deletePin", input.PinUri, "", "", true, ""); err != nil {
		log.Error(ctx, "failed to create audit log", "error", err)
	}

	return &streamplace.ModerationDeletePin_Output{}, nil
}
