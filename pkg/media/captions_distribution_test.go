package media

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	upstream "github.com/streamplace/muxl/go"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/muxl"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/stt"
)

type distributionEngine struct {
	leased, released chan struct{}
	err              error
}

func (e *distributionEngine) Lease(context.Context, stt.LeaseOptions) (stt.Lease, error) {
	select {
	case e.leased <- struct{}{}:
	default:
	}
	if e.err != nil {
		return nil, e.err
	}
	return &distributionLease{e: e}, nil
}
func (*distributionEngine) Models() []stt.ModelInfo { return nil }
func (*distributionEngine) Close() error            { return nil }

type distributionLease struct{ e *distributionEngine }

func (l *distributionLease) Model() stt.Model { return distributionModel{} }
func (l *distributionLease) Release()         { l.e.released <- struct{}{} }

type distributionModel struct{}

func (distributionModel) Info() stt.ModelInfo { return stt.ModelInfo{Name: "fake"} }
func (distributionModel) Transcribe(context.Context, []float32, stt.Options) (*stt.Result, error) {
	return &stt.Result{Language: "en"}, nil
}

func distributionFixture(t *testing.T) ([]byte, []byte) {
	t.Helper()
	data, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)
	events, err := segmentMuxlEvents(context.Background(), data)
	require.NoError(t, err)
	for _, ev := range events {
		if ev.Type == "segment" {
			seg := concatTracksByID(ev.Tracks)
			var header bytes.Buffer
			require.NoError(t, muxl.RunMuxlWrap(context.Background(), bytes.NewReader(seg), "flat", &header))
			return seg, header.Bytes()
		}
	}
	t.Fatal("fixture has no segments")
	return nil, nil
}

func TestDistributionSidecarPolicyAndUpstreamStop(t *testing.T) {
	seg, header := distributionFixture(t)
	eng, err := captions.TextEngine()
	require.NoError(t, err)
	// Add a cue as an observable barrier behind the AV-only policy decision.
	canonical, err := eng.AddTextTrack(context.Background(), seg, upstream.TextTrack{TrackID: 9, Language: "en", Label: "ingest"}, []upstream.TextCue{{ID: "canonical", Text: "Signed captions", Start: 100, End: 400}})
	require.NoError(t, err)
	var canonicalHeader bytes.Buffer
	require.NoError(t, eng.Wrap(context.Background(), bytes.NewReader(canonical), "flat", &canonicalHeader))
	for _, tc := range []struct {
		name, canonical      string
		local, allowed, want bool
	}{
		{"origin off", "off", true, true, true},
		{"origin auto", "auto", true, true, false},
		{"origin ingest", "ingest", true, true, false},
		{"relay fallback", "auto", false, true, true},
		{"streamer opt out", "off", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := &distributionEngine{leased: make(chan struct{}, 4), released: make(chan struct{}, 4)}
			mm := &MediaManager{cli: &config.CLI{Captions: true, BroadcasterHost: "node.example"}, bus: bus.NewBus(), STT: engine}
			vs := &validatedSegment{repoDID: "did:plc:alice", local: tc.local, meta: &SegmentMetadata{StartTime: aqtime.FromTime(time.Unix(1700000000, 0)), Published: true, MetadataConfiguration: &placestream.MetadataConfiguration{CaptionPolicy: &placestream.MetadataCaptionPolicy{Canonical: &tc.canonical, AllowNodeCaptions: &tc.allowed}}}}
			mm.distributeCaptions(context.Background(), vs, seg, header)
			d := mm.captionState()
			d.mu.Lock()
			s := d.streams[vs.repoDID]
			d.mu.Unlock()
			t.Cleanup(func() { mm.EndCaptionSession(vs.repoDID); <-s.done })
			if !tc.want {
				mm.distributeCaptions(context.Background(), vs, canonical, canonicalHeader.Bytes())
				require.Eventually(t, func() bool {
					return len(mm.bus.Captions.Cues(vs.repoDID, "canonical-ingest-en", vs.meta.StartTime.Time(), vs.meta.StartTime.Time().Add(time.Minute))) == 1
				}, 15*time.Second, time.Millisecond)
				select {
				case <-engine.leased:
					t.Fatal("policy leased a speech model")
				default:
				}
				return
			}
			select {
			case <-engine.leased:
			case <-time.After(15 * time.Second):
				t.Fatal("allowed sidecar did not lease")
			}
			ev := captions.Event{Streamer: vs.repoDID, Track: captions.Track{ID: "sidecar-auto-en", Language: "en", Kind: captions.KindCaptions, Origin: captions.OriginSidecar, Source: captions.SourceAuto, Author: "did:web:upstream.example"}, Cue: captions.Cue{ID: "upstream", Text: "Upstream speech", Start: vs.meta.StartTime.Time(), End: vs.meta.StartTime.Time().Add(time.Second), Final: true}}
			data, err := captions.EncodeSidecar(ev)
			require.NoError(t, err)
			decoded, ok := captions.DecodeSidecar(data)
			require.True(t, ok)
			require.True(t, mm.ReceiveSidecar(vs.repoDID, "did:web:upstream.example", decoded))
			select {
			case <-engine.released:
			case <-time.After(15 * time.Second):
				t.Fatal("upstream sidecar did not stop recognition")
			}
			got := mm.bus.Captions.Cues(vs.repoDID, ev.Track.ID, ev.Cue.Start, ev.Cue.End)
			require.Equal(t, []captions.Cue{ev.Cue}, got)
			require.Equal(t, ev.Track.Author, mm.bus.Captions.Tracks(vs.repoDID)[0].Author)
			mm.distributeCaptions(context.Background(), vs, seg, header)
			mm.EndCaptionSession(vs.repoDID)
			<-s.done
			select {
			case <-engine.leased:
				t.Fatal("recognized after upstream appeared")
			default:
			}
			require.Empty(t, mm.bus.Captions.Tracks(vs.repoDID))
		})
	}
}

func TestDistributionCanonicalStopsSidecar(t *testing.T) {
	seg, header := distributionFixture(t)
	engine := &distributionEngine{leased: make(chan struct{}, 4), released: make(chan struct{}, 4)}
	mm := &MediaManager{cli: &config.CLI{Captions: true, BroadcasterHost: "node.example"}, bus: bus.NewBus(), STT: engine}
	vs := &validatedSegment{repoDID: "did:plc:alice", meta: &SegmentMetadata{StartTime: aqtime.FromTime(time.Unix(1700000000, 0)), Published: true}}
	mm.distributeCaptions(context.Background(), vs, seg, header)
	d := mm.captionState()
	d.mu.Lock()
	s := d.streams[vs.repoDID]
	d.mu.Unlock()
	t.Cleanup(func() { mm.EndCaptionSession(vs.repoDID); <-s.done })
	select {
	case <-engine.leased:
	case <-time.After(15 * time.Second):
		t.Fatal("relay did not start recognition")
	}
	eng, err := captions.TextEngine()
	require.NoError(t, err)
	canonical, err := eng.AddTextTrack(context.Background(), seg, upstream.TextTrack{TrackID: 9, Language: "en", Label: "ingest"}, []upstream.TextCue{{ID: "canonical", Text: "Canonical wins", Start: 100, End: 400}})
	require.NoError(t, err)
	var hdr bytes.Buffer
	require.NoError(t, eng.Wrap(context.Background(), bytes.NewReader(canonical), "flat", &hdr))
	mm.distributeCaptions(context.Background(), vs, canonical, hdr.Bytes())
	select {
	case <-engine.released:
	case <-time.After(15 * time.Second):
		t.Fatal("canonical track did not stop recognition")
	}
	got := mm.bus.Captions.Cues(vs.repoDID, "canonical-ingest-en", vs.meta.StartTime.Time(), vs.meta.StartTime.Time().Add(time.Minute))
	require.Len(t, got, 1)
	require.Equal(t, "Canonical wins", got[0].Text)
	mm.distributeCaptions(context.Background(), vs, seg, header)
	mm.EndCaptionSession(vs.repoDID)
	<-s.done
	select {
	case <-engine.leased:
		t.Fatal("recognized after canonical track appeared")
	default:
	}
}

func TestDistributionMissingEngineAndBudgetLeaveCaptionsPlayable(t *testing.T) {
	seg, header := distributionFixture(t)
	eng, err := captions.TextEngine()
	require.NoError(t, err)
	canonical, err := eng.AddTextTrack(context.Background(), seg, upstream.TextTrack{TrackID: 9, Language: "en", Label: "ingest"}, []upstream.TextCue{{ID: "canonical", Text: "Still available", Start: 100, End: 400}})
	require.NoError(t, err)
	var hdr bytes.Buffer
	require.NoError(t, eng.Wrap(context.Background(), bytes.NewReader(canonical), "flat", &hdr))
	for _, engine := range []stt.Engine{nil, &distributionEngine{leased: make(chan struct{}, 4), released: make(chan struct{}, 4), err: stt.ErrOverBudget}} {
		mm := &MediaManager{cli: &config.CLI{Captions: true, BroadcasterHost: "node.example"}, bus: bus.NewBus(), STT: engine}
		vs := &validatedSegment{repoDID: "did:plc:alice", meta: &SegmentMetadata{StartTime: aqtime.FromTime(time.Unix(1700000000, 0)), Published: true}}
		mm.distributeCaptions(context.Background(), vs, seg, header)
		mm.distributeCaptions(context.Background(), vs, canonical, hdr.Bytes())
		require.Eventually(t, func() bool {
			cues := mm.bus.Captions.Cues(vs.repoDID, "canonical-ingest-en", vs.meta.StartTime.Time(), vs.meta.StartTime.Time().Add(time.Minute))
			return len(cues) == 1 && cues[0].Text == "Still available"
		}, 15*time.Second, time.Millisecond)
		for _, track := range mm.bus.Captions.Tracks(vs.repoDID) {
			require.Equal(t, captions.OriginCanonical, track.Origin, "missing capacity must not create a sidecar")
		}
		d := mm.captionState()
		d.mu.Lock()
		s := d.streams[vs.repoDID]
		d.mu.Unlock()
		mm.EndCaptionSession(vs.repoDID)
		<-s.done
	}
}

func TestDistributionCanonicalRejectsEarlySidecarReplay(t *testing.T) {
	seg, _ := distributionFixture(t)
	eng, err := captions.TextEngine()
	require.NoError(t, err)
	canonical, err := eng.AddTextTrack(context.Background(), seg, upstream.TextTrack{TrackID: 9, Language: "en", Label: "ingest"}, []upstream.TextCue{{ID: "canonical", Text: "Canonical replay precedence", Start: 100, End: 400}})
	require.NoError(t, err)
	var hdr bytes.Buffer
	require.NoError(t, eng.Wrap(context.Background(), bytes.NewReader(canonical), "flat", &hdr))
	mm := &MediaManager{cli: &config.CLI{Captions: true}, bus: bus.NewBus()}
	allowed := true
	vs := &validatedSegment{repoDID: "did:plc:alice", meta: &SegmentMetadata{StartTime: aqtime.FromTime(time.Unix(1700000000, 0)), Published: true, MetadataConfiguration: &placestream.MetadataConfiguration{CaptionPolicy: &placestream.MetadataCaptionPolicy{AllowNodeCaptions: &allowed}}}}
	ev := captions.Event{Streamer: vs.repoDID, Track: captions.Track{ID: "sidecar-auto-en", Language: "en", Kind: captions.KindCaptions, Origin: captions.OriginSidecar, Source: captions.SourceAuto, Author: "did:web:upstream.example"}, Cue: captions.Cue{ID: "upstream", Text: "Competing replay", Start: vs.meta.StartTime.Time(), End: vs.meta.StartTime.Time().Add(time.Second), Final: true}}
	require.True(t, mm.ReceiveSidecar(vs.repoDID, "did:web:upstream.example", ev))
	require.Empty(t, mm.bus.Captions.Tracks(vs.repoDID), "unvalidated replay remains private")
	mm.distributeCaptions(context.Background(), vs, canonical, hdr.Bytes())
	d := mm.captionState()
	d.mu.Lock()
	s := d.streams[vs.repoDID]
	d.mu.Unlock()
	t.Cleanup(func() { mm.EndCaptionSession(vs.repoDID); <-s.done })
	require.Eventually(t, func() bool {
		return len(mm.bus.Captions.Cues(vs.repoDID, "canonical-ingest-en", ev.Cue.Start, ev.Cue.Start.Add(time.Minute))) == 1
	}, 15*time.Second, time.Millisecond)
	require.Empty(t, mm.bus.Captions.Cues(vs.repoDID, ev.Track.ID, ev.Cue.Start, ev.Cue.End), "canonical tracks suppress an earlier upstream sidecar replay")
	require.False(t, mm.ReceiveSidecar(vs.repoDID, "did:web:upstream.example", ev), "later competing sidecars remain suppressed")
}
