package vod

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/placestream"
)

func TestParseVideoURI(t *testing.T) {
	u, err := parseVideoURI(" at://did:plc:abc/place.stream.video/3mvmy2nvqbmqv ")
	require.NoError(t, err)
	require.Equal(t, "did:plc:abc", u.Authority().String())
	require.Equal(t, "3mvmy2nvqbmqv", u.RecordKey().String())

	_, err = parseVideoURI("at://did:plc:abc/place.stream.livestream/3mvmy2nvqbmqv")
	require.ErrorIs(t, err, ErrNotAVideo, "a livestream is not a video")
	_, err = parseVideoURI("at://did:plc:abc/place.stream.video")
	require.ErrorIs(t, err, ErrNotAVideo, "a collection is not a record")
	_, err = parseVideoURI("https://example.com/x")
	require.ErrorIs(t, err, ErrNotAVideo)
}

func TestApplyVideoUpdate(t *testing.T) {
	desc := "old"
	v := &placestream.Video{Title: "Old title", Description: &desc, Tags: []string{"a"},
		DescriptionFacets: []placestream.RichtextVideoFacet{{}}}

	require.False(t, applyVideoUpdate(v, VideoUpdate{}), "nothing asked, nothing changed")
	blank := "   "
	require.False(t, applyVideoUpdate(v, VideoUpdate{Title: &blank}), "a blank title is ignored, not applied")
	require.Equal(t, "Old title", v.Title)

	title := "  New title "
	require.True(t, applyVideoUpdate(v, VideoUpdate{Title: &title}))
	require.Equal(t, "New title", v.Title)

	nd := "new"
	require.True(t, applyVideoUpdate(v, VideoUpdate{Description: &nd}))
	require.Equal(t, "new", *v.Description)
	require.Nil(t, v.DescriptionFacets, "facets annotated the old text")

	empty := ""
	require.True(t, applyVideoUpdate(v, VideoUpdate{Description: &empty}))
	require.Nil(t, v.Description, "an empty description clears it")

	require.True(t, applyVideoUpdate(v, VideoUpdate{Tags: []string{"b", " ", "c"}}))
	require.Equal(t, []string{"b", "c"}, v.Tags)
	require.False(t, applyVideoUpdate(v, VideoUpdate{Tags: []string{"b", "c"}}), "same tags, no change")
	require.True(t, applyVideoUpdate(v, VideoUpdate{Tags: []string{}}))
	require.Nil(t, v.Tags, "an empty list clears the tags")
}

func TestVideoRecordTrackURIs(t *testing.T) {
	require.Nil(t, VideoRecord{}.TrackURIs())
	require.Nil(t, VideoRecord{Video: &placestream.Video{}}.TrackURIs(), "no source, no tracks")
	r := VideoRecord{Video: &placestream.Video{Source: placestream.Video_Source{
		MediaDefs_SourceTracks: &placestream.MediaDefs_SourceTracks{Tracks: []comatproto.RepoStrongRef{
			{Uri: "at://did:plc:abc/place.stream.media.track/1"}, {Uri: "at://did:plc:abc/place.stream.media.track/2"},
		}}}}}
	require.Equal(t, []string{"at://did:plc:abc/place.stream.media.track/1", "at://did:plc:abc/place.stream.media.track/2"}, r.TrackURIs())
}

// recordingClient answers getRecord with a canned video and records every
// procedure it is asked to run.
type recordingClient struct {
	video placestream.Video
	cid   string
	calls []string
	inps  []any
}

func (c *recordingClient) Do(_ context.Context, _ string, _ string, path string, params map[string]any, body any, out any) error {
	c.calls = append(c.calls, path)
	c.inps = append(c.inps, body)
	if path == "com.atproto.repo.getRecord" {
		b, _ := json.Marshal(c.video)
		var o comatproto.RepoGetRecord_Output
		raw := `{"uri":"at://` + params["repo"].(string) + `/place.stream.video/` + params["rkey"].(string) + `","cid":"` + c.cid + `","value":` + string(b) + `}`
		if err := json.Unmarshal([]byte(raw), &o); err != nil {
			return err
		}
		*out.(*comatproto.RepoGetRecord_Output) = o
	}
	return nil
}

func TestGetVideoRecordDecodesTypedRecord(t *testing.T) {
	c := &recordingClient{cid: "bafyvideo", video: placestream.Video{Title: "Speech", DurationMs: 42,
		Source: placestream.Video_Source{MediaDefs_SourceTracks: &placestream.MediaDefs_SourceTracks{
			Tracks: []comatproto.RepoStrongRef{{Uri: "at://did:plc:abc/place.stream.media.track/v", Cid: "bafyv"}}}}}}
	u, err := parseVideoURI("at://did:plc:abc/place.stream.video/3mvmy2nvqbmqv")
	require.NoError(t, err)
	rec, err := getVideoRecord(context.Background(), c, u)
	require.NoError(t, err)
	require.Equal(t, "bafyvideo", rec.CID)
	require.Equal(t, "Speech", rec.Video.Title)
	require.Equal(t, int64(42), rec.Video.DurationMs)
	require.Equal(t, []string{"at://did:plc:abc/place.stream.media.track/v"}, rec.TrackURIs())
}

func TestDeleteRecordAddressesTheRightRecord(t *testing.T) {
	c := &recordingClient{}
	u, err := parseVideoURI("at://did:plc:abc/place.stream.video/3mvmy2nvqbmqv")
	require.NoError(t, err)
	require.NoError(t, deleteRecord(context.Background(), c, u))
	require.Equal(t, []string{"com.atproto.repo.deleteRecord"}, c.calls)
	inp := c.inps[0].(comatproto.RepoDeleteRecord_Input)
	require.Equal(t, "place.stream.video", inp.Collection)
	require.Equal(t, "did:plc:abc", inp.Repo)
	require.Equal(t, "3mvmy2nvqbmqv", inp.Rkey)
}
