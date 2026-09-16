package api

import (
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
)

func TestVideoRecordForLivestream(t *testing.T) {
	ls := &model.Livestream{URI: "at://did:plc:x/place.stream.livestream/abc", CID: "bafycid"}
	rec := &placestream.Livestream{Title: "  State of the Union  ", Tags: []string{"politics"},
		Activity: &placestream.Livestream_Activity{Defs_ActivityLabel: &placestream.Defs_ActivityLabel{Label: "Talk"}}}

	v := videoRecordForLivestream(rec, ls, "", "")
	require.Equal(t, "State of the Union", v.Title, "the livestream's title by default")
	require.Nil(t, v.Description)
	require.Equal(t, []string{"politics"}, v.Tags)
	require.NotNil(t, v.Activity.Defs_ActivityLabel)
	require.Len(t, v.Connections, 1)
	require.Equal(t, ls.URI, v.Connections[0].Video_Connection.Ref.Uri, "connected to the livestream it came from")
	require.Equal(t, ls.CID, v.Connections[0].Video_Connection.Ref.Cid)

	v = videoRecordForLivestream(rec, ls, "Replay", " Full speech. ")
	require.Equal(t, "Replay", v.Title)
	require.Equal(t, "Full speech.", *v.Description)

	v = videoRecordForLivestream(&placestream.Livestream{}, ls, "", "")
	require.Equal(t, "Livestream", v.Title, "a record with no title at all")
}
