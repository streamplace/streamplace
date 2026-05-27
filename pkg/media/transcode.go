package media

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/atproto"
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

// completeAudioCodecs ensures a validated segment carries both AAC and Opus
// audio. seg is the bare canonical .m4s for one GoP (all tracks). It inspects
// the embedded catalog; if exactly one audio codec is present it transcodes
// the audio to the other, mints it at a free track id, signs it as a
// c2pa.transcoded derivative of the source audio track (under the node
// identity), and returns seg with the new signed track appended. It is a
// no-op (returns seg unchanged) when both codecs are already present, when
// there is no audio, when there is no video to anchor GoP boundaries, or when
// the audio codec is neither AAC nor Opus.
func (mm *MediaManager) completeAudioCodecs(ctx context.Context, seg []byte) ([]byte, error) {
	// 1. Unwrap the source segment: catalog (codecs/track-ids) + per-track
	//    bytes (the signed source-audio track we'll declare as the parent).
	events, err := unwrapMuxlEvents(ctx, seg)
	if err != nil {
		return nil, fmt.Errorf("unwrap segment: %w", err)
	}
	cat, tracks := catalogAndTracks(events)
	if cat == nil || cat.Audio == nil {
		return seg, nil // no audio to complete
	}

	var (
		haveAAC, haveOpus bool
		srcAudioTID       uint32
		srcAudioCodec     string
		maxTID            uint32
	)
	if cat.Video != nil {
		for _, v := range cat.Video.Renditions {
			maxTID = maxU32(maxTID, v.TrackID())
		}
	}
	for _, a := range cat.Audio.Renditions {
		maxTID = maxU32(maxTID, a.TrackID())
		switch {
		case isAACCodec(a.Codec):
			haveAAC = true
		case isOpusCodec(a.Codec):
			haveOpus = true
		}
		srcAudioTID = a.TrackID()
		srcAudioCodec = a.Codec
	}

	// 2. Decide whether (and to what) we need to transcode.
	if haveAAC && haveOpus {
		return seg, nil // already complete
	}
	var target string
	switch {
	case haveAAC && !haveOpus:
		target = "opus"
	case haveOpus && !haveAAC:
		target = "aac"
	default:
		log.Warn(ctx, "segment audio codec not AAC/Opus, skipping codec completion", "codec", srcAudioCodec)
		return seg, nil
	}
	if cat.Video == nil {
		// No video reference: GoP boundaries can't be anchored cleanly for the
		// transcoded track. Audio-only live is rare; skip for now.
		log.Warn(ctx, "audio-only segment, skipping codec completion")
		return seg, nil
	}

	sourceAudio := tracks[strconv.FormatUint(uint64(srcAudioTID), 10)]
	if len(sourceAudio) == 0 {
		return nil, fmt.Errorf("source audio track %d missing from segment", srcAudioTID)
	}

	cert, keyPEM, err := mm.transcodeSigner()
	if err != nil {
		// No node signing identity (e.g. server repo not initialized) — leave
		// the segment single-codec rather than failing ingest.
		log.Warn(ctx, "node transcode signer unavailable, skipping codec completion", "error", err)
		return seg, nil
	}

	// 3. Wrap the whole segment to a flat MP4 (gstreamer needs video present so
	//    muxl's keyframe-anchored canonicalization yields one aligned segment).
	var flat bytes.Buffer
	if err := muxl.RunMuxlWrap(ctx, bytes.NewReader(seg), "flat", &flat); err != nil {
		return nil, fmt.Errorf("wrap segment for transcode: %w", err)
	}

	// 4. Transcode the audio (video passes through, for GoP alignment only).
	transFmp4, err := transcodeAudioSegment(ctx, flat.Bytes(), target)
	if err != nil {
		return nil, fmt.Errorf("transcode audio to %s: %w", target, err)
	}

	// 5. Find the transcoded audio track's id, then canonicalize remapping it
	//    to a free id so it can join the source tracks without colliding.
	transEvents, err := segmentMuxlEvents(ctx, transFmp4)
	if err != nil {
		return nil, fmt.Errorf("segment transcoded output: %w", err)
	}
	transCat, _ := catalogAndTracks(transEvents)
	if transCat == nil || transCat.Audio == nil {
		return nil, fmt.Errorf("transcoded output has no audio track")
	}
	var transAudioTID uint32
	for _, a := range transCat.Audio.Renditions {
		transAudioTID = a.TrackID()
	}
	freeTID := maxTID + 1
	canon, err := muxl.RunMuxlCanonicalize(ctx, transFmp4, map[uint32]uint32{transAudioTID: freeTID})
	if err != nil {
		return nil, fmt.Errorf("canonicalize transcoded output: %w", err)
	}

	// 6. Extract the (now-free-id) transcoded audio track as a bare unsigned
	//    canonical .m4s — the SignTranscode output.
	canonEvents, err := unwrapMuxlEvents(ctx, canon)
	if err != nil {
		return nil, fmt.Errorf("unwrap canonicalized output: %w", err)
	}
	_, canonTracks := catalogAndTracks(canonEvents)
	output := canonTracks[strconv.FormatUint(uint64(freeTID), 10)]
	if len(output) == 0 {
		return nil, fmt.Errorf("transcoded audio track %d missing after canonicalize", freeTID)
	}

	// 7. Sign the transcoded track as a c2pa.transcoded derivative of the
	//    source audio track, under the node identity.
	signed, err := muxl.RunMuxlSignTranscode(ctx, muxl.TranscodeInput{
		Output:   output,
		Source:   sourceAudio,
		CertPEM:  cert,
		KeyPEM:   keyPEM,
		Manifest: transcodeManifest(mm.cli.BroadcasterDID()),
	})
	if err != nil {
		return nil, fmt.Errorf("sign transcoded track: %w", err)
	}

	// 8. Append the signed track. Its id is the largest, so concatenating it
	//    last preserves the canonical track-id-ascending segment order.
	completed := make([]byte, 0, len(seg)+len(signed))
	completed = append(completed, seg...)
	completed = append(completed, signed...)
	log.Log(ctx, "completed segment audio codecs",
		"source_codec", srcAudioCodec, "added_codec", target,
		"source_track", srcAudioTID, "added_track", freeTID,
		"in_bytes", len(seg), "out_bytes", len(completed))
	return completed, nil
}

// transcodeAudioSegment transcodes the audio of a flat MP4 segment to the
// target codec ("opus" or "aac"), passing video through untouched, and returns
// a fragmented MP4 (video + transcoded audio). Video rides along only so muxl
// can anchor the canonical segment on the video keyframe; the caller keeps the
// original signed video and uses only the transcoded audio track.
func transcodeAudioSegment(ctx context.Context, flat []byte, target string) ([]byte, error) {
	// Watchdog: a per-segment audio transcode is a sub-second real-time job. A
	// stalled gstreamer pipeline only posts non-fatal warnings (not bus errors),
	// so bound it explicitly — a hang surfaces as an error instead of wedging
	// the whole validate/ingest path.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var audioChain string
	switch target {
	case "opus": // source is AAC
		audioChain = "queue name=aq ! aacparse ! fdkaacdec ! audioconvert ! audioresample ! opusenc name=aenc"
	case "aac": // source is Opus
		audioChain = "queue name=aq ! opusparse ! opusdec ! audioconvert ! audioresample ! fdkaacenc name=aenc"
	default:
		return nil, fmt.Errorf("unsupported transcode target %q", target)
	}

	pipeline, err := gst.NewPipelineFromString(strings.Join([]string{
		"appsrc name=src ! qtdemux name=demux",
		"queue name=vq ! h264parse name=vparse",
		audioChain,
	}, "\n"))
	if err != nil {
		return nil, fmt.Errorf("create transcode pipeline: %w", err)
	}
	defer func() {
		if e := pipeline.SetState(gst.StateNull); e != nil {
			log.Error(ctx, "transcode: set null", "error", e)
		}
	}()

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
			log.Error(ctx, "transcode: link demux pad", "name", name, "result", r)
		}
	}); err != nil {
		return nil, fmt.Errorf("connect demux pad-added: %w", err)
	}

	srcEle, err := pipeline.GetElementByName("src")
	if err != nil {
		return nil, err
	}
	app.SrcFromElement(srcEle).SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: ReaderNeedDataIncremental(ctx, bytes.NewReader(flat)),
	})

	var out bytes.Buffer
	app.SinkFromElement(sink).SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, &out),
	})

	errCh := make(chan error, 1)
	go func() { errCh <- HandleBusMessages(ctx, pipeline) }()
	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		return nil, fmt.Errorf("transcode: set playing: %w", err)
	}
	if err := <-errCh; err != nil {
		return nil, fmt.Errorf("transcode pipeline: %w", err)
	}
	if out.Len() == 0 {
		return nil, fmt.Errorf("transcode produced no output")
	}
	return out.Bytes(), nil
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
