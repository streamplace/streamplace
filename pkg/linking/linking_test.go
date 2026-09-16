package linking

import (
	"context"
	"golang.org/x/net/html"
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
	"stream.place/streamplace/pkg/statedb"
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
	require.Contains(t, linkStr, "<title>iame.li&#39;s video on streamplace node</title>",
		"page title names the author (handle without a display name) and the site")
	require.Contains(t, linkStr, `<meta property="og:description" content="`+video.Title+`"/>`,
		"og:description is the video's own title")
	require.Contains(t, linkStr, `<meta property="og:type" content="video.other"/>`)
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

// The app template ships first-party link-preview tags; a card must replace
// them rather than append after them, since crawlers take the first tag.
func TestIsAppBannerMeta(t *testing.T) {
	banner := &html.Node{Type: html.ElementNode, Data: "meta", Attr: []html.Attribute{{Key: "name", Val: "apple-itunes-app"}, {Key: "content", Val: "app-id=1"}}}
	other := &html.Node{Type: html.ElementNode, Data: "meta", Attr: []html.Attribute{{Key: "name", Val: "viewport"}}}
	require.True(t, isAppBannerMeta(banner))
	require.False(t, isAppBannerMeta(other))
	require.False(t, isLinkPreviewMeta(banner), "the banner is not a link-preview tag: it stays unless mobileAppBanner=off")
}

func TestGenerateHTMLReplacesTemplatePreviewTags(t *testing.T) {
	base := []byte(`<!doctype html><html><head>
<title>Streamplace</title>
<meta name="description" content="The video layer for everything." />
<meta property="og:site_name" content="Streamplace" />
<meta property="og:title" content="Streamplace — the video layer for everything" />
<meta property="og:image" content="/linkbanner.png" />
<meta property="og:image:width" content="1200" />
<meta name="twitter:title" content="Streamplace — the video layer for everything" />
<meta name="viewport" content="width=device-width" />
</head><body></body></html>`)
	linker, err := NewLinker(context.Background(), base, nil, &config.CLI{BroadcasterHost: "example.com"})
	require.NoError(t, err)
	u, err := url.Parse("https://example.com/")
	require.NoError(t, err)
	out, err := linker.GenerateDefaultCard(context.Background(), u, "")
	require.NoError(t, err)
	page := string(out)
	require.NotContains(t, page, "video layer for everything")
	require.NotContains(t, page, `content="/linkbanner.png"`)
	require.NotContains(t, page, "og:image:width")
	require.Contains(t, page, `name="viewport"`)
	require.Equal(t, 1, strings.Count(page, `property="og:title"`))
	require.Equal(t, 1, strings.Count(page, "<title>"))
}

func TestGenerateHTMLStartsBrokerIframe(t *testing.T) {
	base := []byte(`<!doctype html><html><head><title>x</title></head><body><div id="root"></div></body></html>`)
	cli := &config.CLI{BroadcasterHost: "example.com"}
	sdb, err := statedb.MakeDB(context.Background(), &config.CLI{DBURL: ":memory:"}, nil, nil)
	require.NoError(t, err)
	linker, err := NewLinker(context.Background(), base, sdb, cli)
	require.NoError(t, err)
	u, _ := url.Parse("https://example.com/")

	out, err := linker.GenerateDefaultCard(context.Background(), u, "")
	require.NoError(t, err)
	require.NotContains(t, string(out), "sp-session-broker", "no broker configured, no iframe")

	require.NoError(t, sdb.PutBrandingBlob("did:web:example.com", "sessionBrokerOrigin", "text/plain", []byte("https://app.example.com/"), nil, nil))
	out, err = linker.GenerateDefaultCard(context.Background(), u, "")
	require.NoError(t, err)
	page := string(out)
	require.Contains(t, page, `<iframe id="sp-session-broker" src="https://app.example.com/session-broker" style="display:none" aria-hidden="true">`)
	require.Less(t, strings.Index(page, `id="root"`), strings.Index(page, "sp-session-broker"), "after the app root, so it never delays it")

	require.Equal(t, "", brokerOrigin("javascript:alert(1)"))
	require.Equal(t, "", brokerOrigin("https://app.example.com/path"))
	require.Equal(t, "", brokerOrigin("http://app.example.com"))
	require.Equal(t, "http://127.0.0.1:38555", brokerOrigin("http://127.0.0.1:38555/"))
}

// The card templates: {name} is the display name when the profile is
// indexed, the handle otherwise; {site} is cardSiteName over siteTitle; the
// tabs each carry their own title and description.
func TestCardTemplates(t *testing.T) {
	ctx := context.Background()
	index := IndexHTML(t)
	cli := &config.CLI{BroadcasterHost: "example.com"}
	sdb, err := statedb.MakeDB(ctx, &config.CLI{DBURL: ":memory:"}, nil, nil)
	require.NoError(t, err)
	linker, err := NewLinker(ctx, index, sdb, cli)
	require.NoError(t, err)
	put := func(key, value string) {
		require.NoError(t, sdb.PutBrandingBlob("did:web:example.com", key, "text/plain", []byte(value), nil, nil))
	}
	put("siteTitle", "Example Social")
	put("siteDescription", "Trust your feed")
	put("cardSiteName", "X")
	put("cardHomeTitle", "Live streams on {site}")
	put("cardHomeDescription", "Watch live streams from verified accounts on {site}.")
	put("cardVideosTitle", "{site} play")
	put("cardVideosDescription", "Watch replays of live streams from verified accounts on {site}.")
	put("cardVideoTitle", "{name} on {site} play")

	name := "Ada Lovelace"
	ls := &placestream.Livestream{CreatedAt: "2025-03-25T00:39:49.121Z", Title: "Going live from the lab"}
	lsv := &placestream.Livestream_LivestreamView{
		Author: appbsky.ActorDefs_ProfileViewBasic{Handle: "ada.example", Did: "did:plc:ada", DisplayName: &name},
		Record: &glex.LexiconTypeDecoder{Val: ls},
		Uri:    "at://did:plc:ada/place.stream.livestream/3ll5zuop2k22x",
	}
	u, _ := url.Parse("https://example.com/ada.example")
	out, err := linker.GenerateStreamerCard(ctx, u, lsv, "")
	require.NoError(t, err)
	page := string(out)
	require.Contains(t, page, `<meta property="og:title" content="Ada Lovelace is live on X"/>`)
	require.Contains(t, page, `<meta property="og:description" content="Going live from the lab"/>`)
	require.Contains(t, page, `<meta property="og:site_name" content="Example Social"/>`)
	require.Contains(t, page, "<title>Ada Lovelace is live on X</title>")

	lsv.Author.DisplayName = nil
	out, err = linker.GenerateStreamerCard(ctx, u, lsv, "")
	require.NoError(t, err)
	require.Contains(t, string(out), `content="ada.example is live on X"`, "no display name: the handle")

	video := &placestream.Video{Title: "Lab replay"}
	vv := &placestream.MediaGetVideo_VideoView{
		Author: appbsky.ActorDefs_ProfileViewBasic{Handle: "ada.example", Did: "did:plc:ada", DisplayName: &name},
		Record: &glex.LexiconTypeDecoder{Val: video},
		Uri:    "at://did:plc:ada/place.stream.video/3mms3tfkcxstu",
	}
	vu, _ := url.Parse("https://example.com/ada.example/video/3mms3tfkcxstu")
	out, err = linker.GenerateVideoCard(ctx, vu, vv, "")
	require.NoError(t, err)
	page = string(out)
	require.Contains(t, page, `<meta property="og:title" content="Ada Lovelace on X play"/>`)
	require.Contains(t, page, `<meta property="og:description" content="Lab replay"/>`)

	home, _ := url.Parse("https://example.com/")
	out, err = linker.GenerateLandingCard(ctx, home, LandingHome, "")
	require.NoError(t, err)
	page = string(out)
	require.Contains(t, page, `<meta property="og:title" content="Live streams on X"/>`)
	require.Contains(t, page, `<meta property="og:description" content="Watch live streams from verified accounts on X."/>`)
	require.Contains(t, page, `<meta property="og:image" content="https://example.com/linkbanner.png"/>`)

	videos, _ := url.Parse("https://example.com/video")
	out, err = linker.GenerateLandingCard(ctx, videos, LandingVideos, "")
	require.NoError(t, err)
	page = string(out)
	require.Contains(t, page, `<meta property="og:title" content="X play"/>`)
	require.Contains(t, page, `content="Watch replays of live streams from verified accounts on X."`)

	// The go-live templates were left at their defaults.
	golive, _ := url.Parse("https://example.com/live")
	out, err = linker.GenerateLandingCard(ctx, golive, LandingGoLive, "")
	require.NoError(t, err)
	page = string(out)
	require.Contains(t, page, `<meta property="og:title" content="Go live on X"/>`)
	require.Contains(t, page, `content="Set up and start your live stream on X."`)

	// Any other page: the site's own title and description.
	out, err = linker.GenerateLandingCard(ctx, home, LandingDefault, "")
	require.NoError(t, err)
	page = string(out)
	require.Contains(t, page, `<meta property="og:title" content="Example Social"/>`)
	require.Contains(t, page, `<meta property="og:description" content="Trust your feed"/>`)
}

// Without the home templates the front page keeps the site's own card.
func TestHomeCardFallsBackToSite(t *testing.T) {
	ctx := context.Background()
	linker, err := NewLinker(ctx, IndexHTML(t), nil, nil)
	require.NoError(t, err)
	home, _ := url.Parse("https://stream.place/")
	out, err := linker.GenerateLandingCard(ctx, home, LandingHome, "")
	require.NoError(t, err)
	page := string(out)
	require.Contains(t, page, `<meta property="og:title" content="streamplace node"/>`)
	require.Contains(t, page, `<meta property="og:description" content="Open-source livestreaming on the AT Protocol."/>`)
	require.Equal(t, 1, strings.Count(page, `property="og:title"`))
}

func TestLandingPageFor(t *testing.T) {
	require.Equal(t, LandingHome, LandingPageFor("/"))
	require.Equal(t, LandingHome, LandingPageFor(""))
	require.Equal(t, LandingVideos, LandingPageFor("/video"))
	require.Equal(t, LandingVideos, LandingPageFor("/video/"))
	require.Equal(t, LandingGoLive, LandingPageFor("/live"))
	require.Equal(t, LandingGoLive, LandingPageFor("/go-live"))
	require.Equal(t, LandingDefault, LandingPageFor("/settings"))
	require.Equal(t, LandingDefault, LandingPageFor("/ada.example"))
	require.Equal(t, LandingDefault, LandingPageFor("/ada.example/video"))
}
