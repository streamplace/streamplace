package moq

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	official "moq.dev/moq"
	"stream.place/streamplace/pkg/muxl"
)

func TestSplitRenditions(t *testing.T) {
	ctx := context.Background()
	fixture, err := os.ReadFile("../../test/fixtures/h264-opus-frag.mp4")
	require.NoError(t, err)
	canonical, err := muxl.RunMuxlCanonicalize(ctx, fixture, nil)
	require.NoError(t, err)
	pieces, err := SplitRenditions(ctx, canonical)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(pieces), 2)
	for i, piece := range pieces {
		require.NotEmpty(t, piece.Payload)
		if i > 0 {
			require.Less(t, pieces[i-1].ID, piece.ID)
		}
		again, err := SplitRenditions(ctx, piece.Payload)
		require.NoError(t, err)
		require.Len(t, again, 1)
		require.Equal(t, piece, again[0])
	}
	_, err = SplitRenditions(ctx, []byte("not a media container"))
	require.Error(t, err)
}

func TestRenditionTrackNames(t *testing.T) {
	require.Equal(t, "rendition-100", RenditionTrack(100))
	id, ok := ParseRenditionTrack("rendition-101")
	require.True(t, ok)
	require.Equal(t, uint32(101), id)
	_, ok = ParseRenditionTrack(SourceTrack)
	require.False(t, ok)
	_, ok = ParseRenditionTrack("rendition-x")
	require.False(t, ok)
	_, ok = ParseRenditionTrack("rendition-001")
	require.False(t, ok)
}

// The catalog section round-trips through the official catalog's
// sections, and a broadcast without one is simply not a Streamplace
// publisher rather than an error.
func TestCatalogRoundTrip(t *testing.T) {
	seq := uint64(7)
	in := Catalog{SourceSequence: &seq, Renditions: []Rendition{{Track: RenditionTrack(100), ID: 100}}}
	js, err := in.JSON()
	require.NoError(t, err)
	out, ok, err := ParseCatalog(&official.Catalog{Sections: map[string]string{CatalogSection: js}})
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, in, out)

	_, ok, err = ParseCatalog(&official.Catalog{})
	require.NoError(t, err)
	require.False(t, ok)
	_, ok, err = ParseCatalog(&official.Catalog{Sections: map[string]string{CatalogSection: "{"}})
	require.True(t, ok)
	require.Error(t, err)

}
