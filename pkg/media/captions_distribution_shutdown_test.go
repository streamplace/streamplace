package media

import (
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
	canonical, hdr := distributionCanonicalFixture(t, seg, []upstream.TextCue{{ID: "last", Text: "Last accepted speech", Start: 100, End: 400}})
	mm := &MediaManager{cli: &config.CLI{}, bus: bus.NewBus()}
	pds := &shutdownCaptionPDS{}
	writer, err := records.NewWriter(records.Config{Hub: mm.bus.Captions, Publisher: pds, Subject: func(context.Context, string, time.Time) (comatproto.RepoStrongRef, error) {
		return comatproto.RepoStrongRef{Uri: "at://alice/place.stream.livestream/live", Cid: "bafy"}, nil
	}})
	require.NoError(t, err)
	mm.captionState().writer = writer
	vs := &validatedSegment{repoDID: "alice", local: true, meta: &SegmentMetadata{StartTime: aqtime.FromTime(time.Now()), Published: true}}
	mm.distributeCaptions(context.Background(), vs, canonical, hdr)
	mm.ShutdownCaptions()
	require.Equal(t, "Last accepted speech", pds.text, "shutdown drains accepted canonical segments before the final transcript snapshot")
}
