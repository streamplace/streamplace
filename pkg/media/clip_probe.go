package media

import (
	"fmt"
	"io"

	"github.com/abema/go-mp4"
)

// ProbeClipFile extracts the track metadata publishTrack needs — duration,
// video dimensions, audio sample rate + channel count — from a muxed clip MP4
// (the ftyp+moov+mdat muxl synthesizes from canonical segments). It mirrors
// the VODResult the gstreamer pipeline produces for uploads so the clip
// publish path can build media.track records identically. publishTrack
// hardcodes the video codec to h264, so the probe only contributes
// width/height; audio codec is "mpeg" (aac) or "x-opus", matching the values
// audioCodecForLexicon maps back to the segment record's enum.
func ProbeClipFile(r io.ReadSeeker) (VODResult, error) {
	var res VODResult
	_, err := mp4.ReadBoxStructure(r, func(h *mp4.ReadHandle) (interface{}, error) {
		// ReadBoxStructure only descends into children when the handler calls
		// Expand — do it for every box so mvhd/trak/stsd are all visited.
		defer func() {
			_, _ = h.Expand()
		}()
		switch h.BoxInfo.Type {
		case mp4.BoxTypeMvhd():
			box, _, err := h.ReadPayload()
			if err != nil {
				return nil, err
			}
			mvhd, ok := box.(*mp4.Mvhd)
			if !ok || mvhd.Timescale == 0 {
				return nil, nil
			}
			duration := uint64(mvhd.DurationV0)
			if mvhd.Version == 1 {
				duration = mvhd.DurationV1
			}
			res.DurationMS = int64(duration * 1000 / uint64(mvhd.Timescale))
		case mp4.BoxTypeAvc1(), mp4.BoxTypeHvc1(), mp4.BoxTypeAv01(), mp4.BoxTypeVp09():
			if res.Video != nil {
				return nil, nil
			}
			box, _, err := h.ReadPayload()
			if err != nil {
				return nil, err
			}
			vse, ok := box.(*mp4.VisualSampleEntry)
			if !ok {
				return nil, nil
			}
			res.Video = &VODVideoTrack{
				Codec:  "h264",
				Width:  int(vse.Width),
				Height: int(vse.Height),
			}
		case mp4.BoxTypeMp4a():
			if res.Audio != nil {
				return nil, nil
			}
			box, _, err := h.ReadPayload()
			if err != nil {
				return nil, err
			}
			ase, ok := box.(*mp4.AudioSampleEntry)
			if !ok {
				return nil, nil
			}
			res.Audio = &VODAudioTrack{
				Codec:    "mpeg",
				Rate:     int(ase.SampleRate >> 16),
				Channels: int(ase.ChannelCount),
			}
		case mp4.BoxTypeOpus():
			if res.Audio != nil {
				return nil, nil
			}
			box, _, err := h.ReadPayload()
			if err != nil {
				return nil, err
			}
			ase, ok := box.(*mp4.AudioSampleEntry)
			if !ok {
				return nil, nil
			}
			res.Audio = &VODAudioTrack{
				Codec:    "x-opus",
				Rate:     int(ase.SampleRate >> 16),
				Channels: int(ase.ChannelCount),
			}
		}
		return nil, nil
	})
	if err != nil {
		return VODResult{}, fmt.Errorf("probe clip mp4: %w", err)
	}
	return res, nil
}
