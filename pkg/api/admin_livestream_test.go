package api

import (
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
)

func TestVideoRecordForLivestreams(t *testing.T) {
	a := livestreamItem{
		ls: &model.Livestream{URI: "at://did:plc:x/place.stream.livestream/first", CID: "bafyfirst"},
		rec: &placestream.Livestream{Title: "  State of the Union  ", Tags: []string{"politics"},
			Activity: &placestream.Livestream_Activity{Defs_ActivityLabel: &placestream.Defs_ActivityLabel{Label: "Talk"}}},
	}
	b := livestreamItem{
		ls:  &model.Livestream{URI: "at://did:plc:x/place.stream.livestream/second", CID: "bafysecond"},
		rec: &placestream.Livestream{Title: "State of the Union (continued)"},
	}

	v := videoRecordForLivestreams([]livestreamItem{a, b}, "", "")
	require.Equal(t, "State of the Union", v.Title, "the first livestream's title by default")
	require.Nil(t, v.Description)
	require.Equal(t, []string{"politics"}, v.Tags)
	require.NotNil(t, v.Activity.Defs_ActivityLabel)
	require.Len(t, v.Connections, 2, "connected to every livestream it came from")
	require.Equal(t, a.ls.URI, v.Connections[0].Video_Connection.Ref.Uri)
	require.Equal(t, a.ls.CID, v.Connections[0].Video_Connection.Ref.Cid)
	require.Equal(t, b.ls.URI, v.Connections[1].Video_Connection.Ref.Uri)

	v = videoRecordForLivestreams([]livestreamItem{a}, "Replay", " Full speech. ")
	require.Equal(t, "Replay", v.Title)
	require.Equal(t, "Full speech.", *v.Description)

	v = videoRecordForLivestreams([]livestreamItem{{ls: a.ls, rec: &placestream.Livestream{}}}, "", "")
	require.Equal(t, "Livestream", v.Title, "a record with no title at all")
}
