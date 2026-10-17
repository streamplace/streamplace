package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSearchReposByHandle(t *testing.T) {
	db := indexedTestDB(t)
	for _, repo := range []Repo{
		{DID: "did:plc:a", Handle: "alice.example"},
		{DID: "did:plc:z", Handle: "alice-z.example"},
		{DID: "did:plc:b", Handle: "bob.example"},
		{DID: "did:plc:suffix", Handle: "not-alice.example"},
		{DID: "did:plc:inactive", Handle: "alice-inactive.example", Status: RepoStatusDeactivated},
		{DID: "did:plc:hidden", Handle: "alice-hidden.example", Status: RepoStatusTakendown},
	} {
		require.NoError(t, db.UpdateRepo(&repo))
	}

	for _, tc := range []struct {
		name  string
		query string
		limit int
		want  []string
	}{
		{name: "case insensitive prefix", query: "ALICE", limit: 20, want: []string{"alice-z.example", "alice.example"}},
		{name: "at handle and whitespace", query: "  @Alice  ", limit: 20, want: []string{"alice-z.example", "alice.example"}},
		{name: "stable limit", query: "alice", limit: 1, want: []string{"alice-z.example"}},
		{name: "no matches", query: "nobody", limit: 20},
		{name: "empty", query: "", limit: 20},
		{name: "blank", query: "  @  ", limit: 20},
		{name: "literal percent", query: "%", limit: 20},
		{name: "literal underscore", query: "alic_", limit: 20},
		{name: "literal escape", query: "alice!", limit: 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repos, err := db.SearchReposByHandle(tc.query, tc.limit)
			require.NoError(t, err)
			var handles []string
			for _, repo := range repos {
				handles = append(handles, repo.Handle)
			}
			require.Equal(t, tc.want, handles)
		})
	}
}
