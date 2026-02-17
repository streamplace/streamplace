package spxrpc

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	appbskytypes "github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/streamplace"
)

func (s *Server) handlePlaceStreamModerationGetAuditLog(ctx context.Context, action string, cursor string, limit int, moderator string) (*streamplace.ModerationGetAuditLog_Output, error) {
	// Get authenticated user - only the streamer can view their audit logs
	session, client := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	streamerDID := session.DID

	// Set default and validate limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	// Parse cursor (RFC3339 timestamp)
	var before *time.Time
	if cursor != "" {
		t, err := time.Parse(time.RFC3339Nano, cursor)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid cursor format, expected RFC3339 timestamp")
		}
		before = &t
	}

	// Prepare optional filters
	var actionFilter *string
	if action != "" {
		actionFilter = &action
	}
	var moderatorFilter *string
	if moderator != "" {
		moderatorFilter = &moderator
	}

	// Fetch audit logs with filters (request extra to determine if there are more)
	auditLogs, err := s.statefulDB.GetAuditLogsFiltered(ctx, streamerDID, limit+1, before, actionFilter, moderatorFilter)
	if err != nil {
		log.Error(ctx, "failed to get audit logs", "err", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to get audit logs")
	}

	// Build a set of result URIs already in the audit log (for de-duplication)
	auditLogURIs := make(map[string]struct{})
	for _, l := range auditLogs {
		if l.ResultURI != "" {
			auditLogURIs[l.ResultURI] = struct{}{}
		}
	}

	// Fetch historical blocks and gates from model DB to include records
	// created before the audit log system was added
	var syntheticLogs []*statedb.ModerationAuditLog

	// Only include historical records if not filtering by moderator (since we assume streamer created them)
	// and if the action filter includes blocks/gates
	includeBlocks := actionFilter == nil || *actionFilter == "createBlock"
	includeGates := actionFilter == nil || *actionFilter == "createGate"
	noModeratorFilter := moderatorFilter == nil || *moderatorFilter == "" || *moderatorFilter == streamerDID

	if noModeratorFilter && includeBlocks {
		blocks, err := s.model.GetBlocksForRepo(ctx, streamerDID)
		if err != nil {
			log.Warn(ctx, "failed to get blocks for repo", "err", err)
		} else {
			for _, block := range blocks {
				blockURI := fmt.Sprintf("at://%s/app.bsky.graph.block/%s", block.RepoDID, block.RKey)
				if _, exists := auditLogURIs[blockURI]; !exists {
					// Apply cursor filter to synthetic entries too
					if before != nil && !block.CreatedAt.Before(*before) {
						continue
					}
					syntheticLogs = append(syntheticLogs, &statedb.ModerationAuditLog{
						ID:           0, // Synthetic entries use negative IDs later
						StreamerDID:  streamerDID,
						ModeratorDID: streamerDID, // Assume streamer created it
						Action:       "createBlock",
						TargetDID:    block.SubjectDID,
						ResultURI:    blockURI,
						Success:      true,
						CreatedAt:    block.CreatedAt,
					})
				}
			}
		}
	}

	if noModeratorFilter && includeGates {
		gates, err := s.model.GetUserGates(ctx, streamerDID)
		if err != nil {
			log.Warn(ctx, "failed to get gates for repo", "err", err)
		} else {
			for _, gate := range gates {
				gateURI := fmt.Sprintf("at://%s/place.stream.chat.gate/%s", gate.RepoDID, gate.RKey)
				if _, exists := auditLogURIs[gateURI]; !exists {
					// Apply cursor filter to synthetic entries too
					if before != nil && !gate.CreatedAt.Before(*before) {
						continue
					}
					// Extract target DID from hidden message URI (at://<did>/place.stream.chat.message/<rkey>)
					var targetDID string
					if strings.HasPrefix(gate.HiddenMessage, "at://") {
						parts := strings.SplitN(gate.HiddenMessage[5:], "/", 2)
						if len(parts) > 0 {
							targetDID = parts[0]
						}
					}
					syntheticLogs = append(syntheticLogs, &statedb.ModerationAuditLog{
						ID:           0,
						StreamerDID:  streamerDID,
						ModeratorDID: streamerDID,
						Action:       "createGate",
						TargetURI:    gate.HiddenMessage,
						TargetDID:    targetDID,
						ResultURI:    gateURI,
						Success:      true,
						CreatedAt:    gate.CreatedAt,
					})
				}
			}
		}
	}

	// Merge audit logs with synthetic entries
	allLogs := append(auditLogs, syntheticLogs...)

	// Sort by createdAt descending
	sort.Slice(allLogs, func(i, j int) bool {
		return allLogs[i].CreatedAt.After(allLogs[j].CreatedAt)
	})

	// Check if there are more results and apply limit
	var nextCursor *string
	if len(allLogs) > limit {
		allLogs = allLogs[:limit]
		lastTimestamp := allLogs[len(allLogs)-1].CreatedAt.Format(time.RFC3339Nano)
		nextCursor = &lastTimestamp
	}

	// Collect unique DIDs for profile resolution
	didSet := make(map[string]struct{})
	for _, l := range allLogs {
		if l.ModeratorDID != "" {
			didSet[l.ModeratorDID] = struct{}{}
		}
		if l.TargetDID != "" {
			didSet[l.TargetDID] = struct{}{}
		}
	}

	// Fetch profiles for all DIDs
	profiles := make(map[string]*appbskytypes.ActorDefs_ProfileViewBasic)
	if len(didSet) > 0 {
		dids := make([]string, 0, len(didSet))
		for did := range didSet {
			dids = append(dids, did)
		}

		// Use the authenticated client to fetch profiles
		client.SetHeaders(map[string]string{
			"Atproto-Proxy": "did:web:api.bsky.app#bsky_appview",
		})

		// Fetch profiles in batches of 25 (API limit)
		for i := 0; i < len(dids); i += 25 {
			end := i + 25
			if end > len(dids) {
				end = len(dids)
			}
			batch := dids[i:end]

			var profilesResp appbskytypes.ActorGetProfiles_Output
			err := client.Do(ctx, xrpc.Query, "application/json", "app.bsky.actor.getProfiles", map[string]any{"actors": batch}, nil, &profilesResp)
			if err != nil {
				log.Warn(ctx, "failed to fetch profiles", "err", err, "dids", batch)
				// Continue without profiles rather than failing the whole request
				continue
			}

			for _, profile := range profilesResp.Profiles {
				profiles[profile.Did] = &appbskytypes.ActorDefs_ProfileViewBasic{
					Did:         profile.Did,
					Handle:      profile.Handle,
					DisplayName: profile.DisplayName,
					Avatar:      profile.Avatar,
				}
			}
		}
	}

	// Build set of deleted URIs for computing canUndo
	// A createBlock/createGate can be undone if there's no corresponding deleteBlock/deleteGate
	deletedURIs := make(map[string]struct{})
	for _, l := range allLogs {
		if (l.Action == "deleteBlock" || l.Action == "deleteGate") && l.TargetURI != "" && l.Success {
			deletedURIs[l.TargetURI] = struct{}{}
		}
	}

	// Convert to API format
	// Use negative IDs for synthetic entries to distinguish them
	syntheticID := int64(-1)
	entries := make([]*streamplace.ModerationGetAuditLog_AuditLogEntry, len(allLogs))
	for i, l := range allLogs {
		var entryID int64
		if l.ID == 0 {
			// Synthetic entry
			entryID = syntheticID
			syntheticID--
		} else {
			entryID = int64(l.ID)
		}

		entry := &streamplace.ModerationGetAuditLog_AuditLogEntry{
			Id:        entryID,
			Action:    l.Action,
			Success:   l.Success,
			CreatedAt: l.CreatedAt.Format(time.RFC3339),
		}

		// Add moderator profile
		if profile, ok := profiles[l.ModeratorDID]; ok {
			entry.Moderator = profile
		} else {
			// Fallback to just the DID
			entry.Moderator = &appbskytypes.ActorDefs_ProfileViewBasic{
				Did:    l.ModeratorDID,
				Handle: l.ModeratorDID,
			}
		}

		// Add optional fields
		if l.TargetURI != "" {
			entry.TargetUri = &l.TargetURI
		}
		if l.TargetDID != "" {
			entry.TargetDid = &l.TargetDID
			if profile, ok := profiles[l.TargetDID]; ok {
				entry.TargetProfile = profile
			}
		}
		if l.ResultURI != "" {
			entry.ResultUri = &l.ResultURI
		}
		if l.ErrorMsg != "" {
			entry.ErrorMsg = &l.ErrorMsg
		}

		// Determine if action can be undone
		// Only successful createBlock/createGate that haven't been deleted can be undone
		_, isDeleted := deletedURIs[l.ResultURI]
		canUndo := l.Success &&
			(l.Action == "createBlock" || l.Action == "createGate") &&
			l.ResultURI != "" &&
			!isDeleted
		entry.CanUndo = &canUndo

		entries[i] = entry
	}

	return &streamplace.ModerationGetAuditLog_Output{
		Logs:   entries,
		Cursor: nextCursor,
	}, nil
}
