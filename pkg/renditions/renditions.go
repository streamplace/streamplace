package renditions

import (
	"fmt"

	"stream.place/streamplace/pkg/streamplace"
)

type FPS struct {
	Passthrough bool
	Num         uint
	Den         uint
}

type Rendition struct {
	Width     int64
	Height    int64
	Bitrate   int
	Framerate FPS
	Profile   string
	Name      string
}

type JsonProfile struct {
	Name    string `json:"name,omitempty"`
	Width   int    `json:"width,omitempty"`
	Height  int    `json:"height,omitempty"`
	Bitrate int    `json:"bitrate,omitempty"`
	FPS     uint   `json:"fps,omitempty"`
	FPSDen  uint   `json:"fpsDen,omitempty"`
	Profile string `json:"profile,omitempty"`
	GOP     string `json:"gop,omitempty"`
	Encoder string `json:"encoder,omitempty"`
	Quality uint   `json:"quality,omitempty"`
}

func (r Rendition) ToLivepeerProfile() JsonProfile {
	p := JsonProfile{
		Name:    r.Name,
		Bitrate: r.Bitrate,
		FPS:     r.Framerate.Num,
		FPSDen:  r.Framerate.Den,
		Profile: r.Profile,
	}
	if r.Height < r.Width {
		p.Height = int(r.Height)
	} else {
		p.Width = int(r.Width)
	}
	return p
}

type Renditions []Rendition

func (rs Renditions) ToLivepeerProfiles() []JsonProfile {
	profiles := make([]JsonProfile, len(rs))
	for i, r := range rs {
		profiles[i] = r.ToLivepeerProfile()
	}
	return profiles
}

var DesiredRenditions = []Rendition{
	{
		Name:    "1080p",
		Width:   1920,
		Height:  1080,
		Bitrate: 6_000_000,
		Framerate: FPS{
			Num: 60,
			Den: 1,
		},
		Profile: "h264constrainedhigh",
	},
	{
		Name:    "720p",
		Width:   1280,
		Height:  720,
		Bitrate: 3_000_000,
		Framerate: FPS{
			Num: 60,
			Den: 1,
		},
		Profile: "h264constrainedhigh",
	},
	{
		Name:    "360p",
		Width:   640,
		Height:  360,
		Bitrate: 1_000_000,
		Framerate: FPS{
			Num: 30,
			Den: 1,
		},
		Profile: "h264constrainedhigh",
	},
	{
		Name:    "240p",
		Width:   426,
		Height:  240,
		Bitrate: 500_000,
		Framerate: FPS{
			Num: 30,
			Den: 1,
		},
		Profile: "h264constrainedhigh",
	},
	{
		Name:    "160p",
		Width:   284,
		Height:  160,
		Bitrate: 250_000,
		Framerate: FPS{
			Num: 30,
			Den: 1,
		},
		Profile: "h264baseline",
	},
}

// GenerateRenditions generates renditions for a given spseg
func GenerateRenditions(spseg *streamplace.Segment) (Renditions, error) {
	vid := spseg.Video[0]
	if vid == nil {
		return nil, fmt.Errorf("no video stream found")
	}
	rs := []Rendition{}
	for _, r := range DesiredRenditions {
		vertical := vid.Height > vid.Width
		if vid.Width <= r.Width && vid.Height <= r.Height {
			continue
		}
		outR := Rendition{
			Name: r.Name,
		}
		if vertical {
			ratio := float64(r.Height) / float64(vid.Height)
			outR.Height = r.Width
			outR.Width = int64(float64(vid.Height) * (16.0 / 9.0) * ratio)
		} else {
			ratio := float64(r.Width) / float64(vid.Width)
			outR.Width = r.Width
			outR.Height = int64(float64(vid.Width) * (9.0 / 16.0) * ratio)
		}
		if vid.Framerate.Den > 0 {
			vidFPS := float64(vid.Framerate.Num) / float64(vid.Framerate.Den)
			rFPS := float64(r.Framerate.Num) / float64(r.Framerate.Den)
			delta := rFPS / vidFPS

			if rFPS < vidFPS {
				if delta < 0.75 {
					outR.Framerate.Num = uint(vid.Framerate.Num)
					outR.Framerate.Den = uint(vid.Framerate.Den * 2)
				}
			} else {
				fmt.Printf("rFPS %f vidFPS %f delta %f\n", rFPS, vidFPS, delta)
			}
		}

		outR.Bitrate = r.Bitrate
		outR.Profile = r.Profile
		rs = append(rs, outR)
	}
	return rs, nil
}
