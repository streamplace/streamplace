package linking

import (
	"context"
	"io"
	"net/url"
	"strings"
	"testing"

	"github.com/ipfs/go-cid"
	glex "github.com/streamplace/glex/runtime"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/js/app"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
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
		Post: &comatproto.RepoStrongRef{
			Cid: "bafyreiczmyne5jd4lpax5ttyb5p2fbcageyt6fsthdpyymecokcsmyh4a4",
			Uri: "at://did:plc:2zmxikig2sj7gqaezl5gntae/app.bsky.feed.post/3ll5zuomua22x",
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
		Record:    &glex.LexiconTypeDecoder{Val: ls},
		Uri:       "at://did:plc:2zmxikig2sj7gqaezl5gntae/place.stream.livestream/3ll5zuop2k22x",
	}
	linkCard, err := linker.GenerateStreamerCard(context.Background(), u, lsv, "")
	require.NoError(t, err)
	linkStr := string(linkCard)
	require.True(t, strings.Contains(linkStr, "iame.li"))
	require.True(t, strings.Contains(linkStr, ls.Title), "should contain the livestream title")
	require.True(t, strings.Count(linkStr, "<title>") == 1, "should have exactly one title tag")

	// at-tags (https://tangled.org/chrisshank.com/at-tags)
	require.Contains(t, linkStr, `<meta name="at:canonical" content="`+lsv.Uri+`"/>`,
		"should map the page to the canonical livestream record")
	require.Contains(t, linkStr, `<meta name="at:author" content="at://did:plc:2zmxikig2sj7gqaezl5gntae"/>`,
		"should identify the streamer as the page author")
	require.NotContains(t, linkStr, `at:me`,
		"at:me should be omitted when the linker has no CLI/broadcaster host")
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
		Thumb: &glex.Blob{
			Ref:      glex.Link(c),
			MimeType: "image/jpeg",
		},
	}
	vv := &placestream.MediaGetVideo_VideoView{
		Author: appbsky.ActorDefs_ProfileViewBasic{
			Handle: "iame.li",
			Did:    "did:plc:2zmxikig2sj7gqaezl5gntae",
		},
		Cid:    "bafyreib2ohz45jileumnuwa3wdoo3o7caikfyq467eanleqcscouh5wery",
		Record: &glex.LexiconTypeDecoder{Val: video},
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
		"https://cdn.bsky.app/img/feed_thumbnail/plain/did:plc:2zmxikig2sj7gqaezl5gntae/"+thumbCID+"@jpeg"),
		"og:image should be the video thumbnail served via the bsky CDN")
	require.True(t, strings.Count(linkStr, "<title>") == 1, "should have exactly one title tag")

	// at-tags (https://tangled.org/chrisshank.com/at-tags)
	require.Contains(t, linkStr, `<meta name="at:canonical" content="`+vv.Uri+`"/>`,
		"should map the page to the canonical place.stream.video record")
	require.Contains(t, linkStr, `<meta name="at:author" content="at://did:plc:2zmxikig2sj7gqaezl5gntae"/>`,
		"should identify the streamer as the page author")
}

func TestGenerateDefaultCardAtMe(t *testing.T) {
	index := IndexHTML(t)
	linker, err := NewLinker(context.Background(), index, nil, &config.CLI{BroadcasterHost: "stream.place"})
	require.NoError(t, err)
	require.NotNil(t, linker)

	u, err := url.Parse("https://stream.place/")
	require.NoError(t, err)
	linkCard, err := linker.GenerateDefaultCard(context.Background(), u, "")
	require.NoError(t, err)
	linkStr := string(linkCard)
	require.Contains(t, linkStr, `<meta name="at:me" content="at://did:web:stream.place"/>`,
		"should identify the node via its did:web")
	require.NotContains(t, linkStr, "at:canonical", "front page has no canonical record")
	require.NotContains(t, linkStr, "at:author", "front page has no single author")
}
