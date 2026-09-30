package api

import (
	"testing"
	"time"

	"github.com/bluenviron/gortmplib/pkg/h264conf"
	"github.com/bluenviron/gortmplib/pkg/message"
	"github.com/stretchr/testify/require"
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
