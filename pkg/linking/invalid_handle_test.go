package linking

import (
	"context"
	"net/url"
	"testing"

	glex "github.com/streamplace/glex/runtime"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/placestream"
)

func TestInvalidHandleLinkCard(t *testing.T) {
	for _, tc := range []struct{ did, handle string }{
		{"did:plc:2zmxikig2sj7gqaezl5gntae", "handle.invalid"},
		{"did:plc:2zmxikig2sj7gqaezl5gntae", ""},
	} {
		did, handle := tc.did, tc.handle
		t.Run(did+"/"+handle, func(t *testing.T) {
			linker, err := NewLinker(context.Background(), []byte("<!doctype html><html><head><title>Streamplace</title></head><body></body></html>"), nil, nil)
			require.NoError(t, err)
			u, err := url.Parse("https://stream.place/" + did)
			require.NoError(t, err)
			card, err := linker.GenerateStreamerCard(context.Background(), u, &placestream.Livestream_LivestreamView{
				Author: appbsky.ActorDefs_ProfileViewBasic{Did: did, Handle: handle},
				Record: &glex.LexiconTypeDecoder{Val: &placestream.Livestream{Title: "Live stream"}},
			}, "")
			require.NoError(t, err)
			require.NotContains(t, string(card), "handle.invalid")
			require.Contains(t, string(card), "<title>"+did)
		})
	}
}
