package media

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/abema/go-mp4"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/av1"
	"github.com/pion/webrtc/v4"
	"stream.place/streamplace/pkg/muxl"
)

// VideoCodec is a video codec Streamplace carries, and what a pipeline needs
// to know to carry it. It is the video side of isAACCodec/isOpusCodec: a muxl
// catalog names a track's codec by its RFC 6381 string (avc1.640028,
// av01.0.12M.08) and a demuxer's pad by its caps (video/x-h264, video/x-av1),
// and each pipeline that touches video picks its elements from the codec the
// track turns out to be.
type VideoCodec struct {
	// Name is the codec's name in place.stream records: "h264", "av1".
	Name string
	// Caps is the caps name of the codec's elementary stream.
	Caps string
	// Parser is the codec's GStreamer parser element.
	Parser string
	// SampleCaps is the form a sample-at-a-time consumer (WebRTC) takes the
	// parser's output in: Annex B access units, or AV1 temporal units of
	// sized OBUs.
	SampleCaps string
	// MimeType is the codec's WebRTC MIME type.
	MimeType string
	// IsKeyframe reports whether one sample (an access unit, a temporal
	// unit) is a frame a decoder can start on: an IDR, a shown key frame.
	IsKeyframe func(sample []byte) bool
	// catalogPrefixes are the codec's RFC 6381 sample-entry prefixes.
	catalogPrefixes []string
}

var (
	VideoH264 = VideoCodec{
		Name:            "h264",
		Caps:            "video/x-h264",
		Parser:          "h264parse",
		SampleCaps:      "video/x-h264,stream-format=byte-stream",
		MimeType:        webrtc.MimeTypeH264,
		IsKeyframe:      auHasIDR,
		catalogPrefixes: []string{"avc1", "avc3"},
	}
	VideoAV1 = VideoCodec{
		Name:            "av1",
		Caps:            "video/x-av1",
		Parser:          "av1parse",
		SampleCaps:      "video/x-av1,stream-format=obu-stream,alignment=tu",
		MimeType:        webrtc.MimeTypeAV1,
		IsKeyframe:      tuHasShownKeyFrame,
		catalogPrefixes: []string{"av01"},
	}
	videoCodecs = []VideoCodec{VideoH264, VideoAV1}
)

// VideoCodecForCatalog is the codec of a muxl catalog codec string.
func VideoCodecForCatalog(codec string) (VideoCodec, bool) {
	for _, c := range videoCodecs {
		for _, p := range c.catalogPrefixes {
			if strings.HasPrefix(codec, p) {
				return c, true
			}
		}
	}
	return VideoCodec{}, false
}

// VideoCodecForCaps is the codec of a caps name ("video/x-av1") or of a caps
// string that starts with one ("video/x-av1, stream-format=obu-stream").
func VideoCodecForCaps(caps string) (VideoCodec, bool) {
	name, _, _ := strings.Cut(caps, ",")
	for _, c := range videoCodecs {
		if strings.TrimSpace(name) == c.Caps {
			return c, true
		}
	}
	return VideoCodec{}, false
}

// VideoCodecForName is the codec a place.stream record names ("av1").
func VideoCodecForName(name string) (VideoCodec, bool) {
	for _, c := range videoCodecs {
		if c.Name == name {
			return c, true
		}
	}
	return VideoCodec{}, false
}

// ParserLaunch is the launch description of c's parser element, named name.
// h264Props are h264parse properties ("config-interval=-1"), which have no
// av1parse counterpart: AV1 carries its sequence header in every key frame's
// temporal unit and in the sample entry's av1C, so there is nothing to
// re-insert.
func (c VideoCodec) ParserLaunch(name, h264Props string) string {
	s := c.Parser
	if name != "" {
		s += " name=" + name
	}
	if c.Name == VideoH264.Name && h264Props != "" {
		s += " " + h264Props
	}
	return s
}

// videoTracks returns a catalog's video renditions in track-id order.
//
// A segment can carry more than one video track: the OBS plugin encodes every
// video rendition the streamer configured into one push, say 4K120 AV1 and a
// 1080p H.264 encode for devices that can't decode AV1. The first track is the
// source. A consumer that takes one video track takes the first in a codec it
// can handle (see pickVideoTrack); HLS lists them all.
func videoTracks(cat *muxl.MuxlCatalog) []muxl.MuxlVideoConfig {
	if cat == nil || cat.Video == nil {
		return nil
	}
	out := make([]muxl.MuxlVideoConfig, 0, len(cat.Video.Renditions))
	for _, v := range cat.Video.Renditions {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TrackID() < out[j].TrackID() })
	return out
}

// pickVideoTrack returns the first of a catalog's video tracks, in track-id
// order, whose codec accept takes; a nil accept takes any codec Streamplace
// knows.
func pickVideoTrack(cat *muxl.MuxlCatalog, accept func(VideoCodec) bool) (muxl.MuxlVideoConfig, VideoCodec, bool) {
	for _, v := range videoTracks(cat) {
		c, ok := VideoCodecForCatalog(v.Codec)
		if ok && (accept == nil || accept(c)) {
			return v, c, true
		}
	}
	return muxl.MuxlVideoConfig{}, VideoCodec{}, false
}

// onlyH264 is a pickVideoTrack accept for the consumers H.264 is all they can
// take: RTMP push, the transcoder.
func onlyH264(c VideoCodec) bool { return c.Name == VideoH264.Name }

// errNoVideoTrack is the error of a consumer that found no video track in a
// codec it takes (see pickVideoTrack).
var errNoVideoTrack = errors.New("no video track in an accepted codec")

// AV1 OBU types (AV1 Bitstream & Decoding Process, section 6.2.2).
const (
	obuSequenceHeader = 1
	obuFrameHeader    = 3
	obuFrame          = 6
)

// av1OBUs splits a temporal unit of sized OBUs (the low-overhead bitstream
// format MP4 samples and av1parse's obu-stream carry) into each OBU's type and
// payload. It stops at the first malformed OBU.
func av1OBUs(tu []byte, each func(typ int, payload []byte) bool) {
	for len(tu) > 0 {
		h := tu[0]
		if h&0x80 != 0 {
			return // forbidden bit
		}
		typ, ext, sized := int(h>>3)&0xf, h&0x04 != 0, h&0x02 != 0
		n := 1
		if ext {
			n++
		}
		if len(tu) < n {
			return
		}
		size := len(tu) - n
		if sized {
			var v, shift int
			for {
				if n >= len(tu) || shift > 49 {
					return
				}
				b := tu[n]
				n++
				v |= int(b&0x7f) << shift
				shift += 7
				if b&0x80 == 0 {
					break
				}
			}
			size = v
		}
		if size < 0 || n+size > len(tu) {
			return
		}
		if !each(typ, tu[n:n+size]) {
			return
		}
		tu = tu[n+size:]
	}
}

// tuHasShownKeyFrame reports whether an AV1 temporal unit holds a shown key
// frame: the only kind a decoder joining the stream can start on. av1parse
// flags any temporal unit with a key frame as a sync point, hidden ones (a
// forward key frame, AV1's open GOP) included, and a segment cut there starts
// on a frame nobody displays.
func tuHasShownKeyFrame(tu []byte) bool {
	reduced, key := false, false
	av1OBUs(tu, func(typ int, p []byte) bool {
		switch typ {
		case obuSequenceHeader:
			// seq_profile(3) still_picture(1) reduced_still_picture_header(1)
			reduced = len(p) > 0 && p[0]&0x08 != 0
		case obuFrameHeader, obuFrame:
			if reduced {
				key = true // a still picture: one shown key frame
				return false
			}
			if len(p) == 0 || p[0]&0x80 != 0 {
				return true // empty, or show_existing_frame
			}
			// show_existing_frame(1) frame_type(2) show_frame(1)
			if (p[0]>>5)&0x3 == 0 && p[0]&0x10 != 0 {
				key = true
				return false
			}
		}
		return true
	})
	return key
}

// tuSequenceHeader returns the sequence header OBU of a run of sized OBUs (a
// temporal unit, an av1C's configOBUs), with its header and size, or nil when
// it has none.
func tuSequenceHeader(tu []byte) []byte {
	var seq []byte
	av1OBUs(tu, func(typ int, p []byte) bool {
		if typ != obuSequenceHeader {
			return true
		}
		// obu_type, obu_has_size_field, then the leb128 obu_size
		seq = binary.AppendUvarint([]byte{obuSequenceHeader<<3 | 0x02}, uint64(len(p)))
		seq = append(seq, p...)
		return false
	})
	return seq
}

// AV1CodecConfig is the AV1CodecConfigurationRecord (av1C) of a stream whose
// sequence header OBU is seq (with or without its size field): an av01 sample
// entry's configuration, and AV1 caps' codec_data. Without codec_data in its
// caps, qtmux writes an av1C of its own with level 1, no sequence header and
// two bytes it never initializes.
func AV1CodecConfig(seq []byte) (*mp4.Av1C, error) {
	var sh av1.SequenceHeader
	if err := sh.Unmarshal(seq); err != nil {
		return nil, fmt.Errorf("invalid AV1 sequence header: %w", err)
	}
	configOBUs, err := av1.Bitstream{seq}.Marshal()
	if err != nil {
		return nil, fmt.Errorf("invalid AV1 sequence header: %w", err)
	}
	bit := func(b bool) uint8 {
		if b {
			return 1
		}
		return 0
	}
	return &mp4.Av1C{
		Marker:               1,
		Version:              1,
		SeqProfile:           sh.SeqProfile,
		SeqLevelIdx0:         sh.SeqLevelIdx[0],
		SeqTier0:             bit(sh.SeqTier[0]),
		HighBitdepth:         bit(sh.ColorConfig.HighBitDepth),
		TwelveBit:            bit(sh.ColorConfig.TwelveBit),
		Monochrome:           bit(sh.ColorConfig.MonoChrome),
		ChromaSubsamplingX:   bit(sh.ColorConfig.SubsamplingX),
		ChromaSubsamplingY:   bit(sh.ColorConfig.SubsamplingY),
		ChromaSamplePosition: uint8(sh.ColorConfig.ChromaSamplePosition),
		ConfigOBUs:           configOBUs,
	}, nil
}

// av1CodecDataCaps is VideoAV1.SampleCaps with the av1C of sequence header
// OBU seq (see AV1CodecConfig) as codec_data.
func av1CodecDataCaps(seq []byte) (string, error) {
	av1C, err := AV1CodecConfig(seq)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if _, err := mp4.Marshal(&b, av1C, mp4.Context{}); err != nil {
		return "", fmt.Errorf("marshal av1C: %w", err)
	}
	return fmt.Sprintf("%s,codec_data=(buffer)%x", VideoAV1.SampleCaps, b.Bytes()), nil
}
