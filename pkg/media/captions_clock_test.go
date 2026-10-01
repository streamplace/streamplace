package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/captions/fmp4"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/muxl"
)

func signedMediaDate(t *testing.T, ctx context.Context, data []byte) time.Time {
	t.Helper()
	output, err := muxl.RunMuxlVerify(ctx, bytes.NewReader(data))
	require.NoError(t, err)
	var doc struct {
		Segments []struct {
			Manifest struct {
				Assertions []struct {
					Label string                     `json:"label"`
					Data  map[string]json.RawMessage `json:"data"`
				} `json:"assertions"`
			} `json:"manifest"`
		} `json:"segments"`
	}
	require.NoError(t, json.Unmarshal([]byte(output), &doc))
	for _, segment := range doc.Segments {
		for _, a := range segment.Manifest.Assertions {
			if a.Label == "cawg.metadata" {
				var date string
				require.NoError(t, json.Unmarshal(a.Data["dc:date"], &date))
				parsed, err := time.Parse(time.RFC3339Nano, date)
				require.NoError(t, err)
				return parsed
			}
		}
	}
	t.Fatal("signed segment has no media date")
	return time.Time{}
}
func TestCaptionMasterSignedDatesFollowMediaNotSigning(t *testing.T) {
	ctx := context.Background()
	fixture, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)
	tracks, err := fmp4.Tracks(fixture)
	require.NoError(t, err)
	var timescale uint32
	for _, track := range tracks {
		if track.ID == 1 {
			timescale = track.Timescale
		}
	}
	mm := NewOffline(&config.CLI{})
	ms := newBareSegmentSigner(t)
	ms.PrebuiltManifest = captionManifest("off")
	events := make(chan *muxl.MuxlEvent, 16)
	done := make(chan error, 1)
	input := newIngestByteBuffer(context.Background())
	wall := time.Now()
	source := bytes.NewReader(fixture)
	var moof []byte
	for source.Len() > 0 {
		box, kind, err := readCaptionBox(source)
		require.NoError(t, err)
		at := wall
		if kind == "moof" {
			moof = box
		}
		if kind == "mdat" && len(moof) > 0 {
			fragments, err := fmp4.Fragments(append(moof, box...))
			require.NoError(t, err)
			for _, fragment := range fragments {
				for _, track := range tracks {
					if fragment.TrackID == track.ID {
						at = wall.Add(time.Duration(fragment.BaseDecodeTime) * time.Second / time.Duration(track.Timescale))
						break
					}
				}
				break
			}
		}
		_, err = input.writeAt(box, at)
		require.NoError(t, err)
	}
	require.NoError(t, input.Close())
	var signed []*muxl.MuxlEvent
	go func() { done <- mm.SignOriginStream(ctx, ms, input, events); close(events) }()
	for event := range events {
		if event.Type == "signed-segment" {
			signed = append(signed, event)
		}
	}
	require.NoError(t, <-done)
	require.Len(t, signed, 2)
	first := signedMediaDate(t, ctx, signed[0].Tracks["1"])
	second := signedMediaDate(t, ctx, signed[1].Tracks["1"])
	expected := time.Duration((signed[1].FirstDecodeTimes["1"]-signed[0].FirstDecodeTimes["1"])*1000/uint64(timescale)) * time.Millisecond
	require.Equal(t, expected, second.Sub(first), "queued GoPs must retain their media-clock spacing")
	require.WithinDuration(t, wall, first, time.Millisecond, "the initial signed date must use arrival, not signing time")
}

func TestCaptionMasterClockReanchorsPushAndSignedTimeTogether(t *testing.T) {
	ctx := context.Background()
	m := newCaptionMaster(ctx, "streamer", &config.CLI{}, nil)
	m.setManifest(captionManifest("ingest"))
	m.mediaFinished = true
	wall := time.UnixMilli(1800000000000)
	m.clockAt(time.UnixMilli(0), wall)
	m.closeGopAt(1000, wall.Add(time.Second))
	require.Equal(t, wall.Add(time.Second), m.segmentTime(1000))
	m.closeGopAt(2000, wall.Add(5*time.Second))
	segmentWall := m.segmentTime(2000)
	require.Equal(t, wall.Add(5*time.Second), segmentWall)
	start, end := segmentWall.Add(200*time.Millisecond), segmentWall.Add(400*time.Millisecond)
	require.NoError(t, m.push(masterTrack(captions.SourceHuman), []captions.Cue{{ID: "reanchored", Start: start, End: end, Text: "after stall", Final: true}}))
	attached, err := m.text(ctx, muxl.TextRequest{StartMs: 2000, EndMs: 3000})
	require.NoError(t, err)
	require.Len(t, attached.Tracks, 1)
	require.Len(t, attached.Tracks[0].Cues, 1)
	cue := attached.Tracks[0].Cues[0]
	require.Equal(t, start, segmentWall.Add(time.Duration(cue.Start-2000)*time.Millisecond))
	require.Equal(t, end, segmentWall.Add(time.Duration(cue.End-2000)*time.Millisecond))
}

func TestCaptionMasterOffSignedStartWaitsForLaggingParser(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	fixture, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)
	m := newCaptionMaster(ctx, "streamer", &config.CLI{}, nil)
	m.setManifest(captionManifest("off"))
	ms := newBareSegmentSigner(t)
	requested := make(chan struct{}, 1)
	returned := make(chan struct{}, 1)
	in := muxl.SignerInput{
		CertPEM: ms.Cert, Sign: muxl.SignerToCallback(ms.Signer, 32),
		TrackManifestFn:   func() ([]byte, error) { return captionManifest("off"), nil },
		WrapperManifestFn: func() ([]byte, error) { return captionManifest("off"), nil },
		TextFn:            m.text,
		SegmentTimeFn: func(start uint64) time.Time {
			select {
			case requested <- struct{}{}:
			default:
			}
			at := m.segmentTime(start)
			select {
			case returned <- struct{}{}:
			default:
			}
			return at
		},
	}
	events := make(chan *muxl.MuxlEvent, 16)
	done := make(chan error, 1)
	go func() {
		done <- muxl.RunMuxlSignSegment(ctx, bytes.NewReader(fixture), in, nil, nil, events)
		close(events)
	}()
	select {
	case <-requested:
	case <-ctx.Done():
		t.Fatal("signer never requested media time")
	}
	// The signer has the complete media, but the independent parser has not run.
	select {
	case <-returned:
		t.Fatal("caption-off signer used an uninitialized media clock")
	case <-time.After(20 * time.Millisecond):
	}
	wall := time.UnixMilli(1800000000123)
	parser := newIngestByteBuffer(ctx)
	_, err = parser.writeAt(fixture, wall)
	require.NoError(t, err)
	require.NoError(t, parser.Close())
	require.NoError(t, m.readMedia(parser))
	var first []byte
	for event := range events {
		if event.Type == "signed-segment" && first == nil {
			first = event.Tracks["1"]
		}
	}
	require.NoError(t, <-done)
	require.True(t, wall.Equal(signedMediaDate(t, ctx, first)), "first signed startTime must equal the supplied arrival anchor")
}

func TestCaptionMasterLongDecodeClock(t *testing.T) {
	fixture, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)
	tracks, err := fmp4.Tracks(fixture)
	require.NoError(t, err)
	// Shift both AV tracks beyond the multiply-before-divide overflow boundary.
	const seconds = uint64(1000000)
	position := 0
	for _, scale := range []uint32{tracks[0].Timescale, tracks[1].Timescale, tracks[0].Timescale, tracks[1].Timescale} {
		offset := bytes.Index(fixture[position:], []byte("tfdt")) + position
		require.GreaterOrEqual(t, offset, position)
		require.Equal(t, byte(1), fixture[offset+4])
		clock := fixture[offset+8 : offset+16]
		binary.BigEndian.PutUint64(clock, binary.BigEndian.Uint64(clock)+seconds*uint64(scale))
		position = offset + 16
	}
	m := newCaptionMaster(context.Background(), "streamer", &config.CLI{}, nil)
	m.setManifest(captionManifest("off"))
	require.NoError(t, m.readMedia(bytes.NewReader(fixture)))
	base := time.UnixMilli(int64(seconds * 1000))
	require.True(t, !m.mediaOrigin.Before(base) && m.mediaOrigin.Before(base.Add(5*time.Second)), "media clock wrapped: %s", m.mediaOrigin)
	require.GreaterOrEqual(t, m.parsedUntil, seconds*1000)
	require.Less(t, m.parsedUntil, seconds*1000+5000, "keyframe watermark must not wrap")
}

func TestCaptionMasterAudioOnlySignsBeforeEOF(t *testing.T) {
	withNoGSTLeaks(t, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		fixture := runSynthPipeline(t, ctx,
			"audiotestsrc num-buffers=235 samplesperbuffer=1024 ! audio/x-raw,rate=48000,channels=2 ! audioconvert ! opusenc ! mp4mux fragment-duration=500 ! appsink name=sink")
		input := newIngestByteBuffer(ctx)
		defer input.Close()
		_, err := input.Write(fixture)
		require.NoError(t, err)
		mm := NewOffline(&config.CLI{})
		ms := newBareSegmentSigner(t)
		ms.PrebuiltManifest = captionManifest("off")
		events := make(chan *muxl.MuxlEvent, 16)
		done := make(chan error, 1)
		go func() {
			done <- mm.SignOriginStream(ctx, ms, input, events)
			close(events)
		}()
		var clocks []uint64
		for len(clocks) < 3 {
			select {
			case event := <-events:
				require.NotNil(t, event, "audio-only signer stopped before producing three GoPs")
				if event.Type == "signed-segment" {
					clocks = append(clocks, event.FirstDecodeTimes["1"])
				}
			case <-ctx.Done():
				t.Fatal("audio-only signing stalled while the input was still open")
			}
		}
		require.Greater(t, clocks[1], clocks[0])
		require.Greater(t, clocks[2], clocks[1])
		require.NoError(t, input.Close())
		for range events {
		}
		require.NoError(t, <-done)
	})
}
