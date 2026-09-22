package spxrpc

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
)

func putLivestream(t *testing.T, mod model.Model, uri string, createdAt time.Time, endedAt *time.Time) {
	t.Helper()
	rec := placestream.Livestream{LexiconTypeID: "place.stream.livestream", Title: "t", CreatedAt: createdAt.UTC().Format(time.RFC3339)}
	if endedAt != nil {
		e := endedAt.UTC().Format(time.RFC3339)
		rec.EndedAt = &e
	}
	var buf bytes.Buffer
	require.NoError(t, rec.MarshalCBOR(&buf))
	body := buf.Bytes()
	au, err := syntax.ParseATURI(uri)
	require.NoError(t, err)
	require.NoError(t, mod.CreateLivestream(context.Background(), &model.Livestream{URI: uri, CID: "bafy" + au.RecordKey().String(), CreatedAt: createdAt, Livestream: &body, RepoDID: au.Authority().String()}))
}

func putChat(t *testing.T, mod model.Model, cid, streamer, text string, at time.Time) {
	t.Helper()
	rec := placestream.ChatMessage{LexiconTypeID: "place.stream.chat.message", Text: text, Streamer: streamer, CreatedAt: at.UTC().Format(time.RFC3339Nano)}
	var buf bytes.Buffer
	require.NoError(t, rec.MarshalCBOR(&buf))
	body := buf.Bytes()
	require.NoError(t, mod.CreateChatMessage(context.Background(), &model.ChatMessage{CID: cid, URI: "at://did:plc:viewer/place.stream.chat.message/" + cid,
		CreatedAt: at, ChatMessage: &body, RepoDID: "did:plc:viewer", StreamerRepoDID: streamer, IndexedAt: &at}))
}

// A recording split across two livestream records replays the chat of both,
// in stream order, from the first record's start; a record of another
// author connected to the video is ignored, and a plain upload has no replay.
func TestChatGetReplay(t *testing.T) {
	ctx := context.Background()
	mod, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	s := &Server{cli: &config.CLI{}, model: mod}
	me := "did:plc:me"
	t0 := time.Date(2026, 9, 16, 7, 12, 30, 0, time.UTC)
	t1 := t0.Add(13 * time.Minute)
	tEnd := t1.Add(61 * time.Minute)
	putLivestream(t, mod, "at://did:plc:me/place.stream.livestream/first", t0, &t1)
	putLivestream(t, mod, "at://did:plc:me/place.stream.livestream/second", t1, &tEnd)
	putLivestream(t, mod, "at://did:plc:other/place.stream.livestream/big", t0, &tEnd)
	putChat(t, mod, "bafybefore", me, "before", t0.Add(-time.Hour))
	putChat(t, mod, "bafya", me, "during first", t0.Add(5*time.Minute))
	putChat(t, mod, "bafyb", me, "during second", t1.Add(20*time.Minute))
	putChat(t, mod, "bafyc", me, "after", tEnd.Add(time.Hour))
	putChat(t, mod, "bafyo", "did:plc:other", "other channel", t0.Add(5*time.Minute))

	conn := func(uri string) placestream.Video_Connections_Elem {
		return placestream.Video_Connections_Elem{Video_Connection: &placestream.Video_Connection{
			LexiconTypeID: "place.stream.video#connection", Ref: &comatproto.RepoStrongRef{Uri: uri, Cid: "bafy"}}}
	}
	video := placestream.Video{LexiconTypeID: "place.stream.video", Title: "Replay", CreatedAt: tEnd.Format(time.RFC3339),
		Source: placestream.Video_Source{MediaDefs_SourceTracks: &placestream.MediaDefs_SourceTracks{LexiconTypeID: "place.stream.media.defs#sourceTracks"}},
		Connections: []placestream.Video_Connections_Elem{
			// listed out of order on purpose
			conn("at://did:plc:me/place.stream.livestream/second"),
			conn("at://did:plc:other/place.stream.livestream/big"),
			conn("at://did:plc:me/place.stream.livestream/first"),
		}}
	vu, _ := syntax.ParseATURI("at://did:plc:me/place.stream.video/replay")
	require.NoError(t, mod.UpsertVideo(ctx, video, vu))
	upload := placestream.Video{LexiconTypeID: "place.stream.video", Title: "Upload", CreatedAt: tEnd.Format(time.RFC3339),
		Source: placestream.Video_Source{MediaDefs_SourceTracks: &placestream.MediaDefs_SourceTracks{LexiconTypeID: "place.stream.media.defs#sourceTracks"}}}
	uu, _ := syntax.ParseATURI("at://did:plc:me/place.stream.video/upload")
	require.NoError(t, mod.UpsertVideo(ctx, upload, uu))

	out, err := s.handlePlaceStreamChatGetReplay(ctx, "", 0, "at://did:plc:me/place.stream.video/replay")
	require.NoError(t, err)
	require.Equal(t, []string{"at://did:plc:me/place.stream.livestream/first", "at://did:plc:me/place.stream.livestream/second"}, out.Livestreams)
	require.NotNil(t, out.StartedAt)
	require.Equal(t, t0.Format(time.RFC3339Nano), *out.StartedAt, "no recording on this node: the first record's start")
	var texts []string
	for _, m := range out.Messages {
		texts = append(texts, m.Record.Val.(*placestream.ChatMessage).Text)
	}
	require.Equal(t, []string{"during first", "during second"}, texts)
	require.Nil(t, out.Cursor)

	out, err = s.handlePlaceStreamChatGetReplay(ctx, "", 0, "at://did:plc:me/place.stream.video/upload")
	require.NoError(t, err)
	require.Empty(t, out.Livestreams)
	require.Empty(t, out.Messages)
	require.Nil(t, out.StartedAt)

	_, err = s.handlePlaceStreamChatGetReplay(ctx, "", 0, "at://did:plc:me/place.stream.video/nope")
	require.Error(t, err)
}
