package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/gstinit"
	"stream.place/streamplace/test/remote"
)

func TestPacketize(t *testing.T) {
	withNoGSTLeaks(t, func() {
		g, _ := errgroup.WithContext(context.Background())
		for range streamplaceTestCount {
			g.Go(func() error {
				innerTestPacketize(t, getFixture("sample-segment.mp4"), 49, 40, time.Duration(800*time.Millisecond))
				return nil
			})
		}
		err := g.Wait()
		require.NoError(t, err)
	})
}

func TestPacketizeMuxl(t *testing.T) {
	t.Run("BasicMuxl", func(t *testing.T) {
		withNoGSTLeaks(t, func() {
			filename := remote.RemoteFixture("c6b57a53fc5a2234dbdd388922f0e293d8063d2b30620321e974b7c85640f228/2026-03-17T19-02-08-607Z-muxl_segment_input.fmp4")
			innerTestPacketize(t, filename, 60, 50, time.Duration(1000*time.Millisecond))
		})
	})
	t.Run("ThreeSecondSeg", func(t *testing.T) {
		withNoGSTLeaks(t, func() {
			filename2 := remote.RemoteFixture("5e2bca8cd42ad624d505c73f7b54ec761639416ef556d83145d37b730ed3606a/2026-04-11T22-24-11-527Z-packetize-input-019d7ea5-3d07-7606-b088-1a7bc315d009.mp4")
			innerTestPacketize(t, filename2, 180, 150, time.Duration(3000*time.Millisecond))
		})
	})
	t.Run("TenSecondSeg", func(t *testing.T) {
		withNoGSTLeaks(t, func() {
			filename3 := remote.RemoteFixture("82d20ee62b02f1c3a727b3001f1fa939afb757f9f205fa438d7b5753e1253eef/2026-04-11T22-39-41-861Z-packetize-input-019d7eb3-6f24-776c-ba1b-2f909a2379d7.mp4")
			innerTestPacketize(t, filename3, 300, 502, time.Duration(10040*time.Millisecond))
		})
	})
}

func innerTestPacketize(t *testing.T, filename string, expectedVideo int, expectedAudio int, expectedDuration time.Duration) {
	inputFile, err := os.Open(filename)
	require.NoError(t, err)
	defer inputFile.Close()

	bs, err := io.ReadAll(inputFile)
	require.NoError(t, err)

	testSeg := &bus.Seg{
		Data:     bs,
		Filepath: filename,
	}

	packet, err := Packetize(context.Background(), &config.CLI{}, testSeg)
	require.NoError(t, err)
	require.NotNil(t, packet)
	require.Equal(t, expectedVideo, len(packet.Video))
	require.Equal(t, expectedAudio, len(packet.Audio))
	require.Equal(t, expectedDuration, packet.Duration)
}

// captionSEINAL builds a minimal valid closed-caption SEI NAL (payload_type 4,
// user_data_registered_itu_t_t35, ATSC A/53 "GA94", two CEA-608 control-code
// pairs) — the kind of NAL a stream with embedded captions carries. Raw NAL
// bytes, no length/start-code framing.
func captionSEINAL() []byte {
	t35 := []byte{
		0xb5,       // itu_t_t35_country_code: United States
		0x00, 0x31, // itu_t_t35_provider_code: ATSC
		'G', 'A', '9', '4', // user_identifier
		0x03,             // user_data_type_code: cc_data
		0x40 | 0x02,      // process_cc_data_flag, cc_count=2
		0xff,             // em_data
		0xfc, 0x94, 0xae, // cc_valid, NTSC field 1: ENM
		0xfc, 0x94, 0x20, // cc_valid, NTSC field 1: RCL
		0xff, // marker_bits
	}
	sei := []byte{0x06, 0x04, byte(len(t35))}
	sei = append(sei, t35...)
	return append(sei, 0x80) // rbsp_trailing_bits
}

// makeTrailingCaptionSEIFlatMP4 synthesizes the sample shape MistServer
// produces for a stream with embedded closed captions: a flat fragmented MP4
// whose video samples end with a caption SEI *after* the frame's slice.
// Returns the mp4 and how many samples carry the trailing SEI. When
// h264parse re-parses such a stream it splits each trailing SEI into its own
// slice-less, timestamp-less AU — the shape Packetize must not emit as a
// standalone video frame.
func makeTrailingCaptionSEIFlatMP4(t *testing.T, ctx context.Context, frames, seiEvery int) ([]byte, int) {
	t.Helper()
	gstinit.InitGST()

	// Encode raw AUs in avc stream-format (length-prefixed NALs), so appending
	// a length-prefixed SEI to a sample is valid surgery.
	type au struct {
		data []byte
		pts  gst.ClockTime
		dur  gst.ClockTime
	}
	aus := []au{}
	var vcaps *gst.Caps
	encPipeline, err := gst.NewPipelineFromString(fmt.Sprintf(
		"videotestsrc num-buffers=%d ! video/x-raw,width=320,height=240,framerate=30/1 ! x264enc tune=zerolatency speed-preset=ultrafast key-int-max=30 ! h264parse ! video/x-h264,stream-format=avc,alignment=au ! appsink name=sink", frames))
	require.NoError(t, err)
	sinkEle, err := encPipeline.GetElementByName("sink")
	require.NoError(t, err)
	app.SinkFromElement(sinkEle).SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}
			if vcaps == nil {
				vcaps = sample.GetCaps()
			}
			buffer := sample.GetBuffer()
			aus = append(aus, au{
				data: append([]byte{}, buffer.Bytes()...),
				pts:  gst.ClockTime(buffer.PresentationTimestamp()),
				dur:  gst.ClockTime(buffer.Duration()),
			})
			return gst.FlowOK
		},
	})
	busErr := make(chan error, 1)
	go func() { busErr <- HandleBusMessages(ctx, encPipeline) }()
	require.NoError(t, encPipeline.SetState(gst.StatePlaying))
	require.NoError(t, <-busErr)
	require.NoError(t, encPipeline.SetState(gst.StateNull))
	require.Len(t, aus, frames)
	require.NotNil(t, vcaps)

	// x264enc offsets its output timestamps by a huge constant (its
	// negative-DTS avoidance trick). Rebase to zero so the video timeline
	// lines up with the audio track below — mismatched timelines make qtmux
	// wait forever for the tracks to interleave.
	base := aus[0].pts
	for i := range aus {
		aus[i].pts -= base
	}

	// The surgery: append a caption SEI to every seiEvery-th sample. seiEvery
	// must not divide frames evenly — a trailing SEI on the very last frame
	// has no following frame to ride with and is (acceptably) dropped, which
	// would confuse the survival assertion.
	require.NotZero(t, frames%seiEvery)
	sei := captionSEINAL()
	prefixed := make([]byte, 4+len(sei))
	binary.BigEndian.PutUint32(prefixed, uint32(len(sei)))
	copy(prefixed[4:], sei)
	seiCount := 0
	for i := seiEvery - 1; i < len(aus)-1; i += seiEvery {
		aus[i].data = append(aus[i].data, prefixed...)
		seiCount++
	}
	require.NotZero(t, seiCount)

	// Remux the doctored AUs (plus an Opus track, which Packetize requires)
	// into a fragmented MP4.
	audioBuffers := (frames*48000/30)/1024 + 1
	// Pad names are explicit: the appsrc's caps aren't known at parse time, so
	// without them gst-parse guesses which mux request pad to link (and
	// guesses wrong).
	muxPipeline, err := gst.NewPipelineFromString(strings.Join([]string{
		"mp4mux name=mux fragment-duration=500 ! appsink name=sink",
		"appsrc name=vsrc format=time ! mux.video_0",
		fmt.Sprintf("audiotestsrc num-buffers=%d samplesperbuffer=1024 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! opusenc ! mux.audio_0", audioBuffers),
	}, "\n"))
	require.NoError(t, err)
	vsrcEle, err := muxPipeline.GetElementByName("vsrc")
	require.NoError(t, err)
	require.NoError(t, vsrcEle.SetProperty("caps", vcaps))
	idx := 0
	app.SrcFromElement(vsrcEle).SetCallbacks(&app.SourceCallbacks{
		NeedDataFunc: func(self *app.Source, _ uint) {
			if idx >= len(aus) {
				self.EndStream()
				return
			}
			a := aus[idx]
			idx++
			buffer := gst.NewBufferFromBytes(a.data)
			buffer.SetPresentationTimestamp(a.pts)
			buffer.SetDuration(a.dur)
			self.PushBuffer(buffer)
		},
	})
	outSinkEle, err := muxPipeline.GetElementByName("sink")
	require.NoError(t, err)
	var out bytes.Buffer
	app.SinkFromElement(outSinkEle).SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: WriterNewSample(ctx, &out),
	})
	busErr2 := make(chan error, 1)
	go func() { busErr2 <- HandleBusMessages(ctx, muxPipeline) }()
	require.NoError(t, muxPipeline.SetState(gst.StatePlaying))
	require.NoError(t, <-busErr2)
	require.NoError(t, muxPipeline.SetState(gst.StateNull))
	require.NotEmpty(t, out.Bytes())
	return out.Bytes(), seiCount
}

// countCaptionSEIs counts caption SEI NALs (nal type 6, payload_type 4) in
// byte-stream H264 data.
func countCaptionSEIs(data []byte) int {
	count := 0
	for i := 0; i+4 < len(data); i++ {
		if data[i] != 0 || data[i+1] != 0 {
			continue
		}
		var nalIdx int
		if data[i+2] == 1 {
			nalIdx = i + 3
		} else if data[i+2] == 0 && i+5 < len(data) && data[i+3] == 1 {
			nalIdx = i + 4
		} else {
			continue
		}
		if data[nalIdx]&0x1f == 6 && data[nalIdx+1] == 0x04 {
			count++
		}
		i = nalIdx // skip the matched start code (else a 4-byte code re-matches as 3-byte)
	}
	return count
}

// TestPacketizeTrailingCaptionSEI: streams with embedded closed captions
// carry trailing caption SEIs that h264parse splits into slice-less,
// timestamp-less AUs. Packetize must fold those into the next real frame —
// not emit them as standalone video "frames", which inflate the frame count
// (skewing the sender's synthesized timing) and break strict WebRTC decoders
// (iOS VideoToolbox errors on a picture-less access unit).
func TestPacketizeTrailingCaptionSEI(t *testing.T) {
	withNoGSTLeaks(t, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		const frames = 60
		flat, seiCount := makeTrailingCaptionSEIFlatMP4(t, ctx, frames, 7)

		packet, err := Packetize(context.Background(), &config.CLI{}, &bus.Seg{Data: flat})
		require.NoError(t, err)
		require.NotNil(t, packet)

		// Exactly one output sample per input frame — caption SEIs must not
		// become frames of their own.
		require.Equal(t, frames, len(packet.Video))
		totalSEIs := 0
		for i, v := range packet.Video {
			require.True(t, hasVideoSlice(v), "video sample %d has no picture", i)
			totalSEIs += countCaptionSEIs(v)
		}
		// ...and the captions must survive, riding with real frames.
		require.Equal(t, seiCount, totalSEIs)
		require.NotEmpty(t, packet.Audio)
	})
}

func TestPacketizeInvalid(t *testing.T) {
	// cur := goleak.IgnoreCurrent()
	// defer goleak.VerifyNone(t, cur)
	withNoGSTLeaks(t, func() {
		rng := rand.New(rand.NewSource(42))
		randomData := make([]byte, 1024*1024) // 1MB
		_, err := rng.Read(randomData)
		require.NoError(t, err)
		packet, err := Packetize(context.Background(), &config.CLI{}, &bus.Seg{
			Data: randomData,
		})
		require.Error(t, err)
		require.Nil(t, packet)
	})
}
