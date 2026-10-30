package lexicons

import (
	"encoding/json"
	"testing"
	"testing/fstest"

	"github.com/bluesky-social/indigo/atproto/lexicon"
	"github.com/stretchr/testify/require"
)

func TestAuthFull(t *testing.T) {
	bundle := fstest.MapFS{
		// Records are listed once each, sorted by NSID, whatever the walk order.
		"place/stream/zed/thing.json": {Data: []byte(`{"lexicon":1,"id":"place.stream.zed.thing","defs":{"main":{"type":"record","key":"tid","record":{"type":"object","properties":{}}}}}`)},
		"place/stream/alpha.json":     {Data: []byte(`{"lexicon":1,"id":"place.stream.alpha","defs":{"main":{"type":"record","key":"tid","record":{"type":"object","properties":{}}}}}`)},
		// Non-records contribute no collection.
		"place/stream/getAlpha.json": {Data: []byte(`{"lexicon":1,"id":"place.stream.getAlpha","defs":{"main":{"type":"query","output":{"encoding":"application/json"}}}}`)},
		"place/stream/defs.json":     {Data: []byte(`{"lexicon":1,"id":"place.stream.defs","defs":{"thing":{"type":"object","properties":{}}}}`)},
		// A previously generated authFull must not feed back into itself.
		"place/stream/authFull.json": {Data: []byte(`{"lexicon":1,"id":"place.stream.authFull","defs":{"main":{"type":"permission-set","permissions":[{"type":"permission","resource":"repo","collection":["place.stream.gone"]}]}}}`)},
		// Imported lexicons are not ours to publish or grant.
		"app/bsky/feed/post.json": {Data: []byte(`{"lexicon":1,"id":"app.bsky.feed.post","defs":{"main":{"type":"record","key":"tid","record":{"type":"object","properties":{}}}}}`)},
	}

	lexs, err := PlaceStream(bundle)
	require.NoError(t, err)
	ids := []string{}
	for _, lex := range lexs {
		ids = append(ids, lex.ID)
	}
	require.ElementsMatch(t, []string{"place.stream.alpha", "place.stream.authFull", "place.stream.defs", "place.stream.getAlpha", "place.stream.zed.thing"}, ids)

	authFull := AuthFull(lexs)
	require.Equal(t, AuthFullID, authFull.ID)
	set, ok := authFull.Defs["main"].Inner.(lexicon.SchemaPermissionSet)
	require.True(t, ok)
	require.Len(t, set.Permissions, 1)
	require.Equal(t, "repo", set.Permissions[0].Resource)
	require.Equal(t, []string{"place.stream.alpha", "place.stream.zed.thing"}, set.Permissions[0].Collection)
	require.NoError(t, authFull.CheckSchema())
}

// The committed place/stream/authFull.json must be what the record lexicons
// generate; otherwise a new record collection is missing from the OAuth grant.
// `make lexicons` regenerates it.
func TestAuthFullUpToDate(t *testing.T) {
	lexs, err := PlaceStream(AllFiles)
	require.NoError(t, err)
	var committed *lexicon.SchemaFile
	for _, lex := range lexs {
		if lex.ID == AuthFullID {
			require.Nil(t, committed, "more than one %s lexicon", AuthFullID)
			committed = lex
		}
	}
	require.NotNil(t, committed, "%s lexicon missing, run `make lexicons`", AuthFullID)

	want, err := json.Marshal(AuthFull(lexs))
	require.NoError(t, err)
	got, err := json.Marshal(committed)
	require.NoError(t, err)
	require.JSONEq(t, string(want), string(got), "place/stream/authFull.json is stale, run `make lexicons`")
}
