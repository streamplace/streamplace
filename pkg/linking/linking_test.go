package linking

import (
	"context"
	"io"
	"net/url"
	"strings"
	"testing"

	"github.com/bluesky-social/indigo/api/atproto"
	"stream.place/streamplace/pkg/appbsky"
	glexrt "github.com/streamplace/glex/runtime"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/js/app"
	"stream.place/streamplace/pkg/lex"
	"stream.place/streamplace/pkg/placestream"
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
	linker, err := NewLinker(context.Background(), index, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, linker)
}

func TestGenerateLinkCard(t *testing.T) {
	index := IndexHTML(t)
	linker, err := NewLinker(context.Background(), index, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, linker)

	u, err := url.Parse("https://stream.place/iame.li")
	require.NoError(t, err)
	sp := "https://stream.place"
	ls := &placestream.Livestream{
		CreatedAt: "2025-03-25T00:39:49.121Z",
		Post: &atproto.RepoStrongRef{
			Cid: "bafyreiczmyne5jd4lpax5ttyb5p2fbcageyt6fsthdpyymecokcsmyh4a4",
			Uri: "at://did:plc:2zmxikig2sj7gqaezl5gntae/app.appbsky.feed.post/3ll5zuomua22x",
		},
		Title: "Back up! Once again water in the firehose. Link cards if this stays stable",
		Url:   &sp,
	}
	lsv := &placestream.Livestream_LivestreamView{
		Author: appbsky.ActorDefs_ProfileViewBasic{
			Handle: "iame.li",
			Did:    "did:plc:2zmxikig2sj7gqaezl5gntae",
		},
		Cid:       "bafyreib2ohz45jileumnuwa3wdoo3o7caikfyq467eanleqcscouh5wery",
		IndexedAt: "2025-03-25T01:16:14Z",
		Record:    &glexrt.LexiconTypeDecoder{Val: ls},
		Uri:       "at://did:plc:2zmxikig2sj7gqaezl5gntae/place.stream.livestream/3ll5zuop2k22x",
	}
	linkCard, err := linker.GenerateStreamerCard(context.Background(), u, lsv, "")
	require.NoError(t, err)
	linkStr := string(linkCard)
	require.True(t, strings.Contains(linkStr, "iame.li"))
	require.True(t, strings.Contains(linkStr, ls.Title), "should contain the livestream title")
	require.True(t, strings.Count(linkStr, "<title>") == 1, "should have exactly one title tag")
}

func TestGenerateVideoCard(t *testing.T) {
	index := IndexHTML(t)
	linker, err := NewLinker(context.Background(), index, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, linker)

	u, err := url.Parse("https://stream.place/iame.li/video/3mms3tfkcxstu")
	require.NoError(t, err)

	thumbCID := "bafyreib2ohz45jileumnuwa3wdoo3o7caikfyq467eanleqcscouh5wery"
	c, err := cid.Decode(thumbCID)
	require.NoError(t, err)

	video := &placestream.Video{
		Title: "My excellent VOD",
		Thumb: &lex.Blob{
			Ref:      lex.Link(c),
			MimeType: "image/jpeg",
		},
	}
	vv := &placestream.MediaGetVideo_VideoView{
		Author: appbsky.ActorDefs_ProfileViewBasic{
			Handle: "iame.li",
			Did:    "did:plc:2zmxikig2sj7gqaezl5gntae",
		},
		Cid:    "bafyreib2ohz45jileumnuwa3wdoo3o7caikfyq467eanleqcscouh5wery",
		Record: &glexrt.LexiconTypeDecoder{Val: video},
		Uri:    "at://did:plc:2zmxikig2sj7gqaezl5gntae/place.stream.video/3mms3tfkcxstu",
	}

	linkCard, err := linker.GenerateVideoCard(context.Background(), u, vv, "")
	require.NoError(t, err)
	linkStr := string(linkCard)
	require.True(t, strings.Contains(linkStr, "iame.li"), "should contain the author handle")
	require.True(t, strings.Contains(linkStr, "<title>"+video.Title+"</title>"),
		"page title should be the video's own title")
	require.True(t, strings.Contains(linkStr, `content="`+video.Title+`"`),
		"og:title/twitter:title should be the video's own title")
	require.True(t, strings.Contains(linkStr,
		"https://cdn.appbsky.app/img/feed_thumbnail/plain/did:plc:2zmxikig2sj7gqaezl5gntae/"+thumbCID+"@jpeg"),
		"og:image should be the video thumbnail served via the bsky CDN")
	require.True(t, strings.Count(linkStr, "<title>") == 1, "should have exactly one title tag")
}
