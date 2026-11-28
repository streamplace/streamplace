package atproto

import (
	"context"
	"strings"
	"testing"

	"github.com/bluesky-social/indigo/atproto/lexicon"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/lexicons"
	"stream.place/streamplace/pkg/spaces"
)

// The lexicon authority publishes the committed authFull exactly once, and
// public repo grants cover public collections, never space-only draft metadata.
func TestPublishedLexiconsOAuthScope(t *testing.T) {
	lexs, err := PublishedLexicons(context.Background())
	require.NoError(t, err)

	authFull := 0
	records := []string{}
	for _, lex := range lexs {
		if lex.ID == lexicons.AuthFullID {
			authFull++
		}
		if _, ok := lex.Defs["main"].Inner.(lexicon.SchemaRecord); ok {
			records = append(records, lex.ID)
		}
	}
	require.Equal(t, 1, authFull)
	require.Contains(t, records, "place.stream.livestream")
	require.Contains(t, records, spaces.DraftMetadataCollection)

	scopes := strings.Fields(OAuthString)
	require.Contains(t, scopes, "include:"+lexicons.AuthFullID)
	for _, record := range records {
		if record == spaces.DraftMetadataCollection {
			require.NotContains(t, scopes, "repo?collection="+record)
			continue
		}
		require.Contains(t, scopes, "repo?collection="+record)
	}
	require.NotContains(t, scopes, "repo?collection="+lexicons.AuthFullID)
}
