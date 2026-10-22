package media

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/go-gst/go-gst/pkg/gst"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/muxl"
)

// Audio codec identification. muxl catalogs carry the MIME-ish codec string
// (`mp4a.40.2` for AAC, `opus` for Opus). We complete every segment to carry
// one of each so downstream pipelines can pick what they need.
func isAACCodec(codec string) bool  { return strings.HasPrefix(codec, "mp4a") }
func isOpusCodec(codec string) bool { return strings.HasPrefix(codec, "opus") }

// Audio track selection. A segment's audio tracks are grouped by their
// catalog alternateGroup: tracks sharing a group carry the same sound in
// different codecs (the AAC and Opus encodes of one mix), and different
// groups are independent sounds — the OBS plugin's full mix and its stems
// ("Music"). Tracks with no group (0) all share one group, which is every
// segment that isn't the plugin's.
//
// The main audio is the lowest group. Every consumer that wants one audio
// track — HLS, RTMP push, VOD, thumbnails, the WebRTC mix, codec completion —
// picks from the main group by codec; the stems reach WebRTC viewers only
// (see Packetize), who mix them at their own volumes.

// maxStems bounds how many stem groups a segment contributes to playback:
// the plugin sends at most two (six encoders per output, two per track).
const maxStems = 2

// audioTracks returns a catalog's audio renditions in track-id order.
func audioTracks(cat *muxl.MuxlCatalog) []muxl.MuxlAudioConfig {
	if cat == nil || cat.Audio == nil {
		return nil
	}
	out := make([]muxl.MuxlAudioConfig, 0, len(cat.Audio.Renditions))
	for _, a := range cat.Audio.Renditions {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TrackID() < out[j].TrackID() })
	return out
}

// mainAudioGroup is the lowest alternateGroup among tracks: 0 for an
// ungrouped segment.
func mainAudioGroup(tracks []muxl.MuxlAudioConfig) uint32 {
	var g uint32
	for i, a := range tracks {
		if i == 0 || a.AlternateGroup < g {
			g = a.AlternateGroup
		}
	}
	return g
}

// mainAudioTracks returns the main group's tracks (see mainAudioGroup), in
// track-id order.
func mainAudioTracks(tracks []muxl.MuxlAudioConfig) []muxl.MuxlAudioConfig {
	g := mainAudioGroup(tracks)
	var out []muxl.MuxlAudioConfig
	for _, a := range tracks {
		if a.AlternateGroup == g {
			out = append(out, a)
		}
	}
	return out
}

// stemOpusTracks returns one Opus track per stem group — the groups above the
// main one, ascending, at most maxStems — in that order. WebRTC carries Opus
// only, so a stem group without an Opus encode is skipped.
func stemOpusTracks(tracks []muxl.MuxlAudioConfig) []muxl.MuxlAudioConfig {
	main := mainAudioGroup(tracks)
	byGroup := map[uint32]muxl.MuxlAudioConfig{}
	var groups []uint32
	for _, a := range tracks {
		if a.AlternateGroup <= main || !isOpusCodec(a.Codec) {
			continue
		}
		if _, seen := byGroup[a.AlternateGroup]; !seen {
			groups = append(groups, a.AlternateGroup)
			byGroup[a.AlternateGroup] = a
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i] < groups[j] })
	if len(groups) > maxStems {
		groups = groups[:maxStems]
	}
	out := make([]muxl.MuxlAudioConfig, 0, len(groups))
	for _, g := range groups {
		out = append(out, byGroup[g])
	}
	return out
}

// transcodeManifest is the C2PA manifest for a transcode-completed audio
// track: it opens the source segment as an ingredient and records a
// c2pa.transcoded action referencing it, both by TranscodeIngredientLabel.
// muxl-sign stamps the per-segment time into cawg.metadata as it signs.
func transcodeManifest(creator string) []byte {
	return transcodeManifestFor(creator, "transcoded audio")
}

// transcodeManifestFor is the C2PA manifest a transcoded track is signed
// with: opened + transcoded actions over the source ingredient, and the
// streamer as creator.
func transcodeManifestFor(creator, title string) []byte {
	return []byte(fmt.Sprintf(`{
		"title": %q,
		"assertions": [
			{"label": "c2pa.actions", "data": {"actions": [
				{"action": "c2pa.opened",     "parameters": {"org.cai.ingredientIds": [%q]}},
				{"action": "c2pa.transcoded", "parameters": {"org.cai.ingredientIds": [%q]}}
			]}},
			{"label": "cawg.metadata", "data": {
				"@context": {"dc": "http://purl.org/dc/elements/1.1/"},
				"dc:creator": %q
			}}
		]
	}`, title, muxl.TranscodeIngredientLabel, muxl.TranscodeIngredientLabel, creator))
}

// transcodeSigner returns the node's S2PA cert + PKCS#8 key PEM, built once
// from the server-repo key. The transcode-completed track is signed under the
// node's own did:web identity (it vouches for its transcode), declaring the
// streamer's source track as a parentOf ingredient. The node key is software,
// so it's signed in-wasm via KeyPEM — the same proven path MediaSignerLocal
// uses for ecdsa keys.
func (mm *MediaManager) transcodeSigner() (cert []byte, keyPEM []byte, err error) {
	mm.nodeSignerOnce.Do(func() {
		signer, e := atproto.ServerCryptoSigner()
		if e != nil {
			mm.nodeSignerErr = fmt.Errorf("node transcode signer: %w", e)
			return
		}
		c, e := signers.GenerateES256KCert(signer)
		if e != nil {
			mm.nodeSignerErr = fmt.Errorf("node transcode cert: %w", e)
			return
		}
		k, e := signers.MarshalES256KPrivateKeyPEM(signer)
		if e != nil {
			mm.nodeSignerErr = fmt.Errorf("node transcode key: %w", e)
			return
		}
		mm.nodeCert = c
		mm.nodeKeyPEM = k
	})
	return mm.nodeCert, mm.nodeKeyPEM, mm.nodeSignerErr
}

// audioCompletionTarget inspects a segment's catalog and reports the codec to
// add ("opus" or "aac") when exactly one of AAC/Opus is present alongside a
// video track (needed as the segmentation clock). need is false when both
// codecs are already present, there's no audio, there's no video, or the codec
// is neither — in which case the segment is distributed as-is.
func (mm *MediaManager) audioCompletionTarget(ctx context.Context, seg []byte) (target string, need bool) {
	events, err := unwrapMuxlEvents(ctx, seg)
	if err != nil {
		log.Warn(ctx, "codec-completion inspect: unwrap failed", "error", err)
		return "", false
	}
	cat, _ := catalogAndTracks(events)
	return completionTarget(cat)
}

// completionTarget is audioCompletionTarget over an already-read catalog.
// Only the main audio group is completed: a stem (a higher group) is
// distributed as sent, since the plugin that labels stems encodes both codecs
// for each, and no HLS or RTMP consumer ever picks a stem. A segment whose
// main group already has both codecs needs nothing, whatever its stems carry.
func completionTarget(cat *muxl.MuxlCatalog) (target string, need bool) {
	if cat == nil || cat.Video == nil {
		return "", false
	}
	var haveAAC, haveOpus bool
	for _, a := range mainAudioTracks(audioTracks(cat)) {
		if isAACCodec(a.Codec) {
			haveAAC = true
		}
		if isOpusCodec(a.Codec) {
			haveOpus = true
		}
	}
	switch {
	case haveAAC && !haveOpus:
		return "opus", true
	case haveOpus && !haveAAC:
		return "aac", true
	default:
		return "", false
	}
}

// buildAudioTranscodePipeline constructs the audio-transcode pipeline shared by
// the per-segment ([transcodeAudioSegment]) and continuous ([streamTranscoder])
// paths: an `appsrc` of fMP4 → qtdemux → video (its codec's parser,
// passthrough) + audio (decode → re-encode to target) → fragmented mp4mux →
// `appsink`. Video rides along only so muxl can anchor the canonical segment on
// the video keyframe; callers keep the original signed video and use only the
// transcoded audio track. Callers wire the `src`/`sink` callbacks and set the
// pipeline playing. target is the codec being produced: "opus" (source AAC) or
// "aac" (source Opus).
func buildAudioTranscodePipeline(target string) (gst.Pipeline, error) {
	// Queue sizing: a single qtdemux feeds both branches, so if either queue
	// hits a limit and blocks the demux, the sibling branch starves and mp4mux
	// deadlocks waiting for it (then appsrc backpressures and the Feed blocks).
	// gst's default queue caps at max-size-time=1s, which a long GoP overflows:
	// a 2s GoP overflowed the video queue's time cap and wedged a live stream
	// (TestStreamTranscoderDoubleGopWedge). Use the shared Queue2Big preset
	// (no time/buffer cap, generous byte cap) like the other demux-fed pipelines
	// (rtmp_push, packetize, media_data_parser) so an over-long GoP flows
	// through instead of deadlocking, while memory stays bounded.
	var audioChain string
	switch target {
	case "opus": // source is AAC
		audioChain = constants.Queue2Big + " name=aq ! aacparse ! fdkaacdec ! audioconvert ! audioresample ! opusenc name=aenc"
	case "aac": // source is Opus
		audioChain = constants.Queue2Big + " name=aq ! opusparse ! opusdec ! audioconvert ! audioresample ! fdkaacenc name=aenc"
	default:
		return nil, fmt.Errorf("unsupported transcode target %q", target)
	}

	launched, err := gst.ParseLaunch(strings.Join([]string{
		"appsrc name=src ! qtdemux name=demux",
		"identity name=vclock",
		audioChain,
	}, "\n"))
	if err != nil {
		return nil, fmt.Errorf("create transcode pipeline: %w", err)
	}
	pipeline, ok := launched.(gst.Pipeline)
	if !ok {
		return nil, fmt.Errorf("create transcode pipeline: parsed description is not a pipeline")
	}

	// Fragmented muxer + appsink, built programmatically so we can set the
	// enum fragment-mode and pre-request pads in video-then-audio order.
	mux := gst.ElementFactoryMakeWithProperties("mp4mux", map[string]any{
		"name":              "mux",
		"fragment-mode":     int32(0),
		"fragment-duration": uint32(1),
	})
	if mux == nil {
		return nil, fmt.Errorf("create mp4mux: element factory returned nil")
	}
	sink := gst.ElementFactoryMakeWithProperties("appsink", map[string]any{"name": "sink", "sync": false})
	if sink == nil {
		return nil, fmt.Errorf("create appsink: element factory returned nil")
	}
	if !pipeline.AddMany(mux, sink) {
		return nil, fmt.Errorf("add mux/sink to pipeline failed")
	}
	if !mux.Link(sink) {
		return nil, fmt.Errorf("link mux→sink failed")
	}

	// Pre-request pads in order so the muxer assigns video=track 1, audio=2.
	videoMuxPad := mux.RequestPadSimple("video_%u")
	audioMuxPad := mux.RequestPadSimple("audio_%u")
	if videoMuxPad == nil || audioMuxPad == nil {
		return nil, fmt.Errorf("failed to request mp4mux pads")
	}

	vclock := pipeline.GetByName("vclock")
	if vclock == nil {
		return nil, fmt.Errorf("vclock element not found")
	}
	aenc := pipeline.GetByName("aenc")
	if aenc == nil {
		return nil, fmt.Errorf("aenc element not found")
	}
	// Repair the passthrough video's timestamps before the muxer. Variable-frame-
	// rate WHIP capture can emit several frames at one instant (observed: three
	// frames sharing a PTS, sub-100µs "durations", N/A durations — a capture
	// artifact ffmpeg tolerates). GStreamer's qtdemux collapses those into buffers
	// with a zero duration and then no PTS *and* no DTS; mp4mux rejects a PTS-less
	// buffer ("Buffer has no PTS" → "Could not multiplex stream"), which kills the
	// whole pipeline — so the stream silently loses its transcoded AAC track for
	// the rest of its life (observed in prod on a 1080p60 High-profile Opus
	// stream). The muxed video is only a segmentation clock and is discarded
	// downstream (finishTranscodedSegment keeps just the transcoded audio), so we
	// force a valid, strictly increasing PTS: keep the real one where present,
	// otherwise carry the previous timestamp forward by one step. Keyframes (the
	// GoP/segment cut points) always carry a real PTS, so segment boundaries — and
	// the 1:1 source pairing — are preserved; only degenerate intra-GoP frames are
	// nudged, by at most a few ms within their own GoP.
	const tsStep = gst.ClockTime(1_000_000) // 1 ms — ≥1 tick at any sane video timescale
	var lastTS gst.ClockTime
	haveLast := false
	vclockSrc := vclock.GetStaticPad("src")
	vclockSrc.AddProbe(gst.PadProbeTypeBuffer, func(_ gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		buf := info.GetBuffer()
		if buf == nil {
			return gst.PadProbeOK
		}
		ts := buf.PTS()
		if ts == gst.ClockTimeNone {
			ts = buf.DTS()
		}
		if haveLast && (ts == gst.ClockTimeNone || ts <= lastTS) {
			ts = lastTS + tsStep // missing or non-monotonic → carry forward
		} else if ts == gst.ClockTimeNone {
			ts = 0 // very first buffer with no timestamp at all
		}
		buf.SetPTS(ts)
		if d := buf.Duration(); d == gst.ClockTimeNone || d == 0 {
			buf.SetDuration(tsStep)
		}
		lastTS = ts
		haveLast = true
		return gst.PadProbeOK
	})
	if r := vclockSrc.Link(videoMuxPad); r != gst.PadLinkOK {
		return nil, fmt.Errorf("link video chain → mux: %v", r)
	}
	if r := aenc.GetStaticPad("src").Link(audioMuxPad); r != gst.PadLinkOK {
		return nil, fmt.Errorf("link audio chain → mux: %v", r)
	}

	aq := pipeline.GetByName("aq")
	if aq == nil {
		return nil, fmt.Errorf("aq element not found")
	}
	demux := pipeline.GetByName("demux")
	if demux == nil {
		return nil, fmt.Errorf("demux element not found")
	}
	demux.ConnectPadAdded(func(demux gst.Element, pad gst.Pad) {
		name := pad.GetName()
		switch {
		case name == "video_0":
			if err := plugCutClock(demux, pad, vclock); err != nil {
				demux.PostMessage(gst.NewMessageError(demux, err.Error(), err))
			}
		case strings.HasPrefix(name, "audio"):
			if r := pad.Link(aq.GetStaticPad("sink")); r != gst.PadLinkOK {
				// non-fatal: a stray pad (e.g. a second audio track) is just dropped
				fmt.Printf("transcode: failed to link demux pad %s: %v\n", name, r)
			}
		}
	})

	return pipeline, nil
}

// plugCutClock links pad, the transcode input's video track, through its
// codec's parser into vclock. The passed-through video is muxl's cut clock for
// the transcoded audio: it must cut exactly where the source segments were
// cut, at frames a decoder can start on only (see installKeyframeProbe), or
// the audio chunks and the source segments pair up out of step and the
// completed track drifts from the video.
func plugCutClock(demux gst.Element, pad gst.Pad, vclock gst.Element) error {
	codec, err := padVideoCodec(pad)
	if err != nil {
		return err
	}
	parse, err := plugDemuxBin(demux, pad, constants.Queue2Big+" ! "+codec.ParserLaunch("", ""))
	if err != nil {
		return err
	}
	installKeyframeProbe(context.Background(), codec, parse.GetStaticPad("src"), "completion")
	if !parse.Link(vclock) {
		return fmt.Errorf("link %s → vclock failed", codec.Parser)
	}
	return nil
}

// --- muxl event helpers ---

func unwrapMuxlEvents(ctx context.Context, seg []byte) ([]*muxl.MuxlEvent, error) {
	return collectMuxlEvents(func(ch chan *muxl.MuxlEvent) error {
		return muxl.RunMuxlUnwrapEvents(ctx, bytes.NewReader(seg), ch)
	})
}

func segmentMuxlEvents(ctx context.Context, fmp4 []byte) ([]*muxl.MuxlEvent, error) {
	return collectMuxlEvents(func(ch chan *muxl.MuxlEvent) error {
		return muxl.RunMuxlSegmenterEvents(ctx, bytes.NewReader(fmp4), ch)
	})
}

func collectMuxlEvents(run func(chan *muxl.MuxlEvent) error) ([]*muxl.MuxlEvent, error) {
	ch := make(chan *muxl.MuxlEvent, 16)
	errCh := make(chan error, 1)
	go func() {
		err := run(ch)
		close(ch)
		errCh <- err
	}()
	var out []*muxl.MuxlEvent
	for ev := range ch {
		out = append(out, ev)
	}
	return out, <-errCh
}

// catalogAndTracks pulls the catalog (from the init event) and the per-track
// bytes (from the first segment/signed-segment event) out of a muxl event
// stream.
func catalogAndTracks(events []*muxl.MuxlEvent) (*muxl.MuxlCatalog, map[string][]byte) {
	var cat *muxl.MuxlCatalog
	var tracks map[string][]byte
	for _, ev := range events {
		switch ev.Type {
		case "init":
			if ev.Catalog != nil {
				cat = ev.Catalog
			}
		case "segment", "signed-segment":
			if tracks == nil && len(ev.Tracks) > 0 {
				tracks = ev.Tracks
			}
		}
	}
	return cat, tracks
}

func maxU32(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}

// concatTracksByID reassembles a bare canonical .m4s from a segment event's
// per-track bytes in ascending track-id order (the canonical track order) — the
// inverse of how catalogAndTracks/the segmenter split a segment into ev.Tracks.
func concatTracksByID(tracks map[string][]byte) []byte {
	ids := make([]int, 0, len(tracks))
	for k := range tracks {
		if n, err := strconv.Atoi(k); err == nil {
			ids = append(ids, n)
		}
	}
	sort.Ints(ids)
	var out []byte
	for _, id := range ids {
		out = append(out, tracks[strconv.Itoa(id)]...)
	}
	return out
}

// MuxlTracks splits a bare canonical segment into each track's canonical
// segment, bytes and signatures verbatim, keyed by the track's MUXL
// rendition name ("video1", "audio2").
func MuxlTracks(ctx context.Context, seg []byte) (map[string][]byte, error) {
	events, err := unwrapMuxlEvents(ctx, seg)
	if err != nil {
		return nil, err
	}
	cat, tracks := catalogAndTracks(events)
	if cat == nil || len(tracks) == 0 {
		return nil, fmt.Errorf("segment has no MUXL tracks")
	}
	out := map[string][]byte{}
	add := func(name string, id uint32) {
		if b := tracks[strconv.FormatUint(uint64(id), 10)]; len(b) > 0 {
			out[name] = b
		}
	}
	if cat.Video != nil {
		for name, v := range cat.Video.Renditions {
			add(name, v.TrackID())
		}
	}
	if cat.Audio != nil {
		for name, a := range cat.Audio.Renditions {
			add(name, a.TrackID())
		}
	}
	return out, nil
}

// concatTrackAcrossSegments returns one track's bytes concatenated across EVERY
// segment event in a muxl event stream. catalogAndTracks deliberately returns
// only the first segment's tracks, so it silently drops data whenever a wrapper
// holds more than one segment — which canonicalize produces when it re-segments
// its input into multiple GoPs. Use this when the input may span several.
func concatTrackAcrossSegments(events []*muxl.MuxlEvent, tid uint32) []byte {
	key := strconv.FormatUint(uint64(tid), 10)
	var out []byte
	for _, ev := range events {
		if ev.Type == "segment" || ev.Type == "signed-segment" {
			out = append(out, ev.Tracks[key]...)
		}
	}
	return out
}

// filterSegmentToCodec returns a bare canonical .m4s containing the first
// video track video accepts plus the single audio track matching the
// requested codec (Opus when wantOpus, else AAC) — how an output consumer
// "asks for the media it needs" from a multi-track segment. See
// filterSegmentAudio.
func filterSegmentToCodec(ctx context.Context, seg []byte, wantOpus bool, video func(VideoCodec) bool) ([]byte, error) {
	out, _, err := filterSegmentAudio(ctx, seg, wantOpus, false, video)
	return out, err
}

// playbackAudio describes one audio track kept by filterSegmentAudio.
type playbackAudio struct {
	TrackID uint32
	Group   uint32 // the track's alternateGroup
	Main    bool   // the main group's track; the others are stems
}

// filterSegmentAudio returns a bare canonical .m4s containing the first video
// track video accepts (see pickVideoTrack; every video track when video is
// nil) plus the main group's audio track matching the requested codec (Opus
// when wantOpus, else AAC) and, with stems, one Opus track per stem group
// (see stemOpusTracks). Track bytes are carried verbatim (signatures intact)
// in ascending track-id order; the kept audio tracks are described in that
// same order, which is the order their streams appear in a presentation of
// the result. If no main audio matches the requested codec, any one main
// track is kept (degraded but playable); with no audio info at all, the
// segment is returned unchanged.
func filterSegmentAudio(ctx context.Context, seg []byte, wantOpus, stems bool, video func(VideoCodec) bool) ([]byte, []playbackAudio, error) {
	events, err := unwrapMuxlEvents(ctx, seg)
	if err != nil {
		return nil, nil, fmt.Errorf("unwrap segment for codec filter: %w", err)
	}
	cat, tracks := catalogAndTracks(events)
	if cat == nil {
		return seg, nil, nil
	}

	keep := map[uint32]bool{}
	if video == nil {
		for _, v := range videoTracks(cat) {
			keep[v.TrackID()] = true
		}
	} else if v, _, ok := pickVideoTrack(cat, video); ok {
		keep[v.TrackID()] = true
	} else {
		return nil, nil, errNoVideoTrack
	}
	var audio []playbackAudio
	all := audioTracks(cat)
	if main := mainAudioTracks(all); len(main) > 0 {
		chosen := main[0] // no exact codec match — keep some audio rather than none
		for _, a := range main {
			if (wantOpus && isOpusCodec(a.Codec)) || (!wantOpus && isAACCodec(a.Codec)) {
				chosen = a
				break
			}
		}
		keep[chosen.TrackID()] = true
		audio = append(audio, playbackAudio{TrackID: chosen.TrackID(), Group: chosen.AlternateGroup, Main: true})
	}
	if stems {
		for _, a := range stemOpusTracks(all) {
			keep[a.TrackID()] = true
			audio = append(audio, playbackAudio{TrackID: a.TrackID(), Group: a.AlternateGroup})
		}
	}
	sort.Slice(audio, func(i, j int) bool { return audio[i].TrackID < audio[j].TrackID })

	ids := make([]uint32, 0, len(keep))
	for id := range keep {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	var out []byte
	for _, id := range ids {
		out = append(out, tracks[strconv.FormatUint(uint64(id), 10)]...)
	}
	if len(out) == 0 {
		return seg, nil, nil
	}
	return out, audio, nil
}
