package atproto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/placestream"
)

func TestModerationPermissionViewEventJSONIncludesRepoRevision(t *testing.T) {
	event := moderationPermissionViewEvent{
		ModerationDefs_PermissionView: placestream.ModerationDefs_PermissionView{
			LexiconTypeID: "place.stream.moderation.defs#permissionView",
			Uri:           "at://did:plc:streamer/place.stream.moderation.permission/3kq",
		},
		RepoRev: "3kqaaab",
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(data, &payload))
	require.Equal(t, event.LexiconTypeID, payload["$type"])
	require.Equal(t, event.Uri, payload["uri"])
	require.Equal(t, event.RepoRev, payload["repoRev"])
}
