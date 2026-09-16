package media

import (
	"context"
	"encoding/binary"

	"github.com/go-gst/go-gst/gst"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/spmetrics"
)

// h264NALTypes lists the NAL unit types of one H.264 access unit, in either
// Annex B (start codes) or AVC (4-byte length prefixes) form. It returns nil
// when the bytes are neither.
func h264NALTypes(b []byte) []int {
	if len(b) < 5 {
		return nil
	}
	if b[0] == 0 && b[1] == 0 && (b[2] == 1 || (b[2] == 0 && b[3] == 1)) {
		var out []int
		for i := 0; i+3 < len(b); {
			if b[i] == 0 && b[i+1] == 0 && b[i+2] == 1 {
				out = append(out, int(b[i+3]&31))
				i += 4
				continue
			}
			i++
		}
		return out
	}
	var out []int
	for i := 0; i+4 <= len(b); {
		l := int(binary.BigEndian.Uint32(b[i:]))
		if l <= 0 || i+4+l > len(b) {
			return nil // not length-prefixed after all
		}
		out = append(out, int(b[i+4]&31))
		i += 4 + l
	}
	return out
}

// auHasIDR reports whether an H.264 access unit contains an IDR slice: the
// only kind of frame a segment may start on, since only an IDR resets the
// decoder and only an IDR is guaranteed to have the parameter sets a
// decoder joining the stream needs.
func auHasIDR(b []byte) bool {
	for _, t := range h264NALTypes(b) {
		if t == 5 {
			return true
		}
	}
	return false
}

// installIDRKeyframeProbe makes the H.264 buffers passing pad carry the
// keyframe flag only when they hold an IDR slice. h264parse marks every
// I-slice as a keyframe, and a broadcast encoder that emits open-GOP
// I-frames (non-IDR, no SPS/PPS, seen at scene cuts) then gets those
// written as sync samples, on which the segmenter starts a new signed
// segment — one no decoder can start on: the transcoder rejects it, the
// packetizer finds no frame in it, and an HLS player joining there stalls.
// Clearing the flag keeps such frames inside the GoP of the IDR before
// them.
func installIDRKeyframeProbe(ctx context.Context, pad *gst.Pad, streamer string) {
	if pad == nil {
		return
	}
	pad.AddProbe(gst.PadProbeTypeBuffer, func(_ *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		buf := info.GetBuffer()
		if buf == nil || buf.HasFlags(gst.BufferFlagDeltaUnit) {
			return gst.PadProbeOK
		}
		m := buf.Map(gst.MapRead)
		if m == nil {
			return gst.PadProbeOK
		}
		idr := auHasIDR(m.Bytes())
		buf.Unmap()
		if !idr {
			buf.SetFlags(gst.BufferFlagDeltaUnit)
			spmetrics.IngestNonIDRKeyframesTotal.WithLabelValues(streamer).Inc()
			log.Debug(ctx, "ingest: keyframe-flagged frame without an IDR slice kept inside its GoP", "streamer", streamer)
		}
		return gst.PadProbeOK
	})
}
