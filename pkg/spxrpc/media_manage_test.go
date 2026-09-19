package spxrpc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

func TestVideoDraftForLivestreams(t *testing.T) {
	a := livestreamItem{
		ls: &model.Livestream{URI: "at://did:plc:x/place.stream.livestream/first", CID: "bafyfirst"},
		rec: &placestream.Livestream{Title: "  State of the Union  ", Tags: []string{"politics"},
			Activity: &placestream.Livestream_Activity{Defs_ActivityLabel: &placestream.Defs_ActivityLabel{Label: "Talk"}}},
	}
	b := livestreamItem{
		ls:  &model.Livestream{URI: "at://did:plc:x/place.stream.livestream/second", CID: "bafysecond"},
		rec: &placestream.Livestream{Title: "State of the Union (continued)"},
	}

	v := videoDraftForLivestreams([]livestreamItem{a, b}, "", "")
	require.Equal(t, "State of the Union", v.Title, "the first livestream's title by default")
	require.Nil(t, v.Description)
	require.Equal(t, []string{"politics"}, v.Tags)
	require.NotNil(t, v.Activity.Defs_ActivityLabel)
	require.Len(t, v.Connections, 2, "connected to every livestream it came from")
	require.Equal(t, a.ls.URI, v.Connections[0].Video_Connection.Ref.Uri)
	require.Equal(t, a.ls.CID, v.Connections[0].Video_Connection.Ref.Cid)
	require.Equal(t, b.ls.URI, v.Connections[1].Video_Connection.Ref.Uri)

	v = videoDraftForLivestreams([]livestreamItem{a}, "Replay", " Full speech. ")
	require.Equal(t, "Replay", v.Title)
	require.Equal(t, "Full speech.", *v.Description)

	v = videoDraftForLivestreams([]livestreamItem{{ls: a.ls, rec: &placestream.Livestream{}}}, "", "")
	require.Equal(t, "Livestream", v.Title, "a record with no title at all")
}

// The draft rides in a task payload, so it has to survive JSON, and the
// record built from it has to be the one PublishVideo expects (a full Video
// with an empty source union does not marshal at all, which is why the
// payload carries a draft and not a record).
func TestVideoDraftPayloadRoundTrip(t *testing.T) {
	a := livestreamItem{
		ls: &model.Livestream{URI: "at://did:plc:x/place.stream.livestream/first", CID: "bafyfirst"},
		rec: &placestream.Livestream{Title: "Speech", Tags: []string{"politics"},
			Activity: &placestream.Livestream_Activity{Defs_ActivityLabel: &placestream.Defs_ActivityLabel{Label: "Talk"}}},
	}
	task := statedb.FinalizeLivestreamVODTask{UploadID: "u", RepoDID: "did:plc:x", LivestreamURI: a.ls.URI,
		Publish: videoDraftForLivestreams([]livestreamItem{a}, "", "the whole thing")}
	b, err := json.Marshal(task)
	require.NoError(t, err)
	var back statedb.FinalizeLivestreamVODTask
	require.NoError(t, json.Unmarshal(b, &back))
	require.NotNil(t, back.Publish)
	rec := back.Publish.Record()
	require.Equal(t, "Speech", rec.Title)
	require.Equal(t, "the whole thing", *rec.Description)
	require.Equal(t, []string{"politics"}, rec.Tags)
	require.Equal(t, "Talk", rec.Activity.Defs_ActivityLabel.Label)
	require.Len(t, rec.Connections, 1)
	require.Equal(t, a.ls.URI, rec.Connections[0].Video_Connection.Ref.Uri)
	require.Equal(t, a.ls.CID, rec.Connections[0].Video_Connection.Ref.Cid)
	require.Equal(t, "place.stream.video", rec.RecordTypeID())
}

func TestUniqueURIs(t *testing.T) {
	one := " at://a "
	require.Equal(t, []string{"at://a", "at://b"}, uniqueURIs(&one, []string{"at://b", "", "at://a"}))
	require.Nil(t, uniqueURIs(nil, nil))
}

func TestVideoOwner(t *testing.T) {
	did, err := videoOwner("at://did:plc:abc/place.stream.video/3k")
	require.NoError(t, err)
	require.Equal(t, "did:plc:abc", did)
	_, err = videoOwner("at://did:plc:abc/place.stream.livestream/3k")
	require.Error(t, err)
	_, err = videoOwner("nope")
	require.Error(t, err)
}
