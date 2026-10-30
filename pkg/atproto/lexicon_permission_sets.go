package atproto

import (
	"fmt"
	"strings"

	"github.com/bluesky-social/indigo/atproto/lexicon"
	"stream.place/streamplace/lexicons"
)

// Scope values for acting on the user's Bluesky account. Users may decline
// these at login, so anything that needs one must check the session's
// granted scope and degrade gracefully.
const (
	ScopeBskyPostCreate  = "repo?collection=app.bsky.feed.post&action=create"
	ScopeBskyActorStatus = "repo?collection=app.bsky.actor.status"
)

// oauthScope is the OAuth scope Streamplace requests: the fixed atproto and
// Bluesky grants, plus place.stream.authFull both as an include and spelled
// out collection by collection, read from the generated permission set in
// lexs so the scope and the published lexicon cannot drift apart.
func oauthScope(lexs []*lexicon.SchemaFile) (string, error) {
	var authFull *lexicon.SchemaPermissionSet
	for _, lex := range lexs {
		if lex.ID != lexicons.AuthFullID {
			continue
		}
		if authFull != nil {
			return "", fmt.Errorf("more than one %s lexicon", lexicons.AuthFullID)
		}
		set, ok := lex.Defs["main"].Inner.(lexicon.SchemaPermissionSet)
		if !ok {
			return "", fmt.Errorf("%s is not a permission set", lexicons.AuthFullID)
		}
		authFull = &set
	}
	if authFull == nil {
		return "", fmt.Errorf("%s lexicon missing, run `make lexicons`", lexicons.AuthFullID)
	}

	scopes := []string{
		"atproto",
		"blob:*/*",
		ScopeBskyPostCreate,
		ScopeBskyActorStatus,
		"repo?collection=app.bsky.graph.block",
		"repo?collection=app.bsky.graph.follow",
		"repo?collection=app.bsky.actor.profile",
		"rpc:app.bsky.actor.getProfile?aud=did:web:api.bsky.app%23bsky_appview",
		"rpc:app.bsky.actor.getProfiles?aud=did:web:api.bsky.app%23bsky_appview",
		"include:" + lexicons.AuthFullID,
		"rpc:com.atproto.moderation.createReport?aud=*",
	}
	for _, perm := range authFull.Permissions {
		if perm.Resource != "repo" {
			continue
		}
		for _, collection := range perm.Collection {
			scopes = append(scopes, "repo?collection="+collection)
		}
	}
	return strings.Join(scopes, " "), nil
}
