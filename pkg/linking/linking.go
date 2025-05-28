package linking

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"

	"golang.org/x/net/html"
	"stream.place/streamplace/pkg/streamplace"
)

type Linker struct {
	BaseHTML []byte
}

func NewLinker(ctx context.Context, baseHTML []byte) (*Linker, error) {
	_, err := html.Parse(bytes.NewReader(baseHTML))
	if err != nil {
		return nil, err
	}

	return &Linker{BaseHTML: baseHTML}, nil
}

type PageConfig struct {
	Title string
	Metas []MetaTag
}

// Define all meta tags in a structured way
type MetaTag struct {
	Type    string // "name" or "property"
	Key     string
	Content string
}

func (l *Linker) GenerateStreamerCard(ctx context.Context, u *url.URL, lsv *streamplace.Livestream_LivestreamView) ([]byte, error) {
	if u == nil {
		return nil, errors.New("url is nil")
	}
	if lsv == nil {
		return nil, errors.New("livestream view is nil")
	}
	ls, ok := lsv.Record.Val.(*streamplace.Livestream)
	if !ok {
		return nil, errors.New("livestream view is not a livestream")
	}

	titleStr := fmt.Sprintf("@%s is 🔴LIVE on %s!", lsv.Author.Handle, u.Host)
	outURL := u.String()

	pageTitle := fmt.Sprintf("@%s | %s", lsv.Author.Handle, u.Host)

	thumbURL, _ := url.Parse(u.String())
	thumbURL.Path = fmt.Sprintf("/api/playback/%s/stream.jpg", lsv.Author.Did)

	// Define all meta tags
	metaTags := []MetaTag{
		// Basic meta
		{Type: "name", Key: "description", Content: ls.Title},

		// Facebook Meta Tags
		{Type: "property", Key: "og:url", Content: u.String()},
		{Type: "property", Key: "og:type", Content: "website"},
		{Type: "property", Key: "og:title", Content: titleStr},
		{Type: "property", Key: "og:description", Content: ls.Title},
		{Type: "property", Key: "og:image", Content: thumbURL.String()},

		// Twitter Meta Tags
		{Type: "name", Key: "twitter:card", Content: "summary_large_image"},
		{Type: "property", Key: "twitter:domain", Content: u.Host},
		{Type: "property", Key: "twitter:url", Content: outURL},
		{Type: "name", Key: "twitter:title", Content: titleStr},
		{Type: "name", Key: "twitter:description", Content: ls.Title},
		{Type: "name", Key: "twitter:image", Content: thumbURL.String()},
	}

	return l.GenerateHTML(ctx, &PageConfig{
		Title: pageTitle,
		Metas: metaTags,
	})
}

func (l *Linker) GenerateDefaultCard(ctx context.Context, u *url.URL) ([]byte, error) {
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
		{Type: "property", Key: "og:description", Content: "Stream.place is open-source livestreaming on the AT Protocol."},
		{Type: "property", Key: "og:image", Content: thumbURL.String()},

		// Twitter Meta Tags
		{Type: "name", Key: "twitter:card", Content: "summary_large_image"},
		{Type: "property", Key: "twitter:domain", Content: u.Host},
		{Type: "property", Key: "twitter:url", Content: u.String()},
		{Type: "name", Key: "twitter:title", Content: "Stream.place"},
		{Type: "name", Key: "twitter:description", Content: "Stream.place is open-source livestreaming on the AT Protocol."},
		{Type: "name", Key: "twitter:image", Content: thumbURL.String()},
	}

	return l.GenerateHTML(ctx, &PageConfig{
		Title: "Stream.place",
		Metas: metaTags,
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

	// Render the HTML to a string
	var buf bytes.Buffer
	if err := html.Render(&buf, root); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
