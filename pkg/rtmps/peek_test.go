package rtmps

import (
	"bytes"
	"testing"

	"github.com/bluenviron/gortmplib/pkg/amf0"
	"github.com/bluenviron/gortmplib/pkg/bytecounter"
	"github.com/bluenviron/gortmplib/pkg/message"
	"github.com/stretchr/testify/require"
)

// clientStream is the client-to-server bytes of an RTMP session that sends
// msgs after a plain handshake.
func clientStream(t *testing.T, msgs ...message.Message) []byte {
	var buf bytes.Buffer
	hs := make([]byte, rtmpHandshakeClientBytes)
	hs[0] = 3
	buf.Write(hs)
	w := message.NewWriter(&buf, bytecounter.NewWriter(&buf), false)
	for _, msg := range msgs {
		require.NoError(t, w.Write(msg))
	}
	return buf.Bytes()
}

func connectCmd(tcURL string) *message.CommandAMF0 {
	return &message.CommandAMF0{
		ChunkStreamID: 3,
		Name:          "connect",
		CommandID:     1,
		Arguments: []any{
			amf0.Object{
				{Key: "app", Value: "live"},
				{Key: "tcUrl", Value: tcURL},
			},
		},
	}
}

func streamCmd(name, streamName string) *message.CommandAMF0 {
	return &message.CommandAMF0{
		ChunkStreamID:   8,
		MessageStreamID: 1,
		Name:            name,
		CommandID:       3,
		Arguments:       []any{nil, streamName, "live"},
	}
}

func TestPeekPublish(t *testing.T) {
	stream := clientStream(t,
		// larger chunks than the default, as OBS and gortmplib send
		&message.SetChunkSize{Value: 65536},
		connectCmd("rtmps://Stream.Place:1935/live"),
		&message.CommandAMF0{ChunkStreamID: 3, Name: "createStream", CommandID: 2, Arguments: []any{nil}},
		streamCmd("publish", "thekey?foo=bar"),
	)
	info, err := PeekPublish(bytes.NewReader(stream))
	require.NoError(t, err)
	require.Equal(t, "rtmps://Stream.Place:1935/live", info.TCURL)
	require.Equal(t, "thekey", info.StreamKey)
}

func TestPeekPublishRejectsPlayback(t *testing.T) {
	stream := clientStream(t, connectCmd("rtmp://example.com/live"), streamCmd("play", "thekey"))
	_, err := PeekPublish(bytes.NewReader(stream))
	require.ErrorContains(t, err, "playing")
}

func TestPeekPublishRejectsEncryptedHandshake(t *testing.T) {
	hs := make([]byte, rtmpHandshakeClientBytes)
	hs[0] = 6 // RTMPE
	_, err := PeekPublish(bytes.NewReader(hs))
	require.ErrorContains(t, err, "handshake version")
}

func TestPeekPublishTruncated(t *testing.T) {
	stream := clientStream(t, connectCmd("rtmp://example.com/live"))
	_, err := PeekPublish(bytes.NewReader(stream))
	require.Error(t, err)
}
