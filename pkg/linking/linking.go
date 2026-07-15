package linking

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"

	"golang.org/x/net/html"
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

var BrandingAssetList = [...]string{
	"siteTitle",
	"siteDescription",
	"primaryColor",
	"accentColor",
	"defaultStreamer",
	"mainLogo",
	"favicon",
	"sidebarBg",
	"legalLinks",
}

// fetch branding assets for a given broadcaster DID
func (l *Linker) getBrandingAssets(broadcasterDid string) ([]placestream.BrandingGetBranding_BrandingAsset, error) {
	ret := make([]placestream.BrandingGetBranding_BrandingAsset, 0)
	for _, asset := range BrandingAssetList {
		blob, err := l.sdb.GetBrandingBlob(broadcasterDid, asset)
		if err != nil {
			// this can probably include a 'record not found' error, in which case we skip
			log.Printf("error fetching branding asset %s for broadcaster %s: %v", asset, broadcasterDid, err)
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
		}
		ret = append(ret, asset)
	}

	return ret, nil
}

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

	titleStr := fmt.Sprintf("@%s's livestream on ", lsv.Author.Handle)
	outURL := u.String()

	thumbURL, _ := url.Parse(u.String())
	thumbURL.Path = "/xrpc/place.stream.live.getProfileCard"
	thumbURL.RawQuery = fmt.Sprintf("id=%s", lsv.Author.Did)

	// Define all meta tags
	metaTags := []MetaTag{
		// Basic meta
		{Type: "name", Key: "description", Content: ls.Title},

		// Facebook Meta Tags
		{Type: "property", Key: "og:url", Content: u.String()},
		{Type: "property", Key: "og:type", Content: "website"},
		{Type: "property", Key: "og:description", Content: ls.Title},
		{Type: "property", Key: "og:image", Content: thumbURL.String()},

		// Twitter Meta Tags
		{Type: "name", Key: "twitter:card", Content: "summary_large_image"},
		{Type: "property", Key: "twitter:domain", Content: u.Host},
		{Type: "property", Key: "twitter:url", Content: outURL},
		{Type: "name", Key: "twitter:description", Content: ls.Title},
		{Type: "name", Key: "twitter:image", Content: thumbURL.String()},
	}
	brandingTitle := "streamplace node"
	if l.sdb != nil && l.cli != nil {
		branding, err := l.getBrandingAssets("did:web:" + l.cli.BroadcasterHost)
		if err == nil {
			for i := range branding {
				val := branding[i]
				if val.Key == "siteTitle" && val.Data != nil {
					brandingTitle = *val.Data
				}
				marshalledJson, err := json.Marshal(val)
				if err != nil {
					fmt.Printf("error marshalling branding asset %s: %v\n", val.Key, err)
					continue
				}
				metaTags = append(metaTags, MetaTag{
					Type:    "name",
					Key:     "internal-brand:" + val.Key,
					Content: string(marshalledJson),
				})
			}
		} else {
			// log but we should not block rendering
			fmt.Printf("error fetching branding assets: %v\n", err)
		}
	}

	// do twitter/og title after
	metaTags = append(metaTags, MetaTag{
		Type:    "property",
		Key:     "og:title",
		Content: fmt.Sprintf("%s%s", titleStr, brandingTitle),
	})
	metaTags = append(metaTags, MetaTag{
		Type:    "name",
		Key:     "twitter:title",
		Content: fmt.Sprintf("%s%s", titleStr, brandingTitle),
	})

	return l.GenerateHTML(ctx, &PageConfig{
		Title:     fmt.Sprintf("%s%s", titleStr, brandingTitle),
		Metas:     metaTags,
		SentryDSN: sentryDSN,
	})
}

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

	outURL := u.String()

	authorDid := ""
	authorHandle := ""
	if !false {
		authorDid = vv.Author.Did
		authorHandle = vv.Author.Handle
	}

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

	// Define all meta tags. The title/description pair is appended after we
	// resolve the node's branding title below: the card headline is the
	// video's own title, and the author + node go in the description.
	metaTags := []MetaTag{
		// Facebook Meta Tags
		{Type: "property", Key: "og:url", Content: outURL},
		{Type: "property", Key: "og:type", Content: "video.other"},
		{Type: "property", Key: "og:image", Content: imageURL},

		// Twitter Meta Tags
		{Type: "name", Key: "twitter:card", Content: "summary_large_image"},
		{Type: "property", Key: "twitter:domain", Content: u.Host},
		{Type: "property", Key: "twitter:url", Content: outURL},
		{Type: "name", Key: "twitter:image", Content: imageURL},
	}

	brandingTitle := "streamplace node"
	if l.sdb != nil && l.cli != nil {
		branding, err := l.getBrandingAssets("did:web:" + l.cli.BroadcasterHost)
		if err == nil {
			for i := range branding {
				val := branding[i]
				if val.Key == "siteTitle" && val.Data != nil {
					brandingTitle = *val.Data
				}
				marshalledJson, err := json.Marshal(val)
				if err != nil {
					fmt.Printf("error marshalling branding asset %s: %v\n", val.Key, err)
					continue
				}
				metaTags = append(metaTags, MetaTag{
					Type:    "name",
					Key:     "internal-brand:" + val.Key,
					Content: string(marshalledJson),
				})
			}
		} else {
			// log but we should not block rendering
			fmt.Printf("error fetching branding assets: %v\n", err)
		}
	}

	// The card headline is the video's own title; the author + node name go
	// in the description. Appended after the branding loop so the description
	// can reference the resolved branding title.
	title := video.Title
	description := fmt.Sprintf("@%s's video on %s", authorHandle, brandingTitle)
	metaTags = append(metaTags,
		MetaTag{Type: "name", Key: "description", Content: description},
		MetaTag{Type: "property", Key: "og:description", Content: description},
		MetaTag{Type: "property", Key: "og:title", Content: title},
		MetaTag{Type: "name", Key: "twitter:description", Content: description},
		MetaTag{Type: "name", Key: "twitter:title", Content: title},
	)

	return l.GenerateHTML(ctx, &PageConfig{
		Title:     title,
		Metas:     metaTags,
		SentryDSN: sentryDSN,
	})
}

func (l *Linker) GenerateDefaultCard(ctx context.Context, u *url.URL, sentryDSN string) ([]byte, error) {
	if u == nil {
		return nil, errors.New("url is nil")
	}

	thumbURL, _ := url.Parse(u.String())
	thumbURL.Path = "/linkbanner.png"

	// Define all meta tags
	metaTags := []MetaTag{
		// Basic meta
		{Type: "name", Key: "description", Content: "Stream.place is open-source livestreaming on the AT Protocol."},

		// Facebook Meta Tags
		{Type: "property", Key: "og:url", Content: u.String()},
		{Type: "property", Key: "og:type", Content: "website"},
		{Type: "property", Key: "og:title", Content: "Stream.place"},
		{Type: "property", Key: "og:description", Content: "Open-source livestreaming on the AT Protocol."},
		{Type: "property", Key: "og:image", Content: thumbURL.String()},

		// Twitter Meta Tags
		{Type: "name", Key: "twitter:card", Content: "summary_large_image"},
		{Type: "property", Key: "twitter:domain", Content: u.Host},
		{Type: "property", Key: "twitter:url", Content: u.String()},
		{Type: "name", Key: "twitter:title", Content: "Stream.place"},
		{Type: "name", Key: "twitter:description", Content: "Open-source livestreaming on the AT Protocol."},
		{Type: "name", Key: "twitter:image", Content: thumbURL.String()},
	}

	brandingTitle := "streamplace node"
	if l.sdb != nil && l.cli != nil {
		branding, err := l.getBrandingAssets("did:web:" + l.cli.BroadcasterHost)
		if err == nil {
			for i := range branding {
				val := branding[i]
				if val.Key == "siteTitle" && val.Data != nil {
					brandingTitle = *val.Data
				}
				marshalledJson, err := json.Marshal(val)
				if err != nil {
					fmt.Printf("error marshalling branding asset %s: %v\n", val.Key, err)
					continue
				}
				metaTags = append(metaTags, MetaTag{
					Type:    "name",
					Key:     "internal-brand:" + val.Key,
					Content: string(marshalledJson),
				})
			}
		} else {
			// log but we should not block rendering
			fmt.Printf("error fetching branding assets: %v\n", err)
		}
	}

	// do twitter/og title after
	metaTags = append(metaTags, MetaTag{
		Type:    "property",
		Key:     "og:title",
		Content: brandingTitle,
	})
	metaTags = append(metaTags, MetaTag{
		Type:    "name",
		Key:     "twitter:title",
		Content: brandingTitle,
	})

	return l.GenerateHTML(ctx, &PageConfig{
		Title:     brandingTitle,
		Metas:     metaTags,
		SentryDSN: sentryDSN,
	})
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

	// Title tag (handled separately as it's not a meta tag)

	var oldTitle *html.Node
	for node := range head.ChildNodes() {
		if node.Type == html.ElementNode && node.Data == "title" {
			oldTitle = node
			break
		}
	}
	if oldTitle != nil {
		head.RemoveChild(oldTitle)
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
