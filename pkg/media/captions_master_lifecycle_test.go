package media

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/muxl"
	"stream.place/streamplace/pkg/stt"
)

type masterBudgetEngine struct{ slots chan struct{} }

func (e *masterBudgetEngine) Lease(context.Context) (stt.Lease, error) {
	select {
	case e.slots <- struct{}{}:
		return &masterBudgetLease{e}, nil
	default:
		return nil, stt.ErrOverBudget
	}
}
func (*masterBudgetEngine) Close() error { return nil }

type masterBudgetLease struct{ e *masterBudgetEngine }

func (*masterBudgetLease) Model() stt.Model { return distributionModel{} }
func (l *masterBudgetLease) Release()       { <-l.e.slots }

func TestCaptionMasterTakeoverReleasesSpeechBeforeEOF(t *testing.T) {
	fixture, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)
	var prefix bytes.Buffer
	source := bytes.NewReader(fixture)
	for {
		box, kind, err := readCaptionBox(source)
		require.NoError(t, err)
		prefix.Write(box)
		if kind == "mdat" {
			break
		}
	}
	for _, takeover := range []string{"push", "off"} {
		t.Run(takeover, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			engine := &masterBudgetEngine{slots: make(chan struct{}, 1)}
			m := newCaptionMaster(ctx, "streamer", &config.CLI{}, engine)
			m.setManifest(captionManifest("auto"))
			reader, writer := io.Pipe()
			defer reader.Close()
			defer writer.Close()
			done := make(chan error, 1)
			go func() { done <- m.readMedia(reader) }()
			_, err := writer.Write(prefix.Bytes())
			require.NoError(t, err)
			require.Eventually(t, func() bool { return len(engine.slots) == 1 }, time.Second, time.Millisecond)
			_, err = engine.Lease(ctx)
			require.ErrorIs(t, err, stt.ErrOverBudget)
			if takeover == "off" {
				m.setManifest(captionManifest("off"))
			} else {
				m.mu.Lock()
				arrival := m.arrival
				m.mu.Unlock()
				require.NoError(t, m.push(masterTrack(captions.SourceHuman), []captions.Cue{{ID: "takeover", Start: arrival, End: arrival.Add(time.Second), Text: "supplied", Final: true}}))
			}
			_, err = writer.Write(fixture[len(prefix.Bytes()):])
			require.NoError(t, err)
			require.Eventually(t, func() bool { return len(engine.slots) == 0 }, time.Second, time.Millisecond, "canonical takeover must free the node budget before EOF")
			other, err := engine.Lease(ctx)
			require.NoError(t, err)
			other.Release()
			require.NoError(t, writer.Close())
			require.NoError(t, <-done)
		})
	}
}

func TestCaptionMasterSuppliedCaptionsSkipSpeechAdmission(t *testing.T) {
	fixture, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)
	engine := &distributionEngine{leased: make(chan struct{}, 4), released: make(chan struct{}, 4)}
	m := newCaptionMaster(context.Background(), "streamer", &config.CLI{}, engine)
	m.setManifest(captionManifest("auto"))
	arrival := time.Now()
	m.clockAt(time.UnixMilli(0), arrival)
	require.NoError(t, m.push(masterTrack(captions.SourceHuman), []captions.Cue{{ID: "first", Start: arrival, End: arrival.Add(time.Second), Text: "supplied", Final: true}}))
	require.NoError(t, m.readMedia(bytes.NewReader(fixture)))
	require.Empty(t, engine.leased, "supplied canonical captions must not claim speech capacity")
}

func TestCaptionMasterResumedRecognitionArchivesAndDeliversNewSpeech(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	engine := &captionTestEngine{}
	m := newCaptionMaster(ctx, "streamer", &config.CLI{}, engine)
	m.mediaFinished = true
	m.clockAt(time.UnixMilli(0), time.Now())
	// Each admission after an off interval starts a new recognizer on the same
	// stable track and private hub, as readMedia's lifecycle does.
	pcm := make([]float32, stt.SampleRate)
	for i := range pcm {
		pcm[i] = 0.2
	}
	for i, text := range []string{"before pause", "after resume"} {
		m.setManifest(captionManifest("auto"))
		engine.scripted = &stt.Result{Language: "en-US", Words: []stt.Word{{Text: text, Start: 250 * time.Millisecond, End: 750 * time.Millisecond, Prob: 1}}}
		r, err := captions.NewRecognizer(ctx, captions.RecognizerOptions{Streamer: m.streamer, Origin: captions.OriginCanonical, Hub: m.hub, Engine: engine})
		require.NoError(t, err)
		r.Push(time.UnixMilli(int64(i)*1000), pcm)
		r.Close()
		if i == 0 {
			m.setManifest(captionManifest("off"))
		}
	}
	fixture, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)
	ms := newBareSegmentSigner(t)
	events := make(chan *muxl.MuxlEvent, 16)
	done := make(chan error, 1)
	go func() {
		done <- muxl.RunMuxlSignSegment(ctx, bytes.NewReader(fixture), muxl.SignerInput{
			CertPEM: ms.Cert, Sign: muxl.SignerToCallback(ms.Signer, 32),
			TrackManifest: captionManifest("auto"), WrapperManifest: captionManifest("auto"),
			TextFn: m.text, SegmentTimeFn: m.segmentTime,
		}, nil, nil, events)
		close(events)
	}()
	mm := &MediaManager{cli: &config.CLI{}, bus: bus.NewBus()}
	defer mm.ShutdownCaptions()
	var archived bytes.Buffer
	for event := range events {
		if event.Type != "signed-segment" {
			continue
		}
		segment := concatTracksByID(event.Tracks)
		archived.Write(segment)
		var header bytes.Buffer
		require.NoError(t, muxl.RunMuxlWrap(ctx, bytes.NewReader(segment), "flat", &header))
		vs := &validatedSegment{repoDID: m.streamer, meta: &SegmentMetadata{StartTime: aqtime.FromTime(signedMediaDate(t, ctx, event.Tracks["1"])), Published: true}}
		mm.distributeCaptions(ctx, vs, segment, header.Bytes())
	}
	require.NoError(t, <-done)
	cues, err := muxl.RunMuxlReadTextCues(ctx, bytes.NewReader(archived.Bytes()), CaptionTrackIDBase)
	require.NoError(t, err)
	require.Len(t, cues, 2, "resumed speech must survive archival despite earlier finalized cue IDs")
	require.Equal(t, "before pause", cues[0].Text)
	require.Equal(t, "after resume", cues[1].Text)
	require.Eventually(t, func() bool {
		delivered := mm.bus.Captions.Cues(m.streamer, captions.TrackID(captions.OriginCanonical, captions.SourceAuto, "en-US"), m.arrival.Add(-time.Second), m.arrival.Add(3*time.Second))
		return len(delivered) == 2 && delivered[0].Text == "before pause" && delivered[1].Text == "after resume"
	}, time.Second, time.Millisecond, "both recognizer incarnations must reach viewers on the same track")
}
