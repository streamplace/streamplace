package media

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/crypto/signers"
	"stream.place/streamplace/pkg/muxl"
	"stream.place/streamplace/pkg/stt"
)

type captionTestEngine struct {
	enter, release chan struct{}
	once           sync.Once
	scripted       *stt.Result
}

func (e *captionTestEngine) Lease(context.Context) (stt.Lease, error) { return e, nil }
func (e *captionTestEngine) Close() error                             { return nil }
func (e *captionTestEngine) Model() stt.Model                         { return e }
func (e *captionTestEngine) Release()                                 {}
func (e *captionTestEngine) Info() stt.ModelInfo                      { return stt.ModelInfo{Name: "fake"} }
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

func TestCaptionMasterLiveSkipsTheHoldTheArchiveWaitsOut(t *testing.T) {
	ctx := context.Background()
	engine := &captionTestEngine{enter: make(chan struct{}), release: make(chan struct{})}
	m := newCaptionMaster(ctx, "streamer", &config.CLI{CaptionsMasterDelay: time.Hour}, engine)
	m.setManifest(captionManifest("auto"))
	m.clockAt(time.UnixMilli(0), time.Now())
	m.closeGopAt(3000, time.Now())
	signer := newBareSegmentSigner(t)
	key, err := signers.MarshalES256KPrivateKeyPEM(signer.Signer)
	require.NoError(t, err)
	archived := make(chan archiveText, 1)
	m.archiveTo(muxl.SignerInput{CertPEM: signer.Cert, KeyPEM: key, TrackManifest: signer.PrebuiltManifest}, func(text archiveText) error {
		archived <- text
		return nil
	})
	defer m.awaitArchive()
	defer m.stop()
	r, err := captions.NewRecognizer(ctx, captions.RecognizerOptions{Streamer: m.streamer, Origin: captions.OriginCanonical, Hub: m.hub, Engine: engine, OnCoverage: m.coverage, Step: time.Millisecond, MinWindow: time.Millisecond, SilenceFlush: 100 * time.Millisecond})
	require.NoError(t, err)
	defer r.Close()
	var release sync.Once
	defer release.Do(func() { close(engine.release) })
	pcm := make([]float32, stt.SampleRate*2)
	for i := range stt.SampleRate {
		pcm[i] = 0.2
	}
	r.Push(time.UnixMilli(2000), pcm)
	<-engine.enter
	live, err := m.text(ctx, muxl.TextRequest{StartMs: 2000, EndMs: 3000})
	require.NoError(t, err)
	require.Empty(t, live.Tracks, "live media is signed without waiting for recognition")
	m.segmentTime(2000)
	select {
	case <-archived:
		t.Fatal("archived before recognition covered the GoP")
	case <-time.After(100 * time.Millisecond):
	}
	release.Do(func() { close(engine.release) })
	var text archiveText
	select {
	case text = <-archived:
	case <-time.After(5 * time.Second):
		t.Fatal("coverage did not release the archive pass")
	}
	require.Equal(t, uint64(2000), text.StartMs, "keyed by the GoP's media start")
	cues, err := muxl.RunMuxlReadTextCues(ctx, bytes.NewReader(text.Runs[CaptionTrackIDBase]), CaptionTrackIDBase)
	require.NoError(t, err)
	require.Len(t, cues, 1)
	require.Equal(t, "held words", cues[0].Text)
	require.Equal(t, uint64(2250), cues[0].Start, "the archive places words in the GoP they were spoken in")
}

func TestCaptionMasterLaysOutLateCuesInOrder(t *testing.T) {
	m := newCaptionMaster(context.Background(), "streamer", &config.CLI{CaptionsMasterDelay: time.Second}, &captionTestEngine{})
	m.closes[1000] = time.Now().Add(-2 * time.Second)
	first, err := m.text(context.Background(), muxl.TextRequest{StartMs: 0, EndMs: 1000})
	require.NoError(t, err)
	require.Empty(t, first.Tracks)
	// Recognition agreed on two batches after their GoP was signed, and on a
	// third that crosses into the next GoP.
	track := masterTrack(captions.SourceAuto)
	m.hub.Publish(m.streamer, track, captions.Cue{ID: "late", Start: time.UnixMilli(500), End: time.UnixMilli(800), Text: "late", Final: true})
	m.hub.Publish(m.streamer, track, captions.Cue{ID: "later", Start: time.UnixMilli(800), End: time.UnixMilli(1200), Text: "later", Final: true})
	m.hub.Publish(m.streamer, track, captions.Cue{ID: "crossing", Start: time.UnixMilli(1400), End: time.UnixMilli(2300), Text: "crossing", Final: true})
	m.closes[2000] = time.Now().Add(-2 * time.Second)
	second, err := m.text(context.Background(), muxl.TextRequest{StartMs: 1000, EndMs: 2000})
	require.NoError(t, err)
	require.Len(t, second.Tracks, 1)
	id := func(cue string) string { return m.sessionID + "/" + track.ID + "/" + cue }
	// Late cues play one after another, each for its whole duration, rather
	// than stacking at the GoP start; each replaces the one before.
	require.ElementsMatch(t, []muxl.TextCue{
		{Start: 1000, End: 1300, Text: "late", ID: id("late")},
		{Start: 1300, End: 1700, Text: "later", ID: id("later")},
		{Start: 1700, End: 2000, Text: "crossing", ID: id("crossing")},
	}, second.Tracks[0].Cues)
	// Recognized speech stays up after its last word until replaced.
	m.coverage(time.UnixMilli(3000))
	m.mediaFinished = true
	third, err := m.text(context.Background(), muxl.TextRequest{StartMs: 2000, EndMs: 3000})
	require.NoError(t, err)
	require.Equal(t, []muxl.TextCue{{Start: 2000, End: 3000, Text: "crossing", ID: id("crossing")}}, third.Tracks[0].Cues)
}

func TestCaptionArchiveLayoutKeepsOnTimeCuesAfterALateOne(t *testing.T) {
	m := newCaptionMaster(context.Background(), "streamer", &config.CLI{}, nil)
	archive := newCaptionLayout(true)
	// The first GoP was laid out before its speech was recognized.
	m.layout(&archive, muxl.TextRequest{StartMs: 0, EndMs: 1000})
	track := masterTrack(captions.SourceHuman)
	m.hub.Publish(m.streamer, track, captions.Cue{ID: "late", Start: time.UnixMilli(500), End: time.UnixMilli(1100), Text: "late", Final: true})
	m.hub.Publish(m.streamer, track, captions.Cue{ID: "on-time", Start: time.UnixMilli(1200), End: time.UnixMilli(1800), Text: "on time", Final: true})
	got := m.layout(&archive, muxl.TextRequest{StartMs: 1000, EndMs: 2000})
	id := func(cue string) string { return m.sessionID + "/" + track.ID + "/" + cue }
	require.Equal(t, []muxl.TextCue{
		{Start: 1000, End: 1200, Text: "late", ID: id("late")},
		{Start: 1200, End: 1800, Text: "on time", ID: id("on-time")},
	}, got.Tracks[0].Cues, "a late cue cannot push the speech after it late")
}

func TestCaptionArchiveLayoutNeverErasesACue(t *testing.T) {
	m := newCaptionMaster(context.Background(), "streamer", &config.CLI{}, nil)
	archive := newCaptionLayout(true)
	track := masterTrack(captions.SourceHuman)
	// Whisper sometimes times a cue to start with the one before it.
	m.hub.Publish(m.streamer, track, captions.Cue{ID: "a", Start: time.UnixMilli(200), End: time.UnixMilli(500), Text: "a", Final: true})
	m.hub.Publish(m.streamer, track, captions.Cue{ID: "b", Start: time.UnixMilli(200), End: time.UnixMilli(400), Text: "b", Final: true})
	got := m.layout(&archive, muxl.TextRequest{StartMs: 0, EndMs: 1000})
	var texts []string
	for _, cue := range got.Tracks[0].Cues {
		require.Greater(t, cue.End, cue.Start)
		texts = append(texts, cue.Text)
	}
	require.ElementsMatch(t, []string{"a", "b"}, texts)
}

func TestCaptionMasterPolicySwitchPreservesTruthfulTracks(t *testing.T) {
	m := newCaptionMaster(context.Background(), "streamer", &config.CLI{}, nil)
	m.mediaFinished = true
	m.clockAt(time.UnixMilli(0), time.Now())
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
	require.Len(t, second.Tracks, 2)
	require.Empty(t, second.Tracks[0].Cues, "the replaced automatic track continues, empty")
	require.Equal(t, CaptionTrackIDBase+1, second.Tracks[1].TrackID)
	require.Equal(t, "ingest", second.Tracks[1].Label)
	m.setManifest(captionManifest("off"))
	off, err := m.text(context.Background(), muxl.TextRequest{StartMs: 2000, EndMs: 3000})
	require.NoError(t, err)
	require.Len(t, off.Tracks, 2)
	for _, track := range off.Tracks {
		require.Empty(t, track.Cues)
	}
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
		segment := concatTracksByID(event.Tracks)
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
	queue := newIngestByteBuffer(context.Background())
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
	mm := NewOffline(&config.CLI{CaptionsMasterDelay: 5 * time.Second})
	mm.STT = engine
	ms := newBareSegmentSigner(t)
	ms.PrebuiltManifest = captionManifest("auto")
	archive := newCaptionArchive(mm.cli.CaptionsMasterDelay)
	events := make(chan *muxl.MuxlEvent, 16)
	done := make(chan error, 1)
	go func() {
		done <- mm.SignOriginStream(withCaptionArchive(ctx, archive), ms, bytes.NewReader(fixture), events)
		close(events)
	}()
	var live [][]byte
	first := true
	for event := range events {
		if event.Type != "signed-segment" {
			continue
		}
		if first {
			first = false
			close(engine.release)
		}
		live = append(live, concatTracksByID(event.Tracks))
	}
	require.NoError(t, <-done)
	stream := bytes.Join(live, nil)
	tracks, err := muxl.RunMuxlTextTracks(ctx, bytes.NewReader(stream))
	require.NoError(t, err)
	require.Equal(t, []muxl.TextTrack{{TrackID: CaptionTrackIDBase, Language: "en-US", Label: "auto"}}, tracks)
	cues, err := muxl.RunMuxlReadTextCues(ctx, bytes.NewReader(stream), CaptionTrackIDBase)
	require.NoError(t, err)
	require.Len(t, cues, 1)
	require.Equal(t, "held words", cues[0].Text)
	require.GreaterOrEqual(t, cues[0].Start, uint64(1000), "late words cannot be written into the already signed first GoP")
	require.GreaterOrEqual(t, cues[0].End-cues[0].Start, uint64(500), "a late cue keeps its whole duration")
	requireValidMuxl(t, ctx, stream)

	recorded := recordSegments(t, ctx, archive, live)
	cues, err = muxl.RunMuxlReadTextCues(ctx, bytes.NewReader(recorded), CaptionTrackIDBase)
	require.NoError(t, err)
	require.Len(t, cues, 1)
	require.Equal(t, "held words", cues[0].Text)
	require.Equal(t, uint64(250), cues[0].Start, "the recording places words in the GoP they were spoken in")
	requireValidMuxl(t, ctx, recorded)
}

func TestCaptionMasterRecordingKeepsVoicedEOFFinals(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	fixture, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)
	engine := &captionTestEngine{scripted: &stt.Result{Language: "en-US", Words: []stt.Word{{Text: "final voiced words", Start: 1500 * time.Millisecond, End: 1800 * time.Millisecond, Prob: 0.99}}}}
	mm := NewOffline(&config.CLI{CaptionsMasterDelay: 5 * time.Second})
	mm.STT = engine
	ms := newBareSegmentSigner(t)
	ms.PrebuiltManifest = captionManifest("auto")
	archive := newCaptionArchive(mm.cli.CaptionsMasterDelay)
	events := make(chan *muxl.MuxlEvent, 16)
	done := make(chan error, 1)
	go func() {
		done <- mm.SignOriginStream(withCaptionArchive(ctx, archive), ms, bytes.NewReader(fixture), events)
		close(events)
	}()
	var live [][]byte
	for event := range events {
		if event.Type == "signed-segment" {
			live = append(live, concatTracksByID(event.Tracks))
		}
	}
	require.NoError(t, <-done)
	recorded := recordSegments(t, ctx, archive, live)
	cues, err := muxl.RunMuxlReadTextCues(ctx, bytes.NewReader(recorded), CaptionTrackIDBase)
	require.NoError(t, err)
	require.Len(t, cues, 1)
	require.Equal(t, "final voiced words", cues[0].Text)
	require.Equal(t, uint64(1500), cues[0].Start)
}

// recordSegments records live segments the way the director does: each one's
// archive copy, once the session's archive pass has reported its GoP.
func recordSegments(t *testing.T, ctx context.Context, archive *captionArchive, live [][]byte) []byte {
	t.Helper()
	var recorded bytes.Buffer
	for _, seg := range live {
		copied := archive.copy(ctx, seg)
		liveTracks, copiedTracks := segmentTracks(t, ctx, seg), segmentTracks(t, ctx, copied)
		for id, run := range liveTracks {
			if n, err := strconv.Atoi(id); err == nil && uint32(n) < CaptionTrackIDBase {
				require.Equal(t, run, copiedTracks[id], "track %s is recorded byte for byte", id)
			}
		}
		require.Equal(t, concatTracksByID(copiedTracks), copied, "recorded runs keep ascending track order")
		recorded.Write(copied)
	}
	return recorded.Bytes()
}

func segmentTracks(t *testing.T, ctx context.Context, seg []byte) map[string][]byte {
	t.Helper()
	events, err := unwrapMuxlEvents(ctx, seg)
	require.NoError(t, err)
	_, tracks := catalogAndTracks(events)
	return tracks
}

func requireValidMuxl(t *testing.T, ctx context.Context, data []byte) {
	t.Helper()
	report, err := muxl.RunMuxlVerify(ctx, bytes.NewReader(data))
	require.NoError(t, err)
	var verified struct {
		Segments []struct {
			ValidationState string `json:"validation_state"`
		} `json:"segments"`
	}
	require.NoError(t, json.Unmarshal([]byte(report), &verified))
	require.NotEmpty(t, verified.Segments)
	for _, track := range verified.Segments {
		require.NotEqual(t, "Invalid", track.ValidationState)
	}
}
