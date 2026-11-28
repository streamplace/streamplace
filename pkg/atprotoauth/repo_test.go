package atprotoauth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRepoGrant(t *testing.T) {
	const collection = "place.stream.video"
	for _, tc := range []struct {
		name     string
		token    string
		action   string
		wildcard bool
		allowed  bool
	}{
		{"unrestricted read", "repo:" + collection, "", false, true},
		{"unrestricted delete", "repo:" + collection, "delete", false, true},
		{"restricted read", "repo:" + collection + "?action=create&action=update&action=delete", "", false, false},
		{"wrong action", "repo:" + collection + "?action=update", "delete", false, false},
		{"repeated collections and actions", "repo?collection=app.bsky.feed.post&action=create&collection=place.stream.%76ideo&action=delete", "delete", false, true},
		{"wrong collection", "repo?collection=app.bsky.feed.post", "create", false, false},
		{"positional wildcard enabled", "repo:*?action=update", "update", true, true},
		{"query wildcard disabled", "repo?collection=%2A", "delete", false, false},
		{"exact match alongside wildcard", "repo?collection=*&collection=" + collection, "create", false, true},
		{"missing collection", "repo?action=create", "create", true, false},
		{"mixed collection forms", "repo:" + collection + "?collection=" + collection, "create", false, false},
		{"invalid repeated collection", "repo?collection=" + collection + "&collection=not-an-nsid", "create", false, false},
		{"empty action", "repo:" + collection + "?action=", "create", false, false},
		{"unknown repeated action", "repo:" + collection + "?action=create&action=read", "create", false, false},
		{"unknown parameter", "repo:" + collection + "?aud=did:web:example.org", "create", false, false},
		{"malformed escape", "repo:" + collection + "?action=create&action=%GG", "create", false, false},
		{"missing equals", "repo:" + collection + "?action=create&action", "create", false, false},
		{"empty query", "repo:" + collection + "?", "create", false, false},
		{"empty parameter", "repo:" + collection + "?action=create&", "create", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.allowed, RepoGrant(tc.token, collection, tc.action, tc.wildcard))
		})
	}
}
