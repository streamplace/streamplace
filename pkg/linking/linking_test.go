package linking

import (
	"context"
	"io"
	"net/url"
	"strings"
	"testing"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/js/app"
	"stream.place/streamplace/pkg/streamplace"
)

func IndexHTML(t *testing.T) []byte {
	allFiles, err := app.Files()
	require.NoError(t, err)
	require.NotNil(t, allFiles)
	index, err := allFiles.Open("index.html")
	require.NoError(t, err)
	indexBs, err := io.ReadAll(index)
	require.NoError(t, err)
	require.NotNil(t, indexBs)
	return indexBs
}

func TestNewLinker(t *testing.T) {
	index := IndexHTML(t)
	linker, err := NewLinker(context.Background(), index)
	require.NoError(t, err)
	require.NotNil(t, linker)
}

func TestGenerateLinkCard(t *testing.T) {
	index := IndexHTML(t)
	linker, err := NewLinker(context.Background(), index)
	require.NoError(t, err)
	require.NotNil(t, linker)

	u, err := url.Parse("https://stream.place/iame.li")
	require.NoError(t, err)
	sp := "https://stream.place"
	ls := &streamplace.Livestream{
		CreatedAt: "2025-03-25T00:39:49.121Z",
		Post: &atproto.RepoStrongRef{
			Cid: "bafyreiczmyne5jd4lpax5ttyb5p2fbcageyt6fsthdpyymecokcsmyh4a4",
			Uri: "at://did:plc:2zmxikig2sj7gqaezl5gntae/app.bsky.feed.post/3ll5zuomua22x",
		},
		Title: "Back up! Once again water in the firehose. Link cards if this stays stable",
		Url:   &sp,
	}
	lsv := &streamplace.Livestream_LivestreamView{
		Author: &bsky.ActorDefs_ProfileViewBasic{
			Handle: "iame.li",
			Did:    "did:plc:2zmxikig2sj7gqaezl5gntae",
		},
		Cid:       "bafyreib2ohz45jileumnuwa3wdoo3o7caikfyq467eanleqcscouh5wery",
		IndexedAt: "2025-03-25T01:16:14Z",
		Record:    &lexutil.LexiconTypeDecoder{Val: ls},
		Uri:       "at://did:plc:2zmxikig2sj7gqaezl5gntae/place.stream.livestream/3ll5zuop2k22x",
	}
	linkCard, err := linker.GenerateStreamerCard(context.Background(), u, lsv, "")
	require.NoError(t, err)
	linkStr := string(linkCard)
	require.True(t, strings.Contains(linkStr, "iame.li"))
	require.True(t, strings.Contains(linkStr, ls.Title), "should contain the livestream title")
	require.True(t, strings.Count(linkStr, "<title>") == 1, "should have exactly one title tag")
}
