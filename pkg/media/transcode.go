package media

import (
	"bytes"
	"context"
	"fmt"
	"sort"
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
	if r := vparse.GetStaticPad("src").Link(videoMuxPad); r != gst.PadLinkOK {
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

// filterSegmentToCodec returns a bare canonical .m4s containing every video
// track plus the single audio track matching the requested codec (Opus when
// wantOpus, else AAC) — how an output consumer "asks for the audio it needs"
// from a dual-codec segment. Track bytes are carried verbatim (signatures
// intact) in ascending track-id order. If no audio matches the requested
// codec, any one audio track is kept (degraded but playable); with no audio
// info at all, the segment is returned unchanged.
func filterSegmentToCodec(ctx context.Context, seg []byte, wantOpus bool) ([]byte, error) {
	events, err := unwrapMuxlEvents(ctx, seg)
	if err != nil {
		return nil, fmt.Errorf("unwrap segment for codec filter: %w", err)
	}
	cat, tracks := catalogAndTracks(events)
	if cat == nil {
		return seg, nil
	}

	keep := map[uint32]bool{}
	if cat.Video != nil {
		for _, v := range cat.Video.Renditions {
			keep[v.TrackID()] = true
		}
	}
	if cat.Audio != nil && len(cat.Audio.Renditions) > 0 {
		var chosen uint32
		found := false
		for _, a := range cat.Audio.Renditions {
			if (wantOpus && isOpusCodec(a.Codec)) || (!wantOpus && isAACCodec(a.Codec)) {
				chosen, found = a.TrackID(), true
				break
			}
		}
		if !found { // no exact codec match — keep some audio rather than none
			for _, a := range cat.Audio.Renditions {
				chosen, found = a.TrackID(), true
				break
			}
		}
		if found {
			keep[chosen] = true
		}
	}

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
		return seg, nil
	}
	return out, nil
}
