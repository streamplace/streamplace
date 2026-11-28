package spxrpc

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/vod"
)

func TestVODStarterStore(t *testing.T) {
	ctx := context.Background()
	store, err := blob.NewFileStore(t.TempDir())
	require.NoError(t, err)
	put := func(key string, data []byte) {
		writer, err := store.NewWriter(ctx, key, "application/octet-stream")
		require.NoError(t, err)
		_, err = writer.Write(data)
		require.NoError(t, err)
		require.NoError(t, writer.Complete())
	}
	box := func(typ string, payload ...[]byte) []byte {
		size := 8
		for _, part := range payload {
			size += len(part)
		}
		out := append(binary.BigEndian.AppendUint32(nil, uint32(size)), typ...)
		for _, part := range payload {
			out = append(out, part...)
		}
		return out
	}
	// Defaults: duration=1000, size=1; sample 0 is sync, sample 1 isn't.
	tfhd := box("tfhd", []byte{0, 2, 0, 0x38, 0, 0, 0, 1, 0, 0, 3, 0xe8, 0, 0, 0, 1, 1, 1, 0, 0})
	tfdt := box("tfdt", make([]byte, 8))
	trun := box("trun", []byte{0, 0, 0, 4, 0, 0, 0, 2, 2, 0, 0, 0})
	fragment := append(box("moof", box("mfhd", make([]byte, 8)), box("traf", tfhd, tfdt, trun)), box("mdat", []byte{1, 2})...)
	content := append(make([]byte, 42), fragment...)
	audioOffset := int64(len(content))
	content = append(content, []byte("audio")...)
	put(vod.BlobsPrefix+"content.mp4", content)
	meta := &vod.Metafile{FlatHeaderSize: 42, Tracks: map[string]vod.MetafileTrack{
		"1": {Type: "video", BlobCID: "content", Timescale: 1000, Segments: []vod.MetafileSegment{{Offset: 0, Size: int64(len(fragment)), DurationTicks: 2000}}},
		"2": {Type: "audio", Codec: "mp4a.40.2", BlobCID: "content", Timescale: 1000, Segments: []vod.MetafileSegment{{Offset: audioOffset - 42, Size: 5, DurationTicks: 2000}}},
	}}
	s := &Server{playbackStore: store}
	parts := &vodParts{meta: meta}
	starter, err := s.loadStarter(ctx, parts)
	require.NoError(t, err)
	require.NotNil(t, starter.Video.Key)
	require.NotNil(t, starter.Video.Rest)
	require.Equal(t, "2", starter.Audio.Track)
	audio, err := base64.StdEncoding.DecodeString(*starter.Audio.Data)
	require.NoError(t, err)
	require.Equal(t, []byte("audio"), audio)
	again, err := s.loadStarter(ctx, parts)
	require.NoError(t, err)
	require.Same(t, starter, again)

	for i := range vodPartsCacheSize + 1 {
		data, err := json.Marshal(meta)
		require.NoError(t, err)
		cid := fmt.Sprintf("meta%d", i)
		put(vod.BlobsPrefix+cid+".json", data)
		_, err = s.videoParts(ctx, cid)
		require.NoError(t, err)
	}
	require.Len(t, s.vodCache.entries, vodPartsCacheSize)
	require.NotContains(t, s.vodCache.entries, "meta0")
	first, err := s.videoParts(ctx, "meta1")
	require.NoError(t, err)
	againParts, err := s.videoParts(ctx, "meta1")
	require.NoError(t, err)
	require.Same(t, first, againParts)
}
