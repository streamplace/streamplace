package atproto

import (
	"context"
	"fmt"
	"strings"

	"github.com/bluesky-social/indigo/atproto/lexicon"
)

// Scope values for acting on the user's Bluesky account. Users may decline
// these at login, so anything that needs one must check the session's
// granted scope and degrade gracefully.
const (
	ScopeBskyPostCreate  = "repo?collection=app.bsky.feed.post&action=create"
	ScopeBskyActorStatus = "repo?collection=app.bsky.actor.status"
)

func generatePermissionSets(ctx context.Context, lexs []*lexicon.SchemaFile) ([]*lexicon.SchemaFile, error) {
	recordLexicons := []*lexicon.SchemaFile{}
	for _, lex := range lexs {
		main, ok := lex.Defs["main"]
		if !ok {
			continue
		}
		switch main.Inner.(type) {
		case lexicon.SchemaRecord:
			recordLexicons = append(recordLexicons, lex)
		case lexicon.SchemaPermissionSet:
			return nil, fmt.Errorf("unexpected permission set in `lexicons` directory: %s", lex.ID)
		}
	}

	allRecords := []string{}
	allCollectionStrings := []string{
		"atproto",
		"blob:*/*",
		ScopeBskyPostCreate,
		ScopeBskyActorStatus,
		"repo?collection=app.bsky.graph.block",
		"repo?collection=app.bsky.graph.follow",
		"repo?collection=app.bsky.actor.profile",
		"rpc:app.bsky.actor.getProfile?aud=did:web:api.bsky.app%23bsky_appview",
		"rpc:app.bsky.actor.getProfiles?aud=did:web:api.bsky.app%23bsky_appview",
		"include:place.stream.authFull",
		"rpc:com.atproto.moderation.createReport?aud=*",
	}
	for _, record := range recordLexicons {
		allRecords = append(allRecords, record.ID)
		allCollectionStrings = append(allCollectionStrings, fmt.Sprintf("repo?collection=%s", record.ID))
	}

	OAuthString = strings.Join(allCollectionStrings, " ")
	permissionSets := []*lexicon.SchemaFile{}

	// place.stream.authFull
	authFullTitle := "Full Streamplace Access"
	authFullDetail := "Full access to all Streamplace features and data."
	authFullSet := &lexicon.SchemaPermissionSet{
		Type:   "permission-set",
		Title:  &authFullTitle,
		Detail: &authFullDetail,
		Permissions: []lexicon.SchemaPermission{
			{
				Type:       "permission",
				Resource:   "repo",
				Collection: allRecords,
			},
		},
	}
	authFull := &lexicon.SchemaFile{
		Lexicon: 1,
		ID:      "place.stream.authFull",
		Defs: map[string]lexicon.SchemaDef{
			"main": {
				Inner: authFullSet,
			},
		},
	}
	permissionSets = append(permissionSets, authFull)

	return permissionSets, nil
}
