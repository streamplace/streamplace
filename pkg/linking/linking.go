package linking

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"stream.place/streamplace/pkg/log"

	"golang.org/x/net/html"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/branding"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

type Linker struct {
	BaseHTML []byte
	sdb      *statedb.StatefulDB
	cli      *config.CLI
}

func NewLinker(ctx context.Context, baseHTML []byte, sdb *statedb.StatefulDB, cli *config.CLI) (*Linker, error) {
	_, err := html.Parse(bytes.NewReader(baseHTML))
	if err != nil {
		return nil, err
	}

	return &Linker{BaseHTML: baseHTML, sdb: sdb, cli: cli}, nil
}

type PageConfig struct {
	Title     string
	Metas     []MetaTag
	SentryDSN string
	Branding  []string
}

// Define all meta tags in a structured way
type MetaTag struct {
	Type    string // "name" or "property"
	Key     string
	Content string
}

var BrandingAssetList = func() []string {
	keys := make([]string, 0, len(branding.Specs))
	for _, spec := range branding.Specs {
		keys = append(keys, spec.Key)
	}
	return keys
}()

// inlineBrandingImageLimit caps the image assets embedded as data URLs in
// the internal-brand meta tags.
const inlineBrandingImageLimit = 96 * 1024

// hexColor accepts #rgb / #rrggbb / #rrggbbaa, the only forms the app's
// theme accepts, so a stored value can be dropped straight into a style.
var hexColor = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

// bodyBackground returns the node's branded dark background color, if any,
// so the page can paint it before the bundle loads instead of the default
// then re-painting (the "flash of unbranded content").
func (l *Linker) bodyBackground() string {
	v := l.brandingText("backgroundColor")
	if !hexColor.MatchString(v) {
		return ""
	}
	return v
}

// brandingText reads a text branding key for this node, "" when unset.
func (l *Linker) brandingText(key string) string {
	if l.sdb == nil || l.cli == nil {
		return ""
	}
	blob, err := l.sdb.GetBrandingBlob("did:web:"+l.cli.BroadcasterHost, key)
	if err != nil || blob == nil {
		return ""
	}
	return strings.TrimSpace(string(blob.Data))
}

func findChild(parent *html.Node, name string) *html.Node {
	for node := range parent.ChildNodes() {
		if node.Type == html.ElementNode && node.Data == name {
			return node
		}
	}
	return nil
}

// isAppBannerMeta reports whether a <meta> is the iOS Smart App Banner
// (apple-itunes-app), which the template ships for the first-party app.
func isAppBannerMeta(node *html.Node) bool {
	for _, attr := range node.Attr {
		if attr.Key == "name" && attr.Val == "apple-itunes-app" {
			return true
		}
	}
	return false
}

// atTags returns meta tags implementing the at-tags proposal
// (https://tangled.org/chrisshank.com/at-tags), which maps web pages back to
// atproto identities and records via <meta name="at:*"> tags in the page
// <head>:
//
//   - at:canonical — the AT URI of the record this page canonically maps to
//     (pass "" to omit, e.g. for pages that don't represent a record)
//   - at:author    — the atproto identity of the page's author, as an
//     at://<did> URI (pass "" to omit)
//   - at:me        — the identity of the overall website, i.e. this node's
//     did:web, when a broadcaster host is configured
func (l *Linker) atTags(canonicalURI string, authorDID string) []MetaTag {
	tags := make([]MetaTag, 0, 3)
	if strings.HasPrefix(canonicalURI, "at://") {
		tags = append(tags, MetaTag{Type: "name", Key: "at:canonical", Content: canonicalURI})
	}
	if authorDID != "" {
		tags = append(tags, MetaTag{Type: "name", Key: "at:author", Content: "at://" + authorDID})
	}
	if l.cli != nil && l.cli.BroadcasterHost != "" {
		tags = append(tags, MetaTag{Type: "name", Key: "at:me", Content: "at://did:web:" + l.cli.BroadcasterHost})
	}
	return tags
}

// fetch branding assets for a given broadcaster DID
func (l *Linker) getBrandingAssets(broadcasterDid string) ([]placestream.BrandingGetBranding_BrandingAsset, error) {
	ret := make([]placestream.BrandingGetBranding_BrandingAsset, 0)
	for _, asset := range BrandingAssetList {
		blob, err := l.sdb.GetBrandingBlob(broadcasterDid, asset)
		if err != nil {
			// this can probably include a 'record not found' error, in which case we skip
			log.Debug(context.Background(), "error fetching branding asset for broadcaster", "asset", asset, "broadcaster", broadcasterDid, "error", err)
			continue
		}
		asset := placestream.BrandingGetBranding_BrandingAsset{
			Key:      blob.Key,
			MimeType: blob.MimeType,
		}

		if blob.Width != nil {
			w := int64(*blob.Width)
			asset.Width = &w
		}
		if blob.Height != nil {
			h := int64(*blob.Height)
			asset.Height = &h
		}

		// process based on mime type
		if blob.MimeType == "text/plain" {
			str := string(blob.Data)
			asset.Data = &str
		} else {
			url := fmt.Sprintf("/xrpc/place.stream.branding.getBlob?key=%s&broadcaster=%s", blob.Key, broadcasterDid)
			asset.Url = &url
			// Small images ride along inline so the very first paint can draw
			// the node's logo instead of the default mark while the blob
			// fetch is in flight. Large ones (sidebar backgrounds) stay
			// URL-only; the meta tag would dwarf the page.
			if len(blob.Data) <= inlineBrandingImageLimit {
				dataURL := "data:" + blob.MimeType + ";base64," + base64.StdEncoding.EncodeToString(blob.Data)
				asset.Data = &dataURL
			}
		}
		ret = append(ret, asset)
	}

	return ret, nil
}

// brandMetas fetches the node's branding once for a card: the
// internal-brand:* meta tags the app hydrates from, plus every text value by
// key so the card's own title and description can be branded.
func (l *Linker) brandMetas(ctx context.Context) ([]MetaTag, map[string]string) {
	values := map[string]string{}
	if l.sdb == nil || l.cli == nil {
		return nil, values
	}
	assets, err := l.getBrandingAssets("did:web:" + l.cli.BroadcasterHost)
	if err != nil {
		// log but we should not block rendering
		log.Error(ctx, "error fetching branding assets", "error", err)
		return nil, values
	}
	var metas []MetaTag
	for i := range assets {
		val := assets[i]
		if val.MimeType == branding.TextMime && val.Data != nil {
			values[val.Key] = strings.TrimSpace(*val.Data)
		}
		marshalledJson, err := json.Marshal(val)
		if err != nil {
			log.Error(ctx, "error marshalling branding asset", "key", val.Key, "error", err)
			continue
		}
		metas = append(metas, MetaTag{
			Type:    "name",
			Key:     "internal-brand:" + val.Key,
			Content: string(marshalledJson),
		})
	}
	return metas, values
}

// The card text when a node has no siteTitle / siteDescription branding.
const (
	defaultSiteTitle       = "streamplace node"
	defaultSiteDescription = "Open-source livestreaming on the AT Protocol."
)

// cardText resolves one of the card template keys: the node's value when
// set, else the vocabulary's default.
func cardText(values map[string]string, key string) string {
	if v := values[key]; v != "" {
		return v
	}
	if spec, ok := branding.Lookup(key); ok {
		return spec.Default
	}
	return ""
}

// siteTitle is the node's name for og:site_name and the page title fallback;
// siteName is how the card templates refer to it ({site}), which a node can
// shorten with cardSiteName ("W" for a "W Social" site).
func siteTitle(values map[string]string) string {
	if v := values["siteTitle"]; v != "" {
		return v
	}
	return defaultSiteTitle
}

func siteName(values map[string]string) string {
	if v := values["cardSiteName"]; v != "" {
		return v
	}
	return siteTitle(values)
}

// expandCard fills a card template's placeholders: {site}, and for a page
// about one account {name} (display name, falling back to the handle) and
// {handle}.
func expandCard(tmpl string, values map[string]string, author *appbsky.ActorDefs_ProfileViewBasic) string {
	r := []string{"{site}", siteName(values)}
	if author != nil {
		name := author.Handle
		if author.DisplayName != nil && strings.TrimSpace(*author.DisplayName) != "" {
			name = strings.TrimSpace(*author.DisplayName)
		}
		r = append(r, "{name}", name, "{handle}", author.Handle)
	}
	return strings.TrimSpace(strings.NewReplacer(r...).Replace(tmpl))
}

// cardTags builds the description, OpenGraph and Twitter tags every card
// shares.
func cardTags(u *url.URL, ogType, title, description, image, site string) []MetaTag {
	tags := []MetaTag{
		// Basic meta
		{Type: "name", Key: "description", Content: description},

		// Facebook Meta Tags
		{Type: "property", Key: "og:url", Content: u.String()},
		{Type: "property", Key: "og:type", Content: ogType},
		{Type: "property", Key: "og:site_name", Content: site},
		{Type: "property", Key: "og:title", Content: title},
		{Type: "property", Key: "og:description", Content: description},
		{Type: "property", Key: "og:image", Content: image},

		// Twitter Meta Tags
		{Type: "name", Key: "twitter:card", Content: "summary_large_image"},
		{Type: "property", Key: "twitter:domain", Content: u.Host},
		{Type: "property", Key: "twitter:url", Content: u.String()},
		{Type: "name", Key: "twitter:title", Content: title},
		{Type: "name", Key: "twitter:description", Content: description},
		{Type: "name", Key: "twitter:image", Content: image},
	}
	return tags
}

// GenerateStreamerCard is the card for a stream page: "<name> is live on
// <site>" (cardLiveTitle) over the stream's post text, with the streamer's
// profile card as the image.
func (l *Linker) GenerateStreamerCard(ctx context.Context, u *url.URL, lsv *placestream.Livestream_LivestreamView, sentryDSN string) ([]byte, error) {
	if u == nil {
		return nil, errors.New("url is nil")
	}
	if lsv == nil {
		return nil, errors.New("livestream view is nil")
	}
	ls, ok := lsv.Record.Val.(*placestream.Livestream)
	if !ok {
		return nil, errors.New("livestream view is not a livestream")
	}

	thumbURL, _ := url.Parse(u.String())
	thumbURL.Path = "/xrpc/place.stream.live.getProfileCard"
	thumbURL.RawQuery = fmt.Sprintf("id=%s", lsv.Author.Did)

	brandMetas, values := l.brandMetas(ctx)
	title := expandCard(cardText(values, "cardLiveTitle"), values, &lsv.Author)
	metaTags := cardTags(u, "website", title, ls.Title, thumbURL.String(), siteTitle(values))
	metaTags = append(metaTags, brandMetas...)

	// at-tags: this page canonically maps to the livestream record, authored
	// by the streamer
	metaTags = append(metaTags, l.atTags(lsv.Uri, lsv.Author.Did)...)

	return l.GenerateHTML(ctx, &PageConfig{
		Title:     title,
		Metas:     metaTags,
		SentryDSN: sentryDSN,
	})
}

// GenerateVideoCard is the card for a video page: "<name>'s video on <site>"
// (cardVideoTitle) over the video's own title.
func (l *Linker) GenerateVideoCard(ctx context.Context, u *url.URL, vv *placestream.MediaGetVideo_VideoView, sentryDSN string) ([]byte, error) {
	if u == nil {
		return nil, errors.New("url is nil")
	}
	if vv == nil {
		return nil, errors.New("video view is nil")
	}
	video, ok := vv.Record.Val.(*placestream.Video)
	if !ok {
		return nil, errors.New("video view record is not a video")
	}

	authorDid := vv.Author.Did

	// og:image is the VOD's own thumbnail, served through the Bluesky image
	// CDN (which fetches the blob from the author's PDS — the same trick used
	// for game cover art). When the video has no thumbnail we fall back to the
	// author's generated profile card so the link still carries an image.
	var imageURL string
	if video.Thumb != nil && authorDid != "" {
		imageURL = fmt.Sprintf(
			"https://cdn.bsky.app/img/feed_thumbnail/plain/%s/%s@jpeg",
			authorDid, video.Thumb.Ref.String(),
		)
	} else {
		cardURL, _ := url.Parse(u.String())
		cardURL.Path = "/xrpc/place.stream.live.getProfileCard"
		cardURL.RawQuery = fmt.Sprintf("id=%s", authorDid)
		imageURL = cardURL.String()
	}

	brandMetas, values := l.brandMetas(ctx)
	title := expandCard(cardText(values, "cardVideoTitle"), values, &vv.Author)
	// The description is the video's title, the post text it was published
	// with; a video without one falls back to its description.
	description := strings.TrimSpace(video.Title)
	if description == "" && video.Description != nil {
		description = strings.TrimSpace(*video.Description)
	}
	metaTags := cardTags(u, "video.other", title, description, imageURL, siteTitle(values))
	metaTags = append(metaTags, brandMetas...)

	// at-tags: this page canonically maps to the place.stream.video record,
	// authored by the streamer
	metaTags = append(metaTags, l.atTags(vv.Uri, authorDid)...)

	return l.GenerateHTML(ctx, &PageConfig{
		Title:     title,
		Metas:     metaTags,
		SentryDSN: sentryDSN,
	})
}

// LandingPage names the app's top-level tabs, each with its own link card.
type LandingPage int

const (
	// LandingDefault is any page without a card of its own: the node's
	// siteTitle and siteDescription.
	LandingDefault LandingPage = iota
	// LandingHome is the front page, the live tab (/): cardHomeTitle and
	// cardHomeDescription, falling back to the site's own.
	LandingHome
	// LandingVideos is the on-demand tab (/video).
	LandingVideos
	// LandingGoLive is the go-live tab (/live, /go-live).
	LandingGoLive
)

// LandingPageFor maps a request path to the tab it shows, LandingDefault for
// anything else.
func LandingPageFor(path string) LandingPage {
	switch strings.Trim(path, "/") {
	case "":
		return LandingHome
	case "video":
		return LandingVideos
	case "live", "go-live":
		return LandingGoLive
	}
	return LandingDefault
}

// GenerateDefaultCard is the card for a page without one of its own: the
// node's siteTitle and siteDescription with its link banner.
func (l *Linker) GenerateDefaultCard(ctx context.Context, u *url.URL, sentryDSN string) ([]byte, error) {
	return l.GenerateLandingCard(ctx, u, LandingDefault, sentryDSN)
}

// GenerateLandingCard is the card for one of the app's tabs: its own title
// and description from the card templates (cardHomeTitle, cardVideosTitle,
// cardGoLiveTitle and their descriptions), the node's link banner as the
// image.
func (l *Linker) GenerateLandingCard(ctx context.Context, u *url.URL, page LandingPage, sentryDSN string) ([]byte, error) {
	if u == nil {
		return nil, errors.New("url is nil")
	}

	// /linkbanner.png serves the uploaded linkBanner asset when there is one
	// and the bundled brand banner otherwise.
	thumbURL, _ := url.Parse(u.String())
	thumbURL.Path = "/linkbanner.png"

	brandMetas, values := l.brandMetas(ctx)
	title := siteTitle(values)
	description := values["siteDescription"]
	if description == "" {
		description = defaultSiteDescription
	}
	switch page {
	case LandingHome:
		// The front page falls back to the site's own title and description;
		// a node phrases its live tab ("Live streams on W") by setting the
		// home templates.
		if t := expandCard(cardText(values, "cardHomeTitle"), values, nil); t != "" {
			title = t
		}
		if d := expandCard(cardText(values, "cardHomeDescription"), values, nil); d != "" {
			description = d
		}
	case LandingVideos:
		title = expandCard(cardText(values, "cardVideosTitle"), values, nil)
		description = expandCard(cardText(values, "cardVideosDescription"), values, nil)
	case LandingGoLive:
		title = expandCard(cardText(values, "cardGoLiveTitle"), values, nil)
		description = expandCard(cardText(values, "cardGoLiveDescription"), values, nil)
	}

	metaTags := cardTags(u, "website", title, description, thumbURL.String(), siteTitle(values))
	metaTags = append(metaTags, brandMetas...)

	// at-tags: the site itself is identified by this node's did:web; there's
	// no single author or canonical record for the front page
	metaTags = append(metaTags, l.atTags("", "")...)

	return l.GenerateHTML(ctx, &PageConfig{
		Title:     title,
		Metas:     metaTags,
		SentryDSN: sentryDSN,
	})
}

// isLinkPreviewMeta reports whether a <meta> is one the cards own: the page
// description and every og: / twitter: tag.
func isLinkPreviewMeta(node *html.Node) bool {
	for _, attr := range node.Attr {
		switch attr.Key {
		case "property":
			if strings.HasPrefix(attr.Val, "og:") || strings.HasPrefix(attr.Val, "twitter:") {
				return true
			}
		case "name":
			if attr.Val == "description" || strings.HasPrefix(attr.Val, "twitter:") {
				return true
			}
		}
	}
	return false
}

func (l *Linker) GenerateHTML(ctx context.Context, pc *PageConfig) ([]byte, error) {

	root, err := html.Parse(bytes.NewReader(l.BaseHTML))
	if err != nil {
		return nil, err
	}

	var htmlNode *html.Node
	for node := range root.ChildNodes() {
		if node.Type == html.ElementNode && node.Data == "html" {
			htmlNode = node
			break
		}
	}
	if htmlNode == nil {
		return nil, errors.New("html not found")
	}

	var head *html.Node
	for node := range htmlNode.ChildNodes() {
		if node.Data == "head" {
			head = node
			break
		}
	}
	if head == nil {
		return nil, errors.New("head not found")
	}

	// The template ships its own title, description and link-preview tags
	// (the first-party brand, for a static host). Every card replaces them,
	// and crawlers honour the first tag they meet, so the template's go.
	// A node that is its own product can also drop the template's iOS Smart
	// App Banner (branding key mobileAppBanner=off).
	dropAppBanner := l.brandingText("mobileAppBanner") == "off"
	var stale []*html.Node
	for node := range head.ChildNodes() {
		if node.Type != html.ElementNode {
			continue
		}
		if node.Data == "title" || (node.Data == "meta" && isLinkPreviewMeta(node)) {
			stale = append(stale, node)
		}
		if dropAppBanner && node.Data == "meta" && isAppBannerMeta(node) {
			stale = append(stale, node)
		}
	}
	for _, node := range stale {
		head.RemoveChild(node)
	}

	title := &html.Node{
		Type: html.ElementNode,
		Data: "title",
	}
	head.AppendChild(title)
	title.AppendChild(&html.Node{
		Type: html.TextNode,
		Data: pc.Title,
	})

	// Add all meta tags in a loop
	for _, tag := range pc.Metas {
		head.AppendChild(&html.Node{
			Type: html.ElementNode,
			Data: "meta",
			Attr: []html.Attribute{
				{Key: tag.Type, Val: tag.Key},
				{Key: "content", Val: tag.Content},
			},
		})
	}

	// Paint the branded background before any script runs: on the root
	// element too, since that is what mobile browsers show behind their
	// translucent bars and in overscroll, and as theme-color for the bars
	// themselves.
	if bg := l.bodyBackground(); bg != "" {
		style := &html.Node{Type: html.ElementNode, Data: "style"}
		head.AppendChild(style)
		style.AppendChild(&html.Node{Type: html.TextNode, Data: "html,body{background-color:" + bg + "}"})
		head.AppendChild(&html.Node{
			Type: html.ElementNode,
			Data: "meta",
			Attr: []html.Attribute{
				{Key: "name", Val: "theme-color"},
				{Key: "content", Val: bg},
			},
		})
	}

	// Add Sentry DSN script if configured
	if pc.SentryDSN != "" {
		script := &html.Node{
			Type: html.ElementNode,
			Data: "script",
		}
		head.AppendChild(script)
		script.AppendChild(&html.Node{
			Type: html.TextNode,
			Data: `window.SENTRY_DSN = "` + pc.SentryDSN + `";`,
		})
	}

	// Render the HTML to a string
	var buf bytes.Buffer
	if err := html.Render(&buf, root); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
