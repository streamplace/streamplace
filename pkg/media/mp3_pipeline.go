package media

import (
	"context"
	"errors"
	"fmt"
	"io"
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

var mp3PipelineTracer = otel.Tracer("mp3-pipeline")

// MP3Result is the probe metadata for the MP3 output pipeline. Audio
// describes the *input* track (codec/rate/channels as it came off the
// demuxer); the output is always raw MP3 frames. Transcoded says
// whether the chain re-encoded — false when the input was already MP3
// and we passed it through, true for AAC/Opus.
type MP3Result struct {
	DurationMS int64
	Audio      *VODAudioTrack
	Transcoded bool
}

// RunMP3Pipeline runs the audio-only side of VOD processing: read the
// source media from src (size bytes total), parse it via parsebin,
// land it on the MP3 appsink, and write the resulting raw MP3 byte
// stream to out.
//
// Supported inputs: MP3 (audio/mpeg mpegversion=1, in MP3 or MP4
// containers — passthrough via mpegaudioparse), AAC (mpegversion 2/4,
// decoded via avdec_aac and re-encoded via lamemp3enc), and Opus
// (audio/x-opus, decoded via opusdec and re-encoded). Anything else
// returns ErrUnsupportedCodec.
//
// Video tracks are ignored. Only the first audio pad is wired; extra
// audio pads (rare for a podcast upload) are dropped with a warning.
//
// On success, the returned MP3Result carries input-side probe metadata
// (codec/rate/channels/duration) for the caller's record-creation
// logic. The output is always audio/mpeg mpegversion=1.
func RunMP3Pipeline(ctx context.Context, src io.ReaderAt, size int64, out io.Writer) (MP3Result, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx = log.WithLogValues(ctx, "func", "RunMP3Pipeline")

	ctx, span := mp3PipelineTracer.Start(ctx, "mp3.RunMP3Pipeline", trace.WithAttributes(
		attribute.Int64("source_size_bytes", size),
	))
	defer span.End()

	log.Debug(ctx, "creating pipeline", "source_size", size)

	var result MP3Result

	pipeline, err := gst.NewPipeline("mp3-process")
	if err != nil {
		span.RecordError(err)
		return result, fmt.Errorf("new pipeline: %w", err)
	}
	defer func() {
		if setErr := pipeline.BlockSetState(gst.StateNull); setErr != nil {
			log.Error(ctx, "failed to set pipeline to null", "error", setErr)
		}
	}()

	srcBin, err := RandomAccessSrcBin(ctx, "mp3-src", src, size)
	if err != nil {
		return result, fmt.Errorf("source bin: %w", err)
	}
	if err := pipeline.Add(srcBin.Element); err != nil {
		return result, fmt.Errorf("add src bin: %w", err)
	}

	parsebin, err := gst.NewElementWithProperties("parsebin", map[string]any{
		"name": "mp3-parsebin",
		// expose-all-streams=false: for inputs that need a parser to
		// fixate caps (e.g. MP3 in MP3+ID3v2, where id3demux ->
		// mpegaudioparse needs to see actual frames before rate /
		// channels are known) parsebin's default expose-all-streams=true
		// races pad-added against the caps fixation. Setting false makes
		// parsebin wait until the audio pad has stable caps before
		// surfacing it to us.
		"expose-all-streams": false,
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

	appsink, err := gst.NewElementWithProperties("appsink", map[string]any{
		"name":  "mp3-appsink",
		"sync":  false,
		"async": false,
	})
	if err != nil {
		return result, fmt.Errorf("create appsink: %w", err)
	}
	if err := pipeline.Add(appsink); err != nil {
		return result, fmt.Errorf("add appsink: %w", err)
	}

	sink := app.SinkFromElement(appsink)
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, out),
	})

	var (
		mu        sync.Mutex
		padErr    error
		gotAudio  bool
		audioCaps string
	)

	// processPad runs the per-pad wiring logic once we have caps. It's
	// extracted so we can defer it via notify::caps for inputs (like MP3
	// in MP3+ID3v2) where parsebin exposes the pad before the chain has
	// fully fixated caps.
	processPad := func(pad *gst.Pad) {
		mu.Lock()
		defer mu.Unlock()
		if padErr != nil {
			return
		}
		caps := padCaps(pad)
		if caps == nil {
			log.Warn(ctx, "parsebin pad still missing caps; ignoring", "pad", pad.GetName())
			return
		}
		s := caps.GetStructureAt(0)
		if s == nil {
			log.Warn(ctx, "parsebin caps missing structure; ignoring", "pad", pad.GetName())
			return
		}
		capsName := s.Name()
		padCtx := log.WithLogValues(ctx, "pad", pad.GetName(), "caps", capsName)
		log.Debug(padCtx, "parsebin pad ready")
		switch {
		case strings.HasPrefix(capsName, "video/"):
			log.Debug(padCtx, "ignoring video pad (mp3-only pipeline)")
			return
		case strings.HasPrefix(capsName, "audio/"):
			if gotAudio {
				log.Warn(padCtx, "additional audio pad; ignoring (only one audio stream supported)")
				return
			}
			transcoded, err := wireMP3AudioChain(padCtx, pipeline, appsink, pad, s)
			if err != nil {
				padErr = err
				pipeline.Error("audio chain failed", err)
				return
			}
			gotAudio = true
			audioCaps = capsName
			result.Transcoded = transcoded
			result.Audio = extractAudioTrack(capsName, s, false)
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
	}

	handlerID, err := parsebin.Connect("pad-added", func(_ *gst.Element, pad *gst.Pad) {
		if padCaps(pad) != nil {
			processPad(pad)
			return
		}
		// Caps not ready yet — wait for them via notify::caps.
		// sync.Once guards against re-running if caps notify fires
		// repeatedly (caps can be refined multiple times as a parser
		// learns more about the stream).
		log.Debug(ctx, "parsebin pad has no caps yet; deferring", "pad", pad.GetName())
		var once sync.Once
		if _, cerr := pad.Connect("notify::caps", func() {
			if padCaps(pad) == nil {
				return
			}
			once.Do(func() { processPad(pad) })
		}); cerr != nil {
			mu.Lock()
			if padErr == nil {
				padErr = fmt.Errorf("connect notify::caps: %w", cerr)
				pipeline.Error("notify::caps connect failed", cerr)
			}
			mu.Unlock()
		}
	})
	if err != nil {
		return result, fmt.Errorf("connect pad-added: %w", err)
	}
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
		attribute.Bool("got_audio", gotAudio),
		attribute.String("audio_caps", audioCaps),
	)
	if padErr != nil {
		span.RecordError(padErr)
		span.SetStatus(codes.Error, "pad-wiring")
		return result, padErr
	}
	if !gotAudio {
		err := errors.New("no audio stream found in input")
		span.RecordError(err)
		span.SetStatus(codes.Error, "no_audio")
		return result, err
	}
	return result, nil
}

// wireMP3AudioChain plugs the per-codec audio chain between the
// parsebin audio pad and the MP3 appsink. Returns whether the chain
// re-encoded (i.e., the input wasn't already MP3).
func wireMP3AudioChain(ctx context.Context, pipeline *gst.Pipeline, appsink *gst.Element, pad *gst.Pad, caps *gst.Structure) (transcoded bool, err error) {
	codec := caps.Name()
	specs, transcoded, err := mp3AudioChainSpec(codec, caps)
	if err != nil {
		return false, err
	}
	chain, err := buildChain(pipeline, "mp3-audio", specs)
	if err != nil {
		return false, err
	}
	if err := linkChain(chain); err != nil {
		return false, err
	}
	if ret := pad.Link(chain[0].GetStaticPad("sink")); ret != gst.PadLinkOK {
		return false, fmt.Errorf("link parsebin -> audio chain: %v", ret)
	}
	if err := chain[len(chain)-1].Link(appsink); err != nil {
		return false, fmt.Errorf("link audio chain -> appsink: %w", err)
	}
	syncAll(chain)
	log.Debug(ctx, "wired mp3 audio chain", "codec", codec, "transcoded", transcoded, "elements", chainNames(chain))
	return transcoded, nil
}

// transcodeToMP3 is the suffix used after a decoder when re-encoding to
// MP3. resample + convert insulate the encoder from arbitrary sample
// rates and channel layouts. The trailing mpegaudioparse cleans up the
// lamemp3enc output (sets framed=true, fills in mpegversion/layer/etc.)
// so a downstream consumer sees well-shaped MP3 caps.
var transcodeToMP3 = []elemSpec{
	{name: "audioconvert"},
	{name: "audioresample"},
	{name: "lamemp3enc"},
	{name: "mpegaudioparse"},
}

// mp3AudioChainSpec returns the per-element chain that lands the given
// input codec on the MP3 appsink, plus a flag indicating whether the
// chain re-encodes (i.e., input != MP3).
func mp3AudioChainSpec(codec string, caps *gst.Structure) (specs []elemSpec, transcoded bool, err error) {
	out := []elemSpec{{name: "queue"}}
	switch codec {
	case "audio/mpeg":
		v, _ := caps.GetValue("mpegversion")
		ver, _ := v.(int)
		switch ver {
		case 1:
			// MP3 input — passthrough via mpegaudioparse. Re-parses so
			// frames are framed=true regardless of how the demuxer
			// emitted them, but doesn't decode.
			out = append(out, elemSpec{name: "mpegaudioparse"})
			return out, false, nil
		case 2, 4:
			// AAC — decode (avdec_aac from gst-libav) then re-encode to
			// MP3. We use avdec_aac rather than fdkaacdec because the
			// latter isn't always linked even when fdkaac is enabled.
			out = append(out, elemSpec{name: "aacparse"}, elemSpec{name: "avdec_aac"})
			out = append(out, transcodeToMP3...)
			return out, true, nil
		default:
			return nil, false, fmt.Errorf("%w: audio/mpeg mpegversion=%d", ErrUnsupportedCodec, ver)
		}
	case "audio/x-opus":
		out = append(out, elemSpec{name: "opusdec"})
		out = append(out, transcodeToMP3...)
		return out, true, nil
	default:
		return nil, false, fmt.Errorf("%w: audio codec %q for mp3 output", ErrUnsupportedCodec, codec)
	}
}
