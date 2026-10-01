package media

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/muxl"
	"stream.place/streamplace/pkg/stt"
)

type captionTestEngine struct {
	enter, release chan struct{}
	once           sync.Once
	scripted       *stt.Result
}

func (e *captionTestEngine) Lease(context.Context, stt.LeaseOptions) (stt.Lease, error) {
	return e, nil
}
func (e *captionTestEngine) Models() []stt.ModelInfo { return nil }
func (e *captionTestEngine) Close() error            { return nil }
func (e *captionTestEngine) Model() stt.Model        { return e }
func (e *captionTestEngine) Release()                {}
func (e *captionTestEngine) Info() stt.ModelInfo     { return stt.ModelInfo{Name: "fake"} }
func (e *captionTestEngine) Transcribe(ctx context.Context, _ []float32, _ stt.Options) (*stt.Result, error) {
	if e.enter != nil {
		e.once.Do(func() { close(e.enter) })
		select {
		case <-e.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if e.scripted != nil {
		return e.scripted, nil
	}
	return &stt.Result{Language: "en-US", Words: []stt.Word{{Text: "held words", Start: 250 * time.Millisecond, End: 750 * time.Millisecond, Prob: 0.99}}}, nil
}
func captionManifest(policy string) []byte {
	return []byte(`{"title":"captions test","assertions":[{"label":"c2pa.actions","data":{"actions":[{"action":"c2pa.created"}]}},{"label":"place.stream.metadata.configuration","data":{"captionPolicy":{"canonical":"` + policy + `","languages":["en-US"]}}}]}`)
}
func masterTrack(source captions.Source) captions.Track {
	return captions.Track{ID: captions.TrackID(captions.OriginCanonical, source, "en-US"), Language: "en-US", Source: source, Origin: captions.OriginCanonical, Kind: captions.KindCaptions}
}

func TestCaptionMasterHoldsUntilRecognitionCoversSpan(t *testing.T) {
	ctx := context.Background()
	engine := &captionTestEngine{enter: make(chan struct{}), release: make(chan struct{})}
	m := newCaptionMaster(ctx, "streamer", &config.CLI{CaptionsMasterDelay: time.Hour}, engine)
	m.mediaFinished = true
	r, err := captions.NewRecognizer(ctx, captions.RecognizerOptions{Streamer: m.streamer, Origin: captions.OriginCanonical, Hub: m.hub, Engine: engine, OnCoverage: m.coverage, Step: time.Millisecond, MinWindow: time.Millisecond, SilenceFlush: 100 * time.Millisecond})
	require.NoError(t, err)
	defer r.Close()
	pcm := make([]float32, stt.SampleRate*2)
	for i := range stt.SampleRate {
		pcm[i] = 0.2
	}
	r.Push(time.UnixMilli(0), pcm)
	<-engine.enter
	result := make(chan *muxl.TextAttachment, 1)
	started := make(chan struct{})
	go func() {
		close(started)
		attachment, err := m.text(ctx, muxl.TextRequest{StartMs: 0, EndMs: 1000})
		if err == nil {
			result <- attachment
		}
	}()
	<-started
	select {
	case <-result:
		t.Fatal("signed before recognition finished")
	default:
	}
	close(engine.release)
	select {
	case attachment := <-result:
		require.Len(t, attachment.Tracks, 1)
		require.Equal(t, "en-US", attachment.Tracks[0].Language)
		require.Equal(t, "auto", attachment.Tracks[0].Label)
		require.Len(t, attachment.Tracks[0].Cues, 1)
		require.Equal(t, uint64(250), attachment.Tracks[0].Cues[0].Start)
		require.GreaterOrEqual(t, attachment.Tracks[0].Cues[0].End, uint64(750))
		require.LessOrEqual(t, attachment.Tracks[0].Cues[0].End, uint64(1000))
		require.Equal(t, "held words", attachment.Tracks[0].Cues[0].Text)
	case <-time.After(5 * time.Second):
		t.Fatal("coverage did not release GoP")
	}
}

func TestCaptionMasterDeadlineClippingAndLateCarry(t *testing.T) {
	m := newCaptionMaster(context.Background(), "streamer", &config.CLI{CaptionsMasterDelay: time.Second}, &captionTestEngine{})
	m.closes[1000] = time.Now().Add(-2 * time.Second)
	first, err := m.text(context.Background(), muxl.TextRequest{StartMs: 0, EndMs: 1000})
	require.NoError(t, err)
	require.Empty(t, first.Tracks)
	track := masterTrack(captions.SourceAuto)
	m.hub.Publish(m.streamer, track, captions.Cue{ID: "late", Start: time.UnixMilli(500), End: time.UnixMilli(800), Text: "late", Final: true})
	m.hub.Publish(m.streamer, track, captions.Cue{ID: "crossing", Start: time.UnixMilli(1400), End: time.UnixMilli(2300), Text: "crossing", Final: true})
	m.closes[2000] = time.Now().Add(-2 * time.Second)
	second, err := m.text(context.Background(), muxl.TextRequest{StartMs: 1000, EndMs: 2000})
	require.NoError(t, err)
	require.Len(t, second.Tracks, 1)
	require.ElementsMatch(t, []muxl.TextCue{{Start: 1000, End: 1300, Text: "late", ID: m.sessionID + "/" + track.ID + "/late"}, {Start: 1400, End: 2000, Text: "crossing", ID: m.sessionID + "/" + track.ID + "/crossing"}}, second.Tracks[0].Cues)
	m.coverage(time.UnixMilli(3000))
	m.mediaFinished = true
	third, err := m.text(context.Background(), muxl.TextRequest{StartMs: 2000, EndMs: 3000})
	require.NoError(t, err)
	require.Equal(t, []muxl.TextCue{{Start: 2000, End: 2300, Text: "crossing", ID: m.sessionID + "/" + track.ID + "/crossing"}}, third.Tracks[0].Cues)
}

func TestCaptionMasterPolicySwitchPreservesTruthfulTracks(t *testing.T) {
	m := newCaptionMaster(context.Background(), "streamer", &config.CLI{}, nil)
	m.mediaFinished = true
	m.clock(time.UnixMilli(0))
	auto := masterTrack(captions.SourceAuto)
	m.hub.Publish(m.streamer, auto, captions.Cue{ID: "auto", Start: time.UnixMilli(100), End: time.UnixMilli(200), Text: "automatic", Final: true})
	first, err := m.text(context.Background(), muxl.TextRequest{EndMs: 1000})
	require.NoError(t, err)
	require.Equal(t, CaptionTrackIDBase, first.Tracks[0].TrackID)
	require.Equal(t, "auto", first.Tracks[0].Label)
	m.setManifest(captionManifest("ingest"))
	ingest := masterTrack(captions.SourceIngest)
	require.NoError(t, m.push(ingest, []captions.Cue{{ID: "ingest", Start: m.arrival.Add(1200 * time.Millisecond), End: m.arrival.Add(1500 * time.Millisecond), Text: "supplied", Final: true}}))
	second, err := m.text(context.Background(), muxl.TextRequest{StartMs: 1000, EndMs: 2000})
	require.NoError(t, err)
	require.Len(t, second.Tracks, 1)
	require.Equal(t, CaptionTrackIDBase+1, second.Tracks[0].TrackID)
	require.Equal(t, "ingest", second.Tracks[0].Label)
	m.setManifest(captionManifest("off"))
	off, err := m.text(context.Background(), muxl.TextRequest{StartMs: 2000, EndMs: 3000})
	require.NoError(t, err)
	require.Empty(t, off.Tracks)
}

type pushedFixtureManifester struct {
	mm   *MediaManager
	once sync.Once
	err  error
}

func (p *pushedFixtureManifester) BuildManifest(ctx context.Context, streamer string, _ int64) ([]byte, error) {
	p.once.Do(func() {
		value, _ := p.mm.captionMasters.Load(streamer)
		master := value.(*captionMaster)
		for {
			master.mu.Lock()
			ready := !master.arrival.IsZero()
			arrival := master.arrival
			changed := master.changed
			master.mu.Unlock()
			if ready {
				p.err = p.mm.PushCanonicalCaptions(streamer, masterTrack(captions.SourceIngest), []captions.Cue{{ID: "fixture", Start: arrival.Add(900 * time.Millisecond), End: arrival.Add(1100 * time.Millisecond), Text: "signed fixture", Final: true}})
				break
			}
			select {
			case <-changed:
			case <-ctx.Done():
				p.err = ctx.Err()
				return
			}
		}
	})
	return captionManifest("ingest"), p.err
}

func TestCaptionMasterOriginStreamingSignedFixture(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	fixture, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)
	mm := NewOffline(&config.CLI{CaptionsMasterDelay: time.Second})
	ms := newBareSegmentSigner(t)
	ms.PrebuiltManifest = nil
	ms.manifestBuilder = &pushedFixtureManifester{mm: mm}
	events := make(chan *muxl.MuxlEvent, 16)
	done := make(chan error, 1)
	go func() { done <- mm.SignOriginStream(ctx, ms, bytes.NewReader(fixture), events); close(events) }()
	var archived bytes.Buffer
	count := 0
	for event := range events {
		if event.Type != "signed-segment" {
			continue
		}
		count++
		segment := concatTracksSorted(event.Tracks)
		report, err := muxl.RunMuxlVerify(ctx, bytes.NewReader(segment))
		require.NoError(t, err)
		var verified struct {
			Segments []struct {
				ValidationState string `json:"validation_state"`
			} `json:"segments"`
		}
		require.NoError(t, json.Unmarshal([]byte(report), &verified))
		require.Len(t, verified.Segments, 3)
		for _, v := range verified.Segments {
			require.NotEqual(t, "Invalid", v.ValidationState)
		}
		archived.Write(segment)
	}
	require.NoError(t, <-done)
	require.Equal(t, 2, count)
	tracks, err := muxl.RunMuxlTextTracks(ctx, bytes.NewReader(archived.Bytes()))
	require.NoError(t, err)
	require.Equal(t, []muxl.TextTrack{{TrackID: CaptionTrackIDBase, Language: "en-US", Label: "ingest"}}, tracks)
	cues, err := muxl.RunMuxlReadTextCues(ctx, bytes.NewReader(archived.Bytes()), CaptionTrackIDBase)
	require.NoError(t, err)
	require.Len(t, cues, 1)
	require.Equal(t, uint64(900), cues[0].Start)
	require.Equal(t, uint64(1100), cues[0].End)
	require.Equal(t, "signed fixture", cues[0].Text)
	require.Contains(t, cues[0].ID, "/push-canonical-ingest-en-us/fixture")
}

func TestCaptionMasterBufferNeverDropsMediaOnHoldOrClose(t *testing.T) {
	queue := newIngestByteBuffer()
	for range 1000 {
		_, err := queue.Write([]byte("media"))
		require.NoError(t, err)
	}
	require.NoError(t, queue.Close())
	got, err := io.ReadAll(queue)
	require.NoError(t, err)
	require.Equal(t, bytes.Repeat([]byte("media"), 1000), got)
}

func TestCaptionMasterOriginStreamingRecognizesDecodedAudio(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	fixture, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)
	engine := &captionTestEngine{enter: make(chan struct{}), release: make(chan struct{})}
	mm := NewOffline(&config.CLI{CaptionsMasterDelay: 500 * time.Millisecond})
	mm.STT = engine
	ms := newBareSegmentSigner(t)
	ms.PrebuiltManifest = captionManifest("auto")
	events := make(chan *muxl.MuxlEvent, 16)
	done := make(chan error, 1)
	go func() { done <- mm.SignOriginStream(ctx, ms, bytes.NewReader(fixture), events); close(events) }()
	var archived bytes.Buffer
	first := true
	for event := range events {
		if event.Type != "signed-segment" {
			continue
		}
		if first {
			first = false
			close(engine.release)
		}
		archived.Write(concatTracksSorted(event.Tracks))
	}
	require.NoError(t, <-done)
	tracks, err := muxl.RunMuxlTextTracks(ctx, bytes.NewReader(archived.Bytes()))
	require.NoError(t, err)
	require.Equal(t, []muxl.TextTrack{{TrackID: CaptionTrackIDBase, Language: "en-US", Label: "auto"}}, tracks)
	cues, err := muxl.RunMuxlReadTextCues(ctx, bytes.NewReader(archived.Bytes()), CaptionTrackIDBase)
	require.NoError(t, err)
	require.Len(t, cues, 1)
	require.Equal(t, "held words", cues[0].Text)
	require.GreaterOrEqual(t, cues[0].Start, uint64(1000), "late words cannot be written into the already signed first GoP")
	require.LessOrEqual(t, cues[0].End, uint64(2000))
	report, err := muxl.RunMuxlVerify(ctx, bytes.NewReader(archived.Bytes()))
	require.NoError(t, err)
	var verified struct {
		Segments []struct {
			ValidationState string `json:"validation_state"`
		} `json:"segments"`
	}
	require.NoError(t, json.Unmarshal([]byte(report), &verified))
	for _, track := range verified.Segments {
		require.NotEqual(t, "Invalid", track.ValidationState)
	}
}

func TestCaptionMasterVoicedEOFFinalsReachLastSignedGoP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	fixture, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)
	engine := &captionTestEngine{scripted: &stt.Result{Language: "en-US", Words: []stt.Word{{Text: "final voiced words", Start: 1500 * time.Millisecond, End: 1800 * time.Millisecond, Prob: 0.99}}}}
	mm := NewOffline(&config.CLI{CaptionsMasterDelay: time.Second})
	mm.STT = engine
	ms := newBareSegmentSigner(t)
	ms.PrebuiltManifest = captionManifest("auto")
	events := make(chan *muxl.MuxlEvent, 16)
	done := make(chan error, 1)
	go func() { done <- mm.SignOriginStream(ctx, ms, bytes.NewReader(fixture), events); close(events) }()
	var last []byte
	for event := range events {
		if event.Type == "signed-segment" {
			last = concatTracksSorted(event.Tracks)
		}
	}
	require.NoError(t, <-done)
	cues, err := muxl.RunMuxlReadTextCues(ctx, bytes.NewReader(last), CaptionTrackIDBase)
	require.NoError(t, err)
	require.Len(t, cues, 1)
	require.Equal(t, "final voiced words", cues[0].Text)
	require.Equal(t, uint64(1500), cues[0].Start)
}
