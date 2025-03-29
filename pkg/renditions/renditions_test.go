package renditions

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/streamplace"
)

func seg(width int, height int, fpsNum int, fpsDen int) *streamplace.Segment {
	return &streamplace.Segment{
		Video: []*streamplace.Segment_Video{
			{
				Width:  int64(width),
				Height: int64(height),
				Framerate: &streamplace.Segment_Framerate{
					Num: int64(fpsNum),
					Den: int64(fpsDen),
				},
			},
		},
	}
}

var cases = []struct {
	name  string
	spseg *streamplace.Segment
	lp    string
}{
	{
		name:  "4K 60fps",
		spseg: seg(3840, 2160, 60, 1),
		lp: `
			[
				{
					"width": 1920,
					"bitrate": 6000000,
					"profile": "h264constrainedhigh",
					"encoder": "h264"
				},
				{
					"width": 1280,
					"bitrate": 3000000,
					"profile": "h264constrainedhigh",
					"encoder": "h264"
				},
				{
					"width": 640,
					"bitrate": 1000000,
					"profile": "h264constrainedhigh",
					"encoder": "h264",
					"fps": 60,
					"fpsDen": 2
				},
				{
					"width": 426,
					"bitrate": 500000,
					"profile": "h264constrainedhigh",
					"encoder": "h264",
					"fps": 60,
					"fpsDen": 2
				},
				{
					"width": 284,
					"bitrate": 250000,
					"profile": "h264baseline",
					"encoder": "h264",
					"fps": 60,
					"fpsDen": 2
				}
			]
		`,
	},
	{
		name:  "2K with fractional framerate",
		spseg: seg(2160, 1440, 60000, 1001),
		lp: `
			[
				{
					"width": 1920,
					"bitrate": 6000000,
					"profile": "h264constrainedhigh",
					"encoder": "h264"
				},
				{
					"width": 1280,
					"bitrate": 3000000,
					"profile": "h264constrainedhigh",
					"encoder": "h264"
				},
				{
					"width": 640,
					"bitrate": 1000000,
					"profile": "h264constrainedhigh",
					"encoder": "h264",
					"fps": 60000,
					"fpsDen": 2002
				},
				{
					"width": 426,
					"bitrate": 500000,
					"profile": "h264constrainedhigh",
					"encoder": "h264",
					"fps": 60000,
					"fpsDen": 2002
				},
				{
					"width": 284,
					"bitrate": 250000,
					"profile": "h264baseline",
					"encoder": "h264",
					"fps": 60000,
					"fpsDen": 2002
				}
			]
		`,
	},
	{
		name:  "720p 50fps",
		spseg: seg(1280, 720, 50, 1),
		lp: `
			[
				{
					"width": 640,
					"bitrate": 1000000,
					"profile": "h264constrainedhigh",
					"encoder": "h264",
					"fps": 50,
					"fpsDen": 2
				},
				{
					"width": 426,
					"bitrate": 500000,
					"profile": "h264constrainedhigh",
					"encoder": "h264",
					"fps": 50,
					"fpsDen": 2
				},
				{
					"width": 284,
					"bitrate": 250000,
					"profile": "h264baseline",
					"encoder": "h264",
					"fps": 50,
					"fpsDen": 2
				}
			]
		`,
	},
	{
		name:  "720p 30fps",
		spseg: seg(1280, 720, 30, 1),
		lp: `
			[
				{
					"width": 640,
					"bitrate": 1000000,
					"profile": "h264constrainedhigh",
					"encoder": "h264"
				},
				{
					"width": 426,
					"bitrate": 500000,
					"profile": "h264constrainedhigh",
					"encoder": "h264"
				},
				{
					"width": 284,
					"bitrate": 250000,
					"profile": "h264baseline",
					"encoder": "h264"
				}
			]
		`,
	},
	{
		name:  "720p 25fps",
		spseg: seg(1280, 720, 25, 1),
		lp: `
			[
				{
					"width": 640,
					"bitrate": 1000000,
					"profile": "h264constrainedhigh",
					"encoder": "h264"
				},
				{
					"width": 426,
					"bitrate": 500000,
					"profile": "h264constrainedhigh",
					"encoder": "h264"
				},
				{
					"width": 284,
					"bitrate": 250000,
					"profile": "h264baseline",
					"encoder": "h264"
				}
			]
		`,
	},
	{
		name:  "Vertical video 60fps",
		spseg: seg(480, 640, 60, 1),
		lp: `
			[
				{
					"height": 640,
					"bitrate": 1000000,
					"profile": "h264constrainedhigh",
					"encoder": "h264",
					"fps": 60,
					"fpsDen": 2
				},
				{
					"height": 426,
					"bitrate": 500000,
					"profile": "h264constrainedhigh",
					"encoder": "h264",
					"fps": 60,
					"fpsDen": 2
				},
				{
					"height": 284,
					"bitrate": 250000,
					"profile": "h264baseline",
					"encoder": "h264",
					"fps": 60,
					"fpsDen": 2
				}
			]
		`,
	},
}

func TestRenditions(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rends, err := GenerateRenditions(c.spseg)
			require.NoError(t, err)
			lp := rends.ToLivepeerProfiles()
			bs, err := json.Marshal(lp)
			require.NoError(t, err)
			require.JSONEq(t, c.lp, string(bs))
		})
	}
}
