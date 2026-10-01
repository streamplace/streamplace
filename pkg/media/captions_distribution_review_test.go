package media

import (
	"bytes"
	"context"
	"testing"
	"time"

	upstream "github.com/streamplace/muxl/go"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/bus"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/captions/records"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/placestream"
)

func TestDistributionSidecarCannotClaimCanonicalNamespace(t *testing.T) {
	mm := &MediaManager{cli: &config.CLI{BroadcasterHost: "node.example"}, bus: bus.NewBus()}
	d := mm.captionState()
	d.streams["alice"] = &captionStream{policy: captions.Policy{AllowNodeCaptions: true}, upstreamSignal: make(chan struct{}, 1)}
	now := time.Now()
	ev := captions.Event{Streamer: "alice", Track: captions.Track{ID: "canonical-auto-en", Origin: captions.OriginSidecar, Source: captions.SourceAuto, Language: "en", Author: mm.cli.ServerDID()}, Cue: captions.Cue{ID: "forged", Text: "Peer speech", Start: now, End: now.Add(time.Second), Final: true}}
	require.True(t, mm.ReceiveSidecar("alice", "did:web:upstream.example", ev))
	tracks := mm.bus.Captions.Tracks("alice")
	require.Equal(t, "sidecar-auto-en", tracks[0].ID)
	require.Equal(t, "did:web:upstream.example", tracks[0].Author, "the connected peer, not its claimed author, determines provenance")
}

func TestDistributionCanonicalAdmissionPrecedesSidecarFrames(t *testing.T) {
	seg, _ := distributionFixture(t)
	eng, err := captions.TextEngine()
	require.NoError(t, err)
	canonical, err := eng.AddTextTrack(context.Background(), seg, upstream.TextTrack{TrackID: 9, Language: "en", Label: "ingest"}, nil)
	require.NoError(t, err)
	var hdr bytes.Buffer
	require.NoError(t, eng.Wrap(context.Background(), bytes.NewReader(canonical), "flat", &hdr))
	mm := &MediaManager{cli: &config.CLI{Captions: true}, bus: bus.NewBus()}
	allowed := true
	now := time.Now()
	vs := &validatedSegment{repoDID: "alice", meta: &SegmentMetadata{StartTime: aqtime.FromTime(now), Published: true, MetadataConfiguration: &placestream.MetadataConfiguration{CaptionPolicy: &placestream.MetadataCaptionPolicy{AllowNodeCaptions: &allowed}}}}
	mm.distributeCaptions(context.Background(), vs, canonical, hdr.Bytes())
	d := mm.captionState()
	d.mu.Lock()
	s := d.streams["alice"]
	d.mu.Unlock()
	defer func() { mm.EndCaptionSession("alice"); <-s.done }()
	ev := captions.Event{Streamer: "alice", Track: captions.Track{ID: "sidecar-auto-en", Origin: captions.OriginSidecar, Source: captions.SourceAuto, Language: "en", Author: "did:web:upstream.example"}, Cue: captions.Cue{ID: "peer", Text: "Competing", Start: now, End: now.Add(time.Second), Final: true}}
	require.False(t, mm.ReceiveSidecar("alice", "did:web:upstream.example", ev), "the frame following canonical media must not enter the hub while extraction runs")
	require.Empty(t, mm.bus.Captions.Tracks("alice"))
}

func TestDistributionShutdownReleasesSpeechAndStopsAdmission(t *testing.T) {
	seg, header := distributionFixture(t)
	engine := &distributionEngine{leased: make(chan struct{}, 4), released: make(chan struct{}, 4)}
	mm := &MediaManager{cli: &config.CLI{Captions: true, BroadcasterHost: "node.example"}, bus: bus.NewBus(), STT: engine}
	vs := &validatedSegment{repoDID: "alice", meta: &SegmentMetadata{StartTime: aqtime.FromTime(time.Now()), Published: true}}
	mm.distributeCaptions(context.Background(), vs, seg, header)
	select {
	case <-engine.leased:
	case <-time.After(15 * time.Second):
		t.Fatal("speech did not start")
	}
	mm.ShutdownCaptions()
	select {
	case <-engine.released:
	default:
		t.Fatal("caption shutdown returned before releasing speech")
	}
	require.NoError(t, engine.Close())
	mm.distributeCaptions(context.Background(), vs, seg, header)
	ev := captions.Event{Streamer: "alice", Track: captions.Track{Origin: captions.OriginSidecar, Source: captions.SourceAuto, Language: "en"}, Cue: captions.Cue{ID: "late", Start: time.Now(), End: time.Now().Add(time.Second)}}
	require.False(t, mm.ReceiveSidecar("alice", "did:web:upstream.example", ev), "shutdown cannot admit new caption work")
	require.Empty(t, mm.bus.Captions.Tracks("alice"))
}

type shutdownCaptionPDS struct{ text string }

func (p *shutdownCaptionPDS) Publish(_ context.Context, _ records.Target, _ string, rec *placestream.CaptionTranscript) (string, error) {
	p.text = rec.Text
	return "at://alice/place.stream.caption.transcript/last", nil
}

func TestDistributionShutdownArchivesAcceptedCanonicalQueue(t *testing.T) {
	seg, _ := distributionFixture(t)
	eng, err := captions.TextEngine()
	require.NoError(t, err)
	canonical, err := eng.AddTextTrack(context.Background(), seg, upstream.TextTrack{TrackID: 9, Language: "en", Label: "ingest"}, []upstream.TextCue{{ID: "last", Text: "Last accepted speech", Start: 100, End: 400}})
	require.NoError(t, err)
	var hdr bytes.Buffer
	require.NoError(t, eng.Wrap(context.Background(), bytes.NewReader(canonical), "flat", &hdr))
	mm := &MediaManager{cli: &config.CLI{}, bus: bus.NewBus()}
	pds := &shutdownCaptionPDS{}
	writer, err := records.NewWriter(records.Config{Hub: mm.bus.Captions, Publisher: pds, Subject: func(context.Context, string) (comatproto.RepoStrongRef, error) {
		return comatproto.RepoStrongRef{Uri: "at://alice/place.stream.livestream/live", Cid: "bafy"}, nil
	}})
	require.NoError(t, err)
	mm.captionState().writer = writer
	vs := &validatedSegment{repoDID: "alice", local: true, meta: &SegmentMetadata{StartTime: aqtime.FromTime(time.Now()), Published: true}}
	mm.distributeCaptions(context.Background(), vs, canonical, hdr.Bytes())
	mm.ShutdownCaptions()
	require.Equal(t, "Last accepted speech", pds.text, "shutdown drains accepted canonical segments before the final transcript snapshot")
}
