package api

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bluenviron/gortmplib"
	"github.com/bluenviron/gortmplib/pkg/bytecounter"
	"github.com/bluenviron/gortmplib/pkg/codecs"
	"github.com/bluenviron/gortmplib/pkg/message"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/media"
)

// testSPS is a real 352x288 High profile SPS; gortmplib's writer parses it.
var testSPS = []byte{
	0x67, 0x64, 0x00, 0x0c, 0xac, 0x3b, 0x50, 0xb0,
	0x4b, 0x42, 0x00, 0x00, 0x03, 0x00, 0x02, 0x00,
	0x00, 0x03, 0x00, 0x3d, 0x08,
}

func TestH264ConfigMessage(t *testing.T) {
	sps := testSPS
	pps := []byte{0x68, 0xce, 0x3c, 0x80}
	idr := []byte{0x65, 0x88, 0x84}
	slice := []byte{0x41, 0x9a}

	msg, ok := h264ConfigMessage([][]byte{sps, pps}, 40*time.Millisecond)
	require.True(t, ok)
	require.Equal(t, message.VideoPacketTypeConfig, msg.PacketType)
	require.EqualValues(t, message.CodecH264, msg.Codec)
	require.Equal(t, 40*time.Millisecond, msg.DTS)
	requireAVCConfig(t, msg, sps, pps)

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
	sps := testSPS
	pps := []byte{0x68, 0xce, 0x3c, 0x80}
	idr := []byte{0x65, 0x88, 0x84, 0x00}
	slice := []byte{0x41, 0x9a, 0x02}

	session := &media.RTMPSession{
		EventChan:  make(chan any, 8),
		VideoTrack: &gortmplib.Track{Codec: &codecs.H264{SPS: sps, PPS: pps}},
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
		typ   message.VideoPacketType
		dts   time.Duration
		frame message.VideoFrameType
	}{
		{message.VideoPacketTypeConfig, 0, message.VideoFrameTypeKeyFrame},
		{message.VideoPacketTypeConfig, 0, message.VideoFrameTypeKeyFrame},
		{message.VideoPacketTypeAU, 0, message.VideoFrameTypeKeyFrame},
		{message.VideoPacketTypeAU, 33 * time.Millisecond, message.VideoFrameTypeInterFrame},
	} {
		require.Equal(t, want.typ, video[i].PacketType, "message %d", i)
		require.Equal(t, want.dts, video[i].DTS, "message %d", i)
		require.Equal(t, want.frame, video[i].FrameType, "message %d", i)
	}
	requireAVCConfig(t, video[1], sps, pps)
}

// requireAVCConfig checks that msg's sequence header survives the wire: it
// is marshalled and read back through gortmplib's own parser.
func requireAVCConfig(t *testing.T, msg *message.Video, sps, pps []byte) {
	t.Helper()
	var buf bytes.Buffer
	rw := message.NewReadWriter(&buf, bytecounter.NewReadWriter(&buf), false)
	require.NoError(t, rw.Write(msg))
	read, err := rw.Read()
	require.NoError(t, err)
	got, ok := read.(*message.Video)
	require.True(t, ok)
	require.NotNil(t, got.AVCConfig)
	require.Equal(t, sps, got.AVCConfig.SequenceParameterSets[0].NALUnit)
	require.Equal(t, pps, got.AVCConfig.PictureParameterSets[0].NALUnit)
}
