package linking

import (
	"bytes"
	"context"
	"net/url"
	"testing"

	glex "github.com/streamplace/glex/runtime"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/placestream"
)

func TestInvalidHandleLinkCard(t *testing.T) {
	for _, tc := range []struct{ did, handle string }{
		{"did:plc:2zmxikig2sj7gqaezl5gntae", "handle.invalid"},
		{"did:plc:2zmxikig2sj7gqaezl5gntae", ""},
		{"did:web:example.com%3A8443", "handle.invalid"},
	} {
		did, handle := tc.did, tc.handle
		t.Run(did+"/"+handle, func(t *testing.T) {
			linker, err := NewLinker(context.Background(), []byte("<!doctype html><html><head><title>Streamplace</title></head><body></body></html>"), nil, nil)
			require.NoError(t, err)
			u, err := url.Parse("https://stream.place/" + url.PathEscape(did))
			require.NoError(t, err)
			card, err := linker.GenerateStreamerCard(context.Background(), u, &placestream.Livestream_LivestreamView{
				Author: appbsky.ActorDefs_ProfileViewBasic{Did: did, Handle: handle},
				Record: &glex.LexiconTypeDecoder{Val: &placestream.Livestream{Title: "Live stream"}},
			}, "")
			require.NoError(t, err)
			require.NotContains(t, string(card), "handle.invalid")
			require.Contains(t, string(card), "<title>"+did)
			document, err := html.Parse(bytes.NewReader(card))
			require.NoError(t, err)
			foundImage := false
			for node := range document.Descendants() {
				var property, content string
				for _, attr := range node.Attr {
					switch attr.Key {
					case "property":
						property = attr.Val
					case "content":
						content = attr.Val
					}
				}
				if node.Data != "meta" || property != "og:image" {
					continue
				}
				imageURL, err := url.Parse(content)
				require.NoError(t, err)
				require.Equal(t, did, imageURL.Query().Get("id"), "thumbnail requests preserve the complete DID")
				foundImage = true
			}
			require.True(t, foundImage)
		})
	}
}
