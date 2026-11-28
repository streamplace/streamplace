package spxrpc

import (
	"strings"

	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"stream.place/streamplace/lexicons"
	"stream.place/streamplace/pkg/atprotoauth"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/spaces"
)

func creatorOAuthAccess(session *oatproxy.OAuthSession, path string) (allowed, spaceOnly bool) {
	var collection, action string
	switch path {
	case "/xrpc/place.stream.multistream.listTargets":
		collection, action = spaces.TargetCollection, "read"
	case "/xrpc/place.stream.multistream.createTarget":
		collection, action = spaces.TargetCollection, "create"
	case "/xrpc/place.stream.multistream.putTarget":
		collection, action = spaces.TargetCollection, "update"
	case "/xrpc/place.stream.multistream.deleteTarget":
		collection, action = spaces.TargetCollection, "delete"
	case "/xrpc/place.stream.vod.getDraft", "/xrpc/place.stream.vod.listDrafts":
		collection, action = spaces.DraftMetadataCollection, "read"
	case "/xrpc/place.stream.vod.createDraft":
		collection, action = spaces.DraftMetadataCollection, "create"
	case "/xrpc/place.stream.vod.updateDraft":
		collection, action = spaces.DraftMetadataCollection, "update"
	case "/xrpc/place.stream.vod.deleteDraft", creatorPublishDraftPath:
		collection, action = spaces.DraftMetadataCollection, "delete"
	default:
		return false, false
	}

	publish := path == creatorPublishDraftPath
	scope := session.UpstreamScope
	if scope == "" {
		// A requested scope is not a grant. Preserve scope-less legacy sessions.
		return session.DownstreamScope == "" && !publish, false
	}
	repoCollection := collection
	if collection == spaces.DraftMetadataCollection {
		repoCollection = "place.stream.vod.draftVideo"
	}
	// Repo scopes have no read action; local fallback reads require an unrestricted grant.
	repoAction := action
	if action == "read" {
		repoAction = ""
	}
	videoAllowed := !publish
	var repoAllowed bool
	for token := range strings.FieldsSeq(scope) {
		if token == "include:"+lexicons.AuthFullID {
			return true, false
		}
		if !repoAllowed && atprotoauth.RepoGrant(token, repoCollection, repoAction, false) {
			repoAllowed = true
		}
		if !videoAllowed && atprotoauth.RepoGrant(token, constants.PLACE_STREAM_VIDEO, "create", false) {
			videoAllowed = true
		}
		if repoAllowed && videoAllowed {
			return true, false
		}
	}

	var spaceAllowed bool
	// Editing draft metadata reads the previous record. Target create/put reads
	// the existing targets to enforce quotas. Deny before claiming if either
	// part of the backend operation is outside the bound grant.
	switch path {
	case creatorPublishDraftPath, "/xrpc/place.stream.vod.updateDraft",
		"/xrpc/place.stream.multistream.createTarget", "/xrpc/place.stream.multistream.putTarget":
		spaceAllowed = videoAllowed && spaces.CollectionActionsGrant(scope, session.DID, collection, "read", action)
	default:
		spaceAllowed = spaces.CollectionActionsGrant(scope, session.DID, collection, action)
	}
	return spaceAllowed, spaceAllowed
}
