package spxrpc

import (
	"context"
	"testing"

	glex "github.com/streamplace/glex/runtime"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

func conn(uri string) placestream.Video_Connections_Elem {
	return placestream.Video_Connections_Elem{Video_Connection: &placestream.Video_Connection{
		Ref: &comatproto.RepoStrongRef{Uri: uri, Cid: "bafy"}}}
}

func TestLivestreamConnections(t *testing.T) {
	v := &placestream.Video{Connections: []placestream.Video_Connections_Elem{
		conn("at://did:plc:me/place.stream.livestream/first"),
		conn("at://did:plc:me/place.stream.livestream/second"),
		conn("at://did:plc:me/place.stream.livestream/first"),  // repeated
		conn("at://did:plc:other/place.stream.livestream/big"), // someone else's stream
		conn("at://did:plc:me/place.stream.video/clip"),        // not a livestream
		{},
	}}
	require.Equal(t, []string{"at://did:plc:me/place.stream.livestream/first", "at://did:plc:me/place.stream.livestream/second"},
		livestreamConnections(v, "did:plc:me"))
	require.Nil(t, livestreamConnections(nil, "did:plc:me"))
}

// A replay connected to two livestream records (a recording split across
// them) counts both streams' sessions on top of its own aggregated views.
func TestAddLivestreamViews(t *testing.T) {
	ctx := context.Background()
	sdb, err := statedb.MakeDB(ctx, &config.CLI{DBURL: ":memory:"}, nil, nil)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		_, err := sdb.AddStreamView(ctx, "did:plc:me", "at://did:plc:me/place.stream.livestream/first")
		require.NoError(t, err)
	}
	for i := 0; i < 5; i++ {
		_, err := sdb.AddStreamView(ctx, "did:plc:me", "at://did:plc:me/place.stream.livestream/second")
		require.NoError(t, err)
	}
	_, err = sdb.AddStreamView(ctx, "did:plc:other", "at://did:plc:other/place.stream.livestream/big")
	require.NoError(t, err)

	s := &Server{cli: &config.CLI{}, statefulDB: sdb}
	view := &placestream.MediaGetVideo_VideoView{
		Uri:    "at://did:plc:me/place.stream.video/replay",
		Author: appbsky.ActorDefs_ProfileViewBasic{Did: "did:plc:me"},
		Record: &glex.LexiconTypeDecoder{Val: &placestream.Video{Title: "Replay", Connections: []placestream.Video_Connections_Elem{
			conn("at://did:plc:me/place.stream.livestream/first"),
			conn("at://did:plc:me/place.stream.livestream/second"),
			conn("at://did:plc:other/place.stream.livestream/big"),
		}}},
		ViewCounts: placestream.MediaGetVideo_ViewCountSummary{Count: 2, Reporters: 1},
	}
	s.addLivestreamViews(ctx, view)
	require.Equal(t, int64(2+3+5), view.ViewCounts.Count, "own views plus both streams, not the other author's")
	require.Equal(t, int64(1), view.ViewCounts.Reporters, "untouched")

	plain := &placestream.MediaGetVideo_VideoView{Author: appbsky.ActorDefs_ProfileViewBasic{Did: "did:plc:me"},
		Record: &glex.LexiconTypeDecoder{Val: &placestream.Video{Title: "Upload"}}}
	s.addLivestreamViews(ctx, plain)
	require.Equal(t, int64(0), plain.ViewCounts.Count, "an upload with no livestream stays as aggregated")
}
