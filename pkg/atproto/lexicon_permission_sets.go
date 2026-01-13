package atproto

import (
	"context"
	"fmt"
	"strings"

	"github.com/bluesky-social/indigo/atproto/lexicon"
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
		"repo?collection=app.bsky.feed.post&action=create",
		"repo?collection=app.bsky.actor.status",
		"repo?collection=app.bsky.graph.block",
		"repo?collection=app.bsky.graph.follow",
		"rpc:app.bsky.actor.getProfile?aud=did:web:api.bsky.app%23bsky_appview",
		"rpc:app.bsky.actor.getProfiles?aud=did:web:api.bsky.app%23bsky_appview",
		"include:place.stream.authFull",
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
