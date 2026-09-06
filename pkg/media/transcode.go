package media

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-gst/go-gst/gst"
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

// transcodeManifest is the C2PA manifest for a transcode-completed audio
// track: it opens the source segment as an ingredient and records a
// c2pa.transcoded action referencing it, both by TranscodeIngredientLabel.
// muxl-sign stamps the per-segment time into cawg.metadata as it signs.
func transcodeManifest(creator string) []byte {
	return []byte(fmt.Sprintf(`{
		"title": "transcoded audio",
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
	}`, muxl.TranscodeIngredientLabel, muxl.TranscodeIngredientLabel, creator))
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
	if cat == nil || cat.Audio == nil || cat.Video == nil {
		return "", false
	}
	var haveAAC, haveOpus bool
	for _, a := range cat.Audio.Renditions {
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
// paths: an `appsrc` of fMP4 → qtdemux → video (h264parse passthrough) + audio
// (decode → re-encode to target) → fragmented mp4mux → `appsink`. Video rides
// along only so muxl can anchor the canonical segment on the video keyframe;
// callers keep the original signed video and use only the transcoded audio
// track. Callers wire the `src`/`sink` callbacks and set the pipeline playing.
// target is the codec being produced: "opus" (source AAC) or "aac" (source Opus).
func buildAudioTranscodePipeline(target string) (*gst.Pipeline, error) {
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

	pipeline, err := gst.NewPipelineFromString(strings.Join([]string{
		"appsrc name=src ! qtdemux name=demux",
		constants.Queue2Big + " name=vq ! h264parse name=vparse",
		audioChain,
	}, "\n"))
	if err != nil {
		return nil, fmt.Errorf("create transcode pipeline: %w", err)
	}

	// Fragmented muxer + appsink, built programmatically so we can set the
	// enum fragment-mode and pre-request pads in video-then-audio order.
	mux, err := gst.NewElementWithProperties("mp4mux", map[string]any{
		"name":              "mux",
		"fragment-mode":     0,
		"fragment-duration": 1,
	})
	if err != nil {
		return nil, fmt.Errorf("create mp4mux: %w", err)
	}
	sink, err := gst.NewElementWithProperties("appsink", map[string]any{"name": "sink", "sync": false})
	if err != nil {
		return nil, fmt.Errorf("create appsink: %w", err)
	}
	if err := pipeline.AddMany(mux, sink); err != nil {
		return nil, fmt.Errorf("add mux/sink: %w", err)
	}
	if err := mux.Link(sink); err != nil {
		return nil, fmt.Errorf("link mux→sink: %w", err)
	}

	// Pre-request pads in order so the muxer assigns video=track 1, audio=2.
	videoMuxPad := mux.GetRequestPad("video_%u")
	audioMuxPad := mux.GetRequestPad("audio_%u")
	if videoMuxPad == nil || audioMuxPad == nil {
		return nil, fmt.Errorf("failed to request mp4mux pads")
	}

	vparse, err := pipeline.GetElementByName("vparse")
	if err != nil {
		return nil, err
	}
	aenc, err := pipeline.GetElementByName("aenc")
	if err != nil {
		return nil, err
	}
	// qtdemux can emit PTS-less / zero-duration buffers (a VFR capture artifact)
	// that mp4mux rejects and would kill the pipeline. The muxed video is only a discarded
	// segmentation clock, so forcing a real, strictly increasing PTS is safe.
	const tsStep = gst.ClockTime(1_000_000) // 1 ms ~ ≥1 tick at any sane video timescale
	var lastTS gst.ClockTime
	haveLast := false
	vparseSrc := vparse.GetStaticPad("src")
	vparseSrc.AddProbe(gst.PadProbeTypeBuffer, func(_ *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		buf := info.GetBuffer()
		if buf == nil {
			return gst.PadProbeOK
		}
		ts := buf.PresentationTimestamp()
		if ts == gst.ClockTimeNone {
			ts = buf.DecodingTimestamp()
		}
		if haveLast && (ts == gst.ClockTimeNone || ts <= lastTS) {
			ts = lastTS + tsStep // missing or non-monotonic → carry forward
		} else if ts == gst.ClockTimeNone {
			ts = 0 // very first buffer with no timestamp at all
		}
		buf.SetPresentationTimestamp(ts)
		if d := buf.Duration(); d == gst.ClockTimeNone || d == 0 {
			buf.SetDuration(tsStep)
		}
		lastTS = ts
		haveLast = true
		return gst.PadProbeOK
	})
	if r := vparseSrc.Link(videoMuxPad); r != gst.PadLinkOK {
		return nil, fmt.Errorf("link video chain → mux: %v", r)
	}
	if r := aenc.GetStaticPad("src").Link(audioMuxPad); r != gst.PadLinkOK {
		return nil, fmt.Errorf("link audio chain → mux: %v", r)
	}

	vq, err := pipeline.GetElementByName("vq")
	if err != nil {
		return nil, err
	}
	aq, err := pipeline.GetElementByName("aq")
	if err != nil {
		return nil, err
	}
	demux, err := pipeline.GetElementByName("demux")
	if err != nil {
		return nil, err
	}
	if _, err := demux.Connect("pad-added", func(self *gst.Element, pad *gst.Pad) {
		name := pad.GetName()
		var dst *gst.Pad
		switch {
		case strings.HasPrefix(name, "video"):
			dst = vq.GetStaticPad("sink")
		case strings.HasPrefix(name, "audio"):
			dst = aq.GetStaticPad("sink")
		default:
			return
		}
		if r := pad.Link(dst); r != gst.PadLinkOK {
			// non-fatal: a stray pad (e.g. a second audio track) is just dropped
			fmt.Printf("transcode: failed to link demux pad %s: %v\n", name, r)
		}
	}); err != nil {
		return nil, fmt.Errorf("connect demux pad-added: %w", err)
	}

	return pipeline, nil
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

// Pick the best video track and optional audio track from a muxl segment. If no
// addressable video track exists (e.g. a single legacy muxl track, TrackID 0),
// returns the input unchanged.
func filterSegmentToSingleTrack(ctx context.Context, seg []byte, wantOpus bool, preferHeight uint32) ([]byte, error) {
	events, err := unwrapMuxlEvents(ctx, seg)
	if err != nil {
		return nil, fmt.Errorf("unwrap segment for single-track filter: %w", err)
	}
	cat, tracks := catalogAndTracks(events)
	if cat == nil || cat.Video == nil || len(cat.Video.Renditions) == 0 {
		return seg, nil
	}

	chosen := selectBestVideoTrackID(cat, preferHeight)
	if chosen == 0 {
		return seg, nil
	}
	keep := map[uint32]bool{chosen: true}
	if audioID := chooseAudioTrackID(cat, wantOpus); audioID != 0 {
		keep[audioID] = true
	}
	return concatTrackBytes(tracks, keep, seg), nil
}

// filterSegmentForRTMPSource is filterSegmentToSingleTrack for RTMP flvmux
// egress: one best video track + AAC audio (RTMP wants AAC).
func filterSegmentForRTMPSource(ctx context.Context, seg []byte, preferHeight uint32) ([]byte, error) {
	return filterSegmentToSingleTrack(ctx, seg, false, preferHeight)
}

// selectBestVideoTrackID returns the best video track ID. Returns 0 if no addressable video track exists.
func selectBestVideoTrackID(cat *muxl.MuxlCatalog, preferHeight uint32) uint32 {
	bestID := uint32(0)
	bestPixels := uint64(0)
	bestWidth := uint32(0)
	atOrBelowID := uint32(0)
	atOrBelowHeight := uint32(0)
	atOrBelowWidth := uint32(0)
	for _, v := range cat.Video.Renditions {
		id, w, h := v.TrackID(), v.CodedWidth, v.CodedHeight
		if id == 0 {
			continue // muxl catalog kind "legacy" (TrackID 0) has no usable CMAF id
		}
		// Top choice: largest coded area (ties broken by width, then lower id).
		if p := uint64(w) * uint64(h); p > bestPixels || (p == bestPixels && w > bestWidth) || (p == bestPixels && w == bestWidth && id < bestID) {
			bestID, bestPixels, bestWidth = id, p, w
		}
		// Preferred choice: tallest that still fits under preferHeight with same tie-breakers.
		if preferHeight > 0 && h <= preferHeight && (h > atOrBelowHeight || (h == atOrBelowHeight && w > atOrBelowWidth) || (h == atOrBelowHeight && w == atOrBelowWidth && id < atOrBelowID)) {
			atOrBelowID, atOrBelowHeight, atOrBelowWidth = id, h, w
		}
	}
	if atOrBelowID != 0 {
		return atOrBelowID
	}
	return bestID
}

// get the single audio track ID
func chooseAudioTrackID(cat *muxl.MuxlCatalog, wantOpus bool) uint32 {
	if cat.Audio == nil || len(cat.Audio.Renditions) == 0 {
		return 0
	}
	for _, a := range cat.Audio.Renditions {
		if (wantOpus && isOpusCodec(a.Codec)) || (!wantOpus && isAACCodec(a.Codec)) {
			return a.TrackID()
		}
	}
	for _, a := range cat.Audio.Renditions { // no exact codec match but we want audio
		return a.TrackID()
	}
	return 0
}

func concatTrackBytes(tracks map[string][]byte, keep map[uint32]bool, seg []byte) []byte {
	// Filter down to the kept track ids, then reuse the canonical numeric-order
	// concatenator. Returns seg unchanged if no kept track has bytes.
	kept := make(map[string][]byte, len(keep))
	for id := range keep {
		key := strconv.FormatUint(uint64(id), 10)
		if b, ok := tracks[key]; ok {
			kept[key] = b
		}
	}
	out := concatTracksSorted(kept)
	if len(out) == 0 {
		return seg
	}
	return out
}
