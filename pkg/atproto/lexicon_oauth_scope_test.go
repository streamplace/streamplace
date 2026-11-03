package atproto

import (
	"context"
	"strings"
	"testing"

	"github.com/bluesky-social/indigo/atproto/lexicon"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/lexicons"
)

// The lexicon authority publishes the committed authFull exactly once, and
// the OAuth scope it derives from it names every record collection.
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

	scopes := strings.Fields(OAuthString)
	require.Contains(t, scopes, "include:"+lexicons.AuthFullID)
	for _, record := range records {
		require.Contains(t, scopes, "repo?collection="+record)
	}
	require.NotContains(t, scopes, "repo?collection="+lexicons.AuthFullID)
}
