package media

import (
	"context"
	"fmt"
	"io"

	"github.com/livepeer/lpms/ffmpeg"
	"golang.org/x/sync/errgroup"
)

func (mm *MediaManager) SegmentToMKV(ctx context.Context, user string, rendition string, w io.Writer) error {
	muxer := ffmpeg.ComponentOptions{
		Name: "matroska",
	}
	return mm.SegmentToStream(ctx, user, rendition, muxer, w)
}

func (mm *MediaManager) SegmentToMKVPlusOpus(ctx context.Context, user string, rendition string, w io.Writer) error {
	muxer := ffmpeg.ComponentOptions{
		Name: "matroska",
	}
	pr, pw := io.Pipe()
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return mm.SegmentToStream(ctx, user, rendition, muxer, pw)
	})
	g.Go(func() error {
		return AddOpusToMKV(ctx, pr, w)
	})
	return g.Wait()
}

func (mm *MediaManager) SegmentToMP4(ctx context.Context, user string, rendition string, w io.Writer) error {
	muxer := ffmpeg.ComponentOptions{
		Name: "mp4",
		Opts: map[string]string{
			"movflags": "frag_keyframe+empty_moov",
		},
	}
	return mm.SegmentToStream(ctx, user, rendition, muxer, w)
}

func (mm *MediaManager) SegmentToStream(ctx context.Context, user string, rendition string, muxer ffmpeg.ComponentOptions, w io.Writer) error {
	tc := ffmpeg.NewTranscoder()
	defer tc.StopTranscoder()
	ourl, or, odone, err := mm.HTTPPipe()
	if err != nil {
		return err
	}
	defer odone()
	iname := fmt.Sprintf("%s/playback/%s/%s/concat", mm.cli.OwnInternalURL(), user, rendition)
	in := &ffmpeg.TranscodeOptionsIn{
		Fname:       iname,
		Transmuxing: true,
		Profile:     ffmpeg.VideoProfile{},
		Loop:        -1,
		Demuxer: ffmpeg.ComponentOptions{
			Name: "concat",
			Opts: map[string]string{
				"safe":               "0",
				"protocol_whitelist": "file,http,https,tcp,tls",
			},
		},
	}
	out := []ffmpeg.TranscodeOptions{
		{
			Oname: ourl,
			VideoEncoder: ffmpeg.ComponentOptions{
				Name: "copy",
			},
			AudioEncoder: ffmpeg.ComponentOptions{
				Name: "copy",
			},
			Profile: ffmpeg.VideoProfile{Format: ffmpeg.FormatNone},
			Muxer:   muxer,
		},
	}
	g, _ := errgroup.WithContext(ctx)
	g.Go(func() error {
		<-ctx.Done()
		or.Close()
		return nil
	})
	g.Go(func() error {
		_, err := tc.Transcode(in, out)
		tc.StopTranscoder()
		return err
	})
	g.Go(func() error {
		_, err := io.Copy(w, or)
		return err
	})
	return g.Wait()
}
