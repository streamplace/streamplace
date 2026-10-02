package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bluenviron/gortmplib/pkg/h264conf"
	"github.com/bluenviron/gortmplib/pkg/message"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/media"
)

func TestH264ConfigMessage(t *testing.T) {
	sps := []byte{0x67, 0x42, 0xc0, 0x15, 0xda, 0x06, 0xc7, 0xf9}
	pps := []byte{0x68, 0xce, 0x3c, 0x80}
	idr := []byte{0x65, 0x88, 0x84}
	slice := []byte{0x41, 0x9a}

	msg, ok := h264ConfigMessage([][]byte{sps, pps}, 40*time.Millisecond)
	require.True(t, ok)
	require.EqualValues(t, message.VideoTypeConfig, msg.Type)
	require.EqualValues(t, message.CodecH264, msg.Codec)
	require.Equal(t, 40*time.Millisecond, msg.DTS)
	// it round-trips through the reader's own parsing
	var conf h264conf.Conf
	require.NoError(t, conf.Unmarshal(msg.Payload))
	require.Equal(t, sps, conf.SPS)
	require.Equal(t, pps, conf.PPS)

	for name, au := range map[string][][]byte{
		"keyframe with parameter sets": {sps, pps, idr},
		"slice":                        {slice},
		"SPS alone":                    {sps},
		"two SPS":                      {sps, sps, pps},
		"empty NALU":                   {sps, {}, pps},
		"empty AU":                     nil,
	} {
		_, ok := h264ConfigMessage(au, 0)
		require.False(t, ok, name)
	}
}

// recordingConn is a gortmplib.Conn that keeps what's written to it.
type recordingConn struct {
	msgs []message.Message
}

func (c *recordingConn) BytesReceived() uint64          { return 0 }
func (c *recordingConn) BytesSent() uint64              { return 0 }
func (c *recordingConn) Read() (message.Message, error) { return nil, errors.New("write-only") }
func (c *recordingConn) Write(msg message.Message) error {
	c.msgs = append(c.msgs, msg)
	return nil
}

// What the relay writes for a stream's start as gortmplib's reader hands it
// on: the replayed sequence header as an SPS/PPS access unit, then the first
// keyframe at the same timestamp. The header has to go out as a sequence
// header, not a frame of its own, so the keyframe is the only frame at that
// timestamp.
func TestRelayRTMPSessionSequenceHeader(t *testing.T) {
	sps := []byte{0x67, 0x42, 0xc0, 0x15, 0xda, 0x06, 0xc7, 0xf9}
	pps := []byte{0x68, 0xce, 0x3c, 0x80}
	idr := []byte{0x65, 0x88, 0x84, 0x00}
	slice := []byte{0x41, 0x9a, 0x02}

	session := &media.RTMPSession{
		EventChan:  make(chan any, 8),
		VideoTrack: &format.H264{PayloadTyp: 96, SPS: sps, PPS: pps, PacketizationMode: 1},
	}
	session.EventChan <- &media.RTMPH264Data{AU: [][]byte{sps, pps}}
	session.EventChan <- &media.RTMPH264Data{AU: [][]byte{idr}}
	session.EventChan <- &media.RTMPH264Data{AU: [][]byte{slice}, PTS: 33 * time.Millisecond, DTS: 33 * time.Millisecond}
	close(session.EventChan)

	conn := &recordingConn{}
	err := relayRTMPSession(context.Background(), conn, session)
	require.ErrorContains(t, err, "session closed")

	var video []*message.Video
	for _, msg := range conn.msgs {
		if v, ok := msg.(*message.Video); ok {
			video = append(video, v)
		}
	}
	// the writer's own header for the track, the relayed one, then frames
	require.Len(t, video, 4)
	for i, want := range []struct {
		typ message.VideoType
		dts time.Duration
		key bool
	}{
		{message.VideoTypeConfig, 0, true},
		{message.VideoTypeConfig, 0, true},
		{message.VideoTypeAU, 0, true},
		{message.VideoTypeAU, 33 * time.Millisecond, false},
	} {
		require.Equal(t, want.typ, video[i].Type, "message %d", i)
		require.Equal(t, want.dts, video[i].DTS, "message %d", i)
		require.Equal(t, want.key, video[i].IsKeyFrame, "message %d", i)
	}
	var conf h264conf.Conf
	require.NoError(t, conf.Unmarshal(video[1].Payload))
	require.Equal(t, sps, conf.SPS)
	require.Equal(t, pps, conf.PPS)
}
