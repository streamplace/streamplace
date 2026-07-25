package spxrpc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/placestream"
)

// TestStartLivestreamInputBlobShapes is the regression test for the prod
// "parsing CID in legacy blob: invalid cid: cid too short" failures on
// place.stream.live.startLivestream. The JS app uploads its thumbnail with
// @atproto/api (getting that library's BlobRef class) and sends the record
// through the @atproto/lex client, whose lexStringify doesn't recognize the
// foreign BlobRef and serializes it field-by-field — producing a modern blob
// WITHOUT $type. That shape (and an explicit null thumb) must decode.
func TestStartLivestreamInputBlobShapes(t *testing.T) {
	// Verbatim shape lexStringify produces for an @atproto/api BlobRef.
	body := `{"streamer":"did:plc:x","livestream":{"$type":"place.stream.livestream","title":"t","createdAt":"2026-07-16T00:00:00Z","thumb":{"ref":{"$link":"bafkreib2rxk3rybk3aobmv5cjuql3bm2twh4jo5uxyjfxzvjcamdmc76jm"},"mimeType":"image/jpeg","size":4096,"original":{"$type":"blob","ref":{"$link":"bafkreib2rxk3rybk3aobmv5cjuql3bm2twh4jo5uxyjfxzvjcamdmc76jm"},"mimeType":"image/jpeg","size":4096}}}}`
	var in placestream.LiveStartLivestream_Input
	require.NoError(t, json.Unmarshal([]byte(body), &in), "$type-less BlobRef-shaped thumb must decode")
	require.NotNil(t, in.Livestream.Thumb)
	require.Equal(t, int64(4096), in.Livestream.Thumb.Size)
	require.Equal(t, "image/jpeg", in.Livestream.Thumb.MimeType)
	require.Equal(t, "bafkreib2rxk3rybk3aobmv5cjuql3bm2twh4jo5uxyjfxzvjcamdmc76jm", in.Livestream.Thumb.Ref.String())

	// An explicit null thumb must decode as absent, not error.
	var in2 placestream.LiveStartLivestream_Input
	require.NoError(t, json.Unmarshal([]byte(`{"streamer":"did:plc:x","livestream":{"title":"t","createdAt":"2026-07-16T00:00:00Z","thumb":null}}`), &in2))
	require.Nil(t, in2.Livestream.Thumb)
}
