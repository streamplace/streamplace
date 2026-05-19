package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"stream.place/streamplace/pkg/log"
)

var vodPipelineTracer = otel.Tracer("vod-pipeline")

// elementaryStreamCaps lists the caps names that come out of a
// demuxer as already-elementary (one codec per pad). We use this set
// in parsebin's autoplug-continue callback to tell parsebin "stop
// here, don't plug another parser" — wireVideoChain and
// wireAudioChain plug all the parsing/decoding we need downstream.
// Keep this in sync with the codecs handled there.
var elementaryStreamCaps = map[string]bool{
	"video/x-h264": true,
	"video/x-h265": true, // unsupported; we want a clean ErrUnsupportedCodec, not a crash
	"audio/mpeg":   true, // AAC (only — MP3 has no decoder linked, will fail in audioChainSpec)
	"audio/x-opus": true,
}

// ErrUnsupportedCodec is returned by RunVODPipeline when the input
// contains a stream we can't process. Currently: any non-h264 video,
// or audio that isn't AAC or Opus. Opus is the only audio codec we
// transcode (to AAC via fdkaacenc); AAC passes through.
var ErrUnsupportedCodec = errors.New("unsupported codec")

// VODResult bundles probe-derived metadata that the caller needs for
// downstream record creation. Populated incrementally by the pad-added
// handler (video/audio codec + dimensions/rate from parsebin caps) and
// finalized after EOS with a duration query against the pipeline.
type VODResult struct {
	// DurationMS is the duration of the source as reported by gstreamer
	// after EOS, in milliseconds. Zero if the duration query failed
	// (rare; bug or live source).
	DurationMS int64
	// Video is the video track's probe metadata. nil if no video pad
	// was wired (only happens when the input is audio-only, which the
	// pipeline rejects today via "no video stream found").
	Video *VODVideoTrack
	// Audio is the audio track's probe metadata. nil if the input had
	// no audio.
	Audio *VODAudioTrack
}

// VODVideoTrack is per-track probe metadata for a video stream.
// Codec is the gstreamer caps name without the leading "video/" (e.g.
// "x-h264"); width/height are pixels; framerate is the parsebin-
// reported frame rate in num/den form (often non-integer for variable-
// frame-rate content).
type VODVideoTrack struct {
	Codec  string
	Width  int
	Height int
	FPSNum int
	FPSDen int
}

// VODAudioTrack is per-track probe metadata for an audio stream. Codec
// is the gstreamer caps name (e.g. "x-opus", "mpeg"). For audio/mpeg
// callers should also consult MPEGVersion to disambiguate AAC vs MP3.
type VODAudioTrack struct {
	Codec       string
	Rate        int
	Channels    int
	MPEGVersion int
}

// RunVODPipeline runs the gstreamer side of VOD processing: read the
// source media from src (size bytes total), parse it via parsebin,
// re-mux it to fragmented MP4, and write that fMP4 to out. Audio is
// transcoded to AAC via fdkaacenc when not already AAC; video must be
// h264 — anything else returns ErrUnsupportedCodec.
//
// The function blocks until the input is fully consumed or an error
// occurs. out is written from a gstreamer streaming thread, so it must
// be safe for concurrent use with this goroutine (typically an io.Pipe
// or a buffered writer fed by a single reader).
//
// On success, the returned VODResult carries probe metadata gathered
// during the run (track codec/dimensions, total duration) for use by
// the caller's downstream record-creation logic.
func RunVODPipeline(ctx context.Context, src io.ReaderAt, size int64, out io.Writer) (VODResult, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx = log.WithLogValues(ctx, "func", "RunVODPipeline")

	ctx, span := vodPipelineTracer.Start(ctx, "vod.RunVODPipeline", trace.WithAttributes(
		attribute.Int64("source_size_bytes", size),
	))
	defer span.End()

	log.Debug(ctx, "creating pipeline", "source_size", size)

	var result VODResult

	pipeline, err := gst.NewPipeline("vod-process")
	if err != nil {
		span.RecordError(err)
		return result, fmt.Errorf("new pipeline: %w", err)
	}
	defer func() {
		if setErr := pipeline.BlockSetState(gst.StateNull); setErr != nil {
			log.Error(ctx, "failed to set pipeline to null", "error", setErr)
		}
	}()

	srcBin, err := RandomAccessSrcBin(ctx, "vod-src", src, size)
	if err != nil {
		return result, fmt.Errorf("source bin: %w", err)
	}
	if err := pipeline.Add(srcBin.Element); err != nil {
		return result, fmt.Errorf("add src bin: %w", err)
	}

	parsebin, err := gst.NewElementWithProperties("parsebin", map[string]any{
		"name":               "vod-parsebin",
		"expose-all-streams": true,
	})
	if err != nil {
		return result, fmt.Errorf("create parsebin: %w", err)
	}
	if err := pipeline.Add(parsebin); err != nil {
		return result, fmt.Errorf("add parsebin: %w", err)
	}
	if err := srcBin.Link(parsebin); err != nil {
		return result, fmt.Errorf("link src bin -> parsebin: %w", err)
	}

	// Tell parsebin's autoplug machinery to stop at the elementary-stream
	// pads coming out of the demuxer. We plug our own h264parse / aacparse
	// / mpegaudioparse / opusdec etc. in wireVideoChain and wireAudioChain
	// downstream, so parsebin doesn't need to add another parser on top.
	//
	// Returning FALSE here is also our workaround for a parsebin bug
	// where audio/mpeg (AAC) triggers an unbounded recursion: aacparse-0
	// fixes its src caps on the first frame, the resulting "caps" notify
	// re-enters parsebin's pad_added_cb, which plugs aacparse-1, whose
	// caps notify plugs aacparse-2, and so on until the cgo stack
	// overflows and the process crashes. Stopping autoplug before any
	// parser is added keeps that loop from ever starting.
	autoplugID, err := parsebin.Connect("autoplug-continue", func(_ *gst.Element, _ *gst.Pad, caps *gst.Caps) bool {
		if caps == nil {
			return true
		}
		s := caps.GetStructureAt(0)
		if s == nil {
			return true
		}
		return !elementaryStreamCaps[s.Name()]
	})
	if err != nil {
		return result, fmt.Errorf("connect autoplug-continue: %w", err)
	}
	defer parsebin.HandlerDisconnect(autoplugID)

	mp4mux, err := gst.NewElementWithProperties("mp4mux", map[string]any{
		"name":              "vod-mp4mux",
		"fragment-mode":     0,
		"fragment-duration": 1,
	})
	if err != nil {
		return result, fmt.Errorf("create mp4mux: %w", err)
	}
	if err := pipeline.Add(mp4mux); err != nil {
		return result, fmt.Errorf("add mp4mux: %w", err)
	}

	appsink, err := gst.NewElementWithProperties("appsink", map[string]any{
		"name":  "vod-appsink",
		"sync":  false,
		"async": false,
	})
	if err != nil {
		return result, fmt.Errorf("create appsink: %w", err)
	}
	if err := pipeline.Add(appsink); err != nil {
		return result, fmt.Errorf("add appsink: %w", err)
	}
	if err := mp4mux.Link(appsink); err != nil {
		return result, fmt.Errorf("link mp4mux -> appsink: %w", err)
	}

	sink := app.SinkFromElement(appsink)
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, out),
	})

	// padErr captures the first wiring error from pad-added so we can
	// surface it after the bus loop returns. gotVideo/gotAudio prevent us
	// from blindly wiring multiple streams of the same kind into mp4mux
	// (we'd just get the first one; warn the user about extras).
	// videoCaps/audioCaps capture the actual codec strings so we can
	// attach them to the trace span.
	var (
		mu        sync.Mutex
		padErr    error
		gotVideo  bool
		gotAudio  bool
		videoCaps string
		audioCaps string
	)

	handlerID, err := parsebin.Connect("pad-added", func(_ *gst.Element, pad *gst.Pad) {
		mu.Lock()
		defer mu.Unlock()
		if padErr != nil {
			return
		}
		caps := padCaps(pad)
		if caps == nil {
			log.Warn(ctx, "parsebin pad missing caps; ignoring", "pad", pad.GetName())
			return
		}
		s := caps.GetStructureAt(0)
		if s == nil {
			log.Warn(ctx, "parsebin caps missing structure; ignoring", "pad", pad.GetName())
			return
		}
		capsName := s.Name()
		padCtx := log.WithLogValues(ctx, "pad", pad.GetName(), "caps", capsName)
		log.Debug(padCtx, "parsebin pad-added")
		switch {
		case strings.HasPrefix(capsName, "video/"):
			if gotVideo {
				log.Warn(padCtx, "additional video pad; ignoring (only one video stream supported)")
				return
			}
			if err := wireVideoChain(padCtx, pipeline, mp4mux, pad, s); err != nil {
				padErr = err
				pipeline.Error("video chain failed", err)
				return
			}
			gotVideo = true
			videoCaps = capsName
			result.Video = extractVideoTrack(capsName, s)
			log.Debug(padCtx, "video probe",
				"codec", result.Video.Codec,
				"width", result.Video.Width,
				"height", result.Video.Height,
				"fps_num", result.Video.FPSNum,
				"fps_den", result.Video.FPSDen,
			)
		case strings.HasPrefix(capsName, "audio/"):
			if gotAudio {
				log.Warn(padCtx, "additional audio pad; ignoring (only one audio stream supported)")
				return
			}
			transcoded, err := wireAudioChain(padCtx, pipeline, mp4mux, pad, s)
			if err != nil {
				padErr = err
				pipeline.Error("audio chain failed", err)
				return
			}
			gotAudio = true
			audioCaps = capsName
			// After transcoding (opus/mp3/etc -> AAC), the audio track
			// in the final fMP4 is AAC. We report what the *output*
			// looks like, not the input. For passthrough AAC we keep
			// the caps as-is.
			result.Audio = extractAudioTrack(capsName, s, transcoded)
			log.Debug(padCtx, "audio probe",
				"codec", result.Audio.Codec,
				"rate", result.Audio.Rate,
				"channels", result.Audio.Channels,
				"transcoded", transcoded,
			)
			span.SetAttributes(attribute.Bool("audio_transcode", transcoded))
		default:
			log.Warn(padCtx, "non-A/V parsebin pad; ignoring")
		}
	})
	if err != nil {
		return result, fmt.Errorf("connect pad-added: %w", err)
	}
	// Disconnecting before pipeline tear-down breaks the closure->pipeline
	// reference cycle that would otherwise pin every element alive past
	// the function's return.
	defer parsebin.HandlerDisconnect(handlerID)

	busErr := make(chan error, 1)
	go func() { busErr <- HandleBusMessages(ctx, pipeline) }()

	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		span.RecordError(err)
		return result, fmt.Errorf("set playing: %w", err)
	}
	log.Debug(ctx, "pipeline transitioned to PLAYING")

	if err := <-busErr; err != nil {
		mu.Lock()
		pe := padErr
		mu.Unlock()
		if pe != nil {
			span.RecordError(pe)
			span.SetStatus(codes.Error, "pad-wiring")
			return result, pe
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "bus")
		return result, fmt.Errorf("pipeline: %w", err)
	}
	log.Debug(ctx, "pipeline EOS reached cleanly")

	// Query the pipeline's duration before tearing it down. gstreamer
	// reports it in nanoseconds (FormatTime); we expose milliseconds
	// because that's what place.stream.video's duration field takes.
	if ok, durNS := pipeline.QueryDuration(gst.FormatTime); ok {
		result.DurationMS = durNS / int64(time.Millisecond)
		span.SetAttributes(attribute.Int64("duration_ms", result.DurationMS))
		log.Debug(ctx, "queried pipeline duration", "duration_ms", result.DurationMS)
	} else {
		log.Warn(ctx, "pipeline duration query failed; record will report 0 ms")
	}

	mu.Lock()
	defer mu.Unlock()
	span.SetAttributes(
		attribute.Bool("got_video", gotVideo),
		attribute.Bool("got_audio", gotAudio),
		attribute.String("video_caps", videoCaps),
		attribute.String("audio_caps", audioCaps),
	)
	if padErr != nil {
		span.RecordError(padErr)
		span.SetStatus(codes.Error, "pad-wiring")
		return result, padErr
	}
	if !gotVideo {
		err := fmt.Errorf("no video stream found in input")
		span.RecordError(err)
		span.SetStatus(codes.Error, "no_video")
		return result, err
	}
	return result, nil
}

// extractVideoTrack pulls codec/dimensions/framerate out of a parsebin
// video pad's caps Structure. The framerate is a GstFraction that
// go-gst doesn't expose typed access for, so we stringify and parse
// num/den ourselves — same workaround the existing media_data_parser
// uses for live-segment probing.
func extractVideoTrack(capsName string, s *gst.Structure) *VODVideoTrack {
	t := &VODVideoTrack{Codec: strings.TrimPrefix(capsName, "video/")}
	if v, _ := s.GetValue("width"); v != nil {
		if w, ok := v.(int); ok {
			t.Width = w
		}
	}
	if v, _ := s.GetValue("height"); v != nil {
		if h, ok := v.(int); ok {
			t.Height = h
		}
	}
	if v, _ := s.GetValue("framerate"); v != nil {
		t.FPSNum, t.FPSDen = parseFraction(v)
	}
	return t
}

// extractAudioTrack pulls codec/rate/channels out of a parsebin audio
// pad's caps Structure. When the chain transcodes to AAC (the common
// case for anything that isn't already AAC), the reported codec is
// "mpeg" with mpegversion=4, since that's the actual MP4 output —
// callers downstream care about what landed in the final blob, not
// what came in.
func extractAudioTrack(capsName string, s *gst.Structure, transcoded bool) *VODAudioTrack {
	t := &VODAudioTrack{Codec: strings.TrimPrefix(capsName, "audio/")}
	if v, _ := s.GetValue("rate"); v != nil {
		if r, ok := v.(int); ok {
			t.Rate = r
		}
	}
	if v, _ := s.GetValue("channels"); v != nil {
		if c, ok := v.(int); ok {
			t.Channels = c
		}
	}
	if v, _ := s.GetValue("mpegversion"); v != nil {
		if mv, ok := v.(int); ok {
			t.MPEGVersion = mv
		}
	}
	if transcoded {
		// The chain re-encoded to AAC; the final fMP4 carries AAC
		// regardless of what came in.
		t.Codec = "mpeg"
		t.MPEGVersion = 4
	}
	return t
}

// parseFraction takes a GstFraction-shaped value from a caps Structure
// (go-gst returns it as a stringer with "num/den" format) and returns
// the numerator and denominator. Returns 0,0 on any parse failure.
func parseFraction(v any) (num, den int) {
	str := fmt.Sprintf("%v", v)
	parts := strings.SplitN(str, "/", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	n, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	d, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return 0, 0
	}
	return n, d
}

type elemSpec struct {
	name  string
	props map[string]any
}

func wireVideoChain(ctx context.Context, pipeline *gst.Pipeline, mp4mux *gst.Element, pad *gst.Pad, caps *gst.Structure) error {
	codec := caps.Name()
	if codec != "video/x-h264" {
		return fmt.Errorf("%w: video codec %q (only h264 is supported)", ErrUnsupportedCodec, codec)
	}
	chain, err := buildChain(pipeline, "vod-video", []elemSpec{
		{name: "queue"},
		{name: "h264parse", props: map[string]any{"config-interval": 0, "disable-passthrough": true}},
	})
	if err != nil {
		return err
	}
	if err := linkChain(chain); err != nil {
		return err
	}
	sinkPad := mp4mux.GetRequestPad("video_%u")
	if sinkPad == nil {
		return fmt.Errorf("mp4mux: no video request pad")
	}
	if ret := pad.Link(chain[0].GetStaticPad("sink")); ret != gst.PadLinkOK {
		return fmt.Errorf("link parsebin -> video chain: %v", ret)
	}
	if ret := chain[len(chain)-1].GetStaticPad("src").Link(sinkPad); ret != gst.PadLinkOK {
		return fmt.Errorf("link video chain -> mp4mux: %v", ret)
	}
	syncAll(chain)
	log.Debug(ctx, "wired video chain", "codec", codec, "elements", chainNames(chain))
	return nil
}

// wireAudioChain returns whether the chain inserted a transcode step
// (i.e., the input wasn't already AAC).
func wireAudioChain(ctx context.Context, pipeline *gst.Pipeline, mp4mux *gst.Element, pad *gst.Pad, caps *gst.Structure) (transcoded bool, err error) {
	codec := caps.Name()
	specs, transcoded, err := audioChainSpec(codec, caps)
	if err != nil {
		return false, err
	}
	chain, err := buildChain(pipeline, "vod-audio", specs)
	if err != nil {
		return false, err
	}
	if err := linkChain(chain); err != nil {
		return false, err
	}
	sinkPad := mp4mux.GetRequestPad("audio_%u")
	if sinkPad == nil {
		return false, fmt.Errorf("mp4mux: no audio request pad")
	}
	if ret := pad.Link(chain[0].GetStaticPad("sink")); ret != gst.PadLinkOK {
		return false, fmt.Errorf("link parsebin -> audio chain: %v", ret)
	}
	if ret := chain[len(chain)-1].GetStaticPad("src").Link(sinkPad); ret != gst.PadLinkOK {
		return false, fmt.Errorf("link audio chain -> mp4mux: %v", ret)
	}
	syncAll(chain)
	log.Debug(ctx, "wired audio chain", "codec", codec, "transcoded", transcoded, "elements", chainNames(chain))
	return transcoded, nil
}

func chainNames(elems []*gst.Element) []string {
	names := make([]string, len(elems))
	for i, e := range elems {
		names[i] = e.GetName()
	}
	return names
}

// transcodeToAAC is the suffix used after a decoder when re-encoding to
// AAC for ingest into mp4mux. resample + convert insulate the encoder
// from arbitrary sample rates and channel layouts.
var transcodeToAAC = []elemSpec{
	{name: "audioconvert"},
	{name: "audioresample"},
	{name: "fdkaacenc"},
	{name: "aacparse"},
}

// audioChainSpec returns the per-element chain needed to land the given
// codec on mp4mux's audio sink, plus a flag indicating whether the chain
// re-encodes (i.e., input != AAC). Only the codecs whose decoders/encoders
// are actually linked into the static build are supported here: AAC
// (aacparse passthrough) and Opus (opusdec → fdkaacenc). MP3, Vorbis,
// FLAC, etc. are rejected with ErrUnsupportedCodec — the decoders they'd
// need (mpg123audiodec / vorbisdec / flacdec) aren't in the plugin list
// in the Makefile.
func audioChainSpec(codec string, caps *gst.Structure) (specs []elemSpec, transcoded bool, err error) {
	out := []elemSpec{{name: "queue"}}
	switch codec {
	case "audio/mpeg":
		v, _ := caps.GetValue("mpegversion")
		ver, _ := v.(int)
		if ver != 2 && ver != 4 {
			return nil, false, fmt.Errorf("%w: audio/mpeg mpegversion=%d", ErrUnsupportedCodec, ver)
		}
		// AAC (mpegversion 2 = MPEG-2 AAC, 4 = MPEG-4 AAC); pass through.
		out = append(out, elemSpec{name: "aacparse"})
		return out, false, nil
	case "audio/x-opus":
		out = append(out, elemSpec{name: "opusdec"})
		out = append(out, transcodeToAAC...)
		return out, true, nil
	default:
		return nil, false, fmt.Errorf("%w: audio codec %q", ErrUnsupportedCodec, codec)
	}
}

func buildChain(pipeline *gst.Pipeline, prefix string, specs []elemSpec) ([]*gst.Element, error) {
	out := make([]*gst.Element, len(specs))
	for i, spec := range specs {
		props := map[string]any{}
		for k, v := range spec.props {
			props[k] = v
		}
		props["name"] = fmt.Sprintf("%s-%s-%d", prefix, spec.name, i)
		e, err := gst.NewElementWithProperties(spec.name, props)
		if err != nil {
			return nil, fmt.Errorf("create %s: %w", spec.name, err)
		}
		if err := pipeline.Add(e); err != nil {
			return nil, fmt.Errorf("add %s: %w", spec.name, err)
		}
		out[i] = e
	}
	return out, nil
}

func linkChain(elems []*gst.Element) error {
	for i := 0; i < len(elems)-1; i++ {
		if err := elems[i].Link(elems[i+1]); err != nil {
			return fmt.Errorf("link %s -> %s: %w", elems[i].GetName(), elems[i+1].GetName(), err)
		}
	}
	return nil
}

func syncAll(elems []*gst.Element) {
	for _, e := range elems {
		e.SyncStateWithParent()
	}
}

// padCaps resolves a pad's current caps with fallbacks for the case
// parsebin emits its source pads before the caps event has propagated:
//
//  1. GetCurrentCaps — fast path, works once a CAPS event has fixed
//     the pad. For many inputs this is set at pad-added time.
//  2. GetStream().Caps — parsebin attaches a GstStream to each
//     exposed pad with the codec caps from its internal parser,
//     even before the downstream caps query has fixated. Works
//     reliably for parsebin and decodebin3.
//  3. GetAllowedCaps — fallback intersection of the pad's template
//     and the (still-empty) peer caps. Rarely fixes a codec name
//     directly but catches typed pad templates as a last resort.
//
// Returns nil if all three fall short, in which case the caller
// should log+ignore the pad — there's no productive way to wire it.
func padCaps(pad *gst.Pad) *gst.Caps {
	if c := pad.GetCurrentCaps(); c != nil {
		return c
	}
	if stream := pad.GetStream(); stream != nil {
		if c := stream.Caps(); c != nil {
			return c
		}
	}
	if c := pad.GetAllowedCaps(); c != nil {
		return c
	}
	return nil
}
