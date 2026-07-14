package spxrpc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFeedSkeletonRE(t *testing.T) {
	tests := []struct {
		feed string
		name string
		did  string
		want bool
	}{
		{
			feed: "at://did:plc:oio4hkxaop4ao4wz2pp3f4cr/app.bsky.feed.generator/atproto",
			did:  "did:plc:oio4hkxaop4ao4wz2pp3f4cr",
			name: "atproto",
			want: true,
		},
		{
			feed: "at://did:web:iame.li/app.bsky.feed.generator/feedwithnumbers123",
			did:  "did:web:iame.li",
			name: "feedwithnumbers123",
			want: true,
		},
		{
			feed: "at://did:web:iame.li/app.bsky.feed.generator/feed-with-dashes",
			did:  "did:web:iame.li",
			name: "feed-with-dashes",
			want: true,
		},
		{
			feed: "foo",
			want: false,
		},
		{
			feed: "at:///app.bsky.feed.generator/feedwithnumbers123",
			want: false,
		},
		{
			feed: "at://did:web:iame.li/app.bsky.feed.generator/",
			want: false,
		},
		{
			feed: "at://did:web:iame.li/app.bsky.feed.generator/feedwithnumbers123/errantsuffix",
			want: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			did, name, err := parseFeedSkeleton(test.feed)
			if !test.want {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.did, did)
			require.Equal(t, test.name, name)
		})
	}
}
