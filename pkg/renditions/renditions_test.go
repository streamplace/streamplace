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
					"name": "1080p",
					"height": 1080,
					"bitrate": 6000000,
					"profile": "h264constrainedhigh"
				},
				{
					"name": "720p",
					"height": 720,
					"bitrate": 3000000,
					"profile": "h264constrainedhigh"
				},
				{
					"name": "360p",
					"height": 360,
					"bitrate": 1000000,
					"profile": "h264constrainedhigh",
					"fps": 60,
					"fpsDen": 2
				},
				{
					"name": "240p",
					"height": 240,
					"bitrate": 500000,
					"profile": "h264constrainedhigh",
					"fps": 60,
					"fpsDen": 2
				},
				{
					"name": "160p",
					"height": 160,
					"bitrate": 250000,
					"profile": "h264baseline",
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
					"name": "1080p",
					"height": 1080,
					"bitrate": 6000000,
					"profile": "h264constrainedhigh"
				},
				{
					"name": "720p",
					"height": 720,
					"bitrate": 3000000,
					"profile": "h264constrainedhigh"
				},
				{
					"name": "360p",
					"height": 360,
					"bitrate": 1000000,
					"profile": "h264constrainedhigh",
					"fps": 60000,
					"fpsDen": 2002
				},
				{
					"name": "240p",
					"height": 240,
					"bitrate": 500000,
					"profile": "h264constrainedhigh",
					"fps": 60000,
					"fpsDen": 2002
				},
				{
					"name": "160p",
					"height": 160,
					"bitrate": 250000,
					"profile": "h264baseline",
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
					"name": "360p",
					"height": 360,
					"bitrate": 1000000,
					"profile": "h264constrainedhigh",
					"fps": 50,
					"fpsDen": 2
				},
				{
					"name": "240p",
					"height": 240,
					"bitrate": 500000,
					"profile": "h264constrainedhigh",
					"fps": 50,
					"fpsDen": 2
				},
				{
					"name": "160p",
					"height": 160,
					"bitrate": 250000,
					"profile": "h264baseline",
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
					"name": "360p",
					"height": 360,
					"bitrate": 1000000,
					"profile": "h264constrainedhigh"
				},
				{
					"name": "240p",
					"height": 240,
					"bitrate": 500000,
					"profile": "h264constrainedhigh"
				},
				{
					"name": "160p",
					"height": 160,
					"bitrate": 250000,
					"profile": "h264baseline"
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
					"name": "360p",
					"height": 360,
					"bitrate": 1000000,
					"profile": "h264constrainedhigh"
				},
				{
					"name": "240p",
					"height": 240,
					"bitrate": 500000,
					"profile": "h264constrainedhigh"
				},
				{
					"name": "160p",
					"height": 160,
					"bitrate": 250000,
					"profile": "h264baseline"
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
					"name": "360p",
					"width": 360,
					"bitrate": 1000000,
					"profile": "h264constrainedhigh",
					"fps": 60,
					"fpsDen": 2
				},
				{
					"name": "240p",
					"width": 240,
					"bitrate": 500000,
					"profile": "h264constrainedhigh",
					"fps": 60,
					"fpsDen": 2
				},
				{
					"name": "160p",
					"width": 160,
					"bitrate": 250000,
					"profile": "h264baseline",
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

var singleCases = []struct {
	name       string
	spseg      *streamplace.Segment
	lp         string
	dimensions []int
}{
	{
		name:       "Nearly-Square Landscape 4K 60fps",
		spseg:      seg(3840, 3830, 60, 1),
		dimensions: []int{1083, 1080},
		lp: `
			{
				"name": "1080p",
				"height": 1080,
				"bitrate": 6000000,
				"profile": "h264constrainedhigh"
			}
	`,
	},
	{
		name:       "Nearly-Square Portrait 4K 60fps",
		spseg:      seg(3830, 3840, 60, 1),
		dimensions: []int{1080, 1083},
		lp: `
			{
				"name": "1080p",
				"width": 1080,
				"bitrate": 6000000,
				"profile": "h264constrainedhigh"
			}
		`,
	},
	{
		name:       "Stupidly-Wide Landscape 4K 60fps",
		spseg:      seg(5000, 2160, 60, 1),
		dimensions: []int{1920, 829},
		lp: `
			{
				"name": "1080p",
				"width": 1920,
				"bitrate": 6000000,
				"profile": "h264constrainedhigh"
			}
	`,
	},
	{
		name:       "Stupidly-Tall Portrait 4K 60fps",
		spseg:      seg(2160, 5000, 60, 1),
		dimensions: []int{829, 1920},
		lp: `
			{
				"name": "1080p",
				"height": 1920,
				"bitrate": 6000000,
				"profile": "h264constrainedhigh"
			}
		`,
	},
}

func TestSingleRendition(t *testing.T) {
	for _, c := range singleCases {
		t.Run(c.name, func(t *testing.T) {
			rends, err := GenerateRenditions(c.spseg)
			require.NoError(t, err)
			first := rends[0]
			require.Equal(t, c.dimensions, []int{int(first.Width), int(first.Height)})
			lp := rends.ToLivepeerProfiles()
			bs, err := json.Marshal(lp[0])
			require.NoError(t, err)
			require.JSONEq(t, c.lp, string(bs))
		})
	}
}
