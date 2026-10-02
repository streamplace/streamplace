package statedb

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/model"
	placestream "stream.place/streamplace/pkg/placestream"
)

func TestVideoDraftForLivestreams(t *testing.T) {
	a := LivestreamItem{
		Livestream: &model.Livestream{URI: "at://did:plc:x/place.stream.livestream/first", CID: "bafyfirst"},
		Record: &placestream.Livestream{Title: "  State of the Union  ", Tags: []string{"politics"},
			Activity: &placestream.Livestream_Activity{Defs_ActivityLabel: &placestream.Defs_ActivityLabel{Label: "Talk"}}},
	}
	b := LivestreamItem{
		Livestream: &model.Livestream{URI: "at://did:plc:x/place.stream.livestream/second", CID: "bafysecond"},
		Record:     &placestream.Livestream{Title: "State of the Union (continued)"},
	}

	v := VideoDraftForLivestreams([]LivestreamItem{a, b}, "", "")
	require.Equal(t, "State of the Union", v.Title, "the first livestream's title by default")
	require.Nil(t, v.Description)
	require.Equal(t, []string{"politics"}, v.Tags)
	require.NotNil(t, v.Activity.Defs_ActivityLabel)
	require.Len(t, v.Connections, 2, "connected to every livestream it came from")
	require.Equal(t, a.Livestream.URI, v.Connections[0].Video_Connection.Ref.Uri)
	require.Equal(t, a.Livestream.CID, v.Connections[0].Video_Connection.Ref.Cid)
	require.Equal(t, b.Livestream.URI, v.Connections[1].Video_Connection.Ref.Uri)

	v = VideoDraftForLivestreams([]LivestreamItem{a}, "Replay", " Full speech. ")
	require.Equal(t, "Replay", v.Title)
	require.Equal(t, "Full speech.", *v.Description)

	v = VideoDraftForLivestreams([]LivestreamItem{{Livestream: a.Livestream, Record: &placestream.Livestream{}}}, "", "")
	require.Equal(t, "Livestream", v.Title, "a record with no title at all")
}

// The draft rides in a task payload, so it has to survive JSON, and the
// record built from it has to be the one PublishVideo expects (a full Video
// with an empty source union does not marshal at all, which is why the
// payload carries a draft and not a record).
func TestVideoDraftPayloadRoundTrip(t *testing.T) {
	a := LivestreamItem{
		Livestream: &model.Livestream{URI: "at://did:plc:x/place.stream.livestream/first", CID: "bafyfirst"},
		Record: &placestream.Livestream{Title: "Speech", Tags: []string{"politics"},
			Activity: &placestream.Livestream_Activity{Defs_ActivityLabel: &placestream.Defs_ActivityLabel{Label: "Talk"}}},
	}
	task := FinalizeLivestreamVODTask{UploadID: "u", RepoDID: "did:plc:x", LivestreamURI: a.Livestream.URI,
		Publish: VideoDraftForLivestreams([]LivestreamItem{a}, "", "the whole thing")}
	b, err := json.Marshal(task)
	require.NoError(t, err)
	var back FinalizeLivestreamVODTask
	require.NoError(t, json.Unmarshal(b, &back))
	require.NotNil(t, back.Publish)
	rec := back.Publish.Record()
	require.Equal(t, "Speech", rec.Title)
	require.Equal(t, "the whole thing", *rec.Description)
	require.Equal(t, []string{"politics"}, rec.Tags)
	require.Equal(t, "Talk", rec.Activity.Defs_ActivityLabel.Label)
	require.Len(t, rec.Connections, 1)
	require.Equal(t, a.Livestream.URI, rec.Connections[0].Video_Connection.Ref.Uri)
	require.Equal(t, a.Livestream.CID, rec.Connections[0].Video_Connection.Ref.Cid)
	require.Equal(t, "place.stream.video", rec.RecordTypeID())
}
