package records

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/captions/transcript"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/placestream"
)

type captionPDS struct {
	calls []comatproto.RepoPutRecord_Input
}

func (p *captionPDS) Do(_ context.Context, _, _, _ string, _ map[string]any, body, out any) error {
	in := body.(comatproto.RepoPutRecord_Input)
	p.calls = append(p.calls, in)
	if len(p.calls) == 1 {
		return errors.New("temporary PDS failure")
	}
	*out.(*comatproto.RepoPutRecord_Output) = comatproto.RepoPutRecord_Output{Uri: "at://" + in.Repo + "/place.stream.caption.transcript/" + in.Rkey, Cid: "bafy"}
	return nil
}

func TestDistributionWriterFinalFlushPDSRetry(t *testing.T) {
	pds := &captionPDS{}
	h := newHarness(t, func(cfg *Config) {
		cfg.Publisher = &RepoPublisher{Clients: func(context.Context, string) (XRPCClient, error) { return pds, nil }}
	})
	h.w.StartSessionWithOrigin(context.Background(), streamer, t0, true)
	cue := captions.Cue{ID: "gop", Text: "A continuing cue", Start: t0.Add(time.Second), End: t0.Add(2 * time.Second), Final: true}
	h.hub.PublishCanonical(streamer, autoTrack, cue)
	cue.Start = t0.Add(2 * time.Second)
	cue.End = t0.Add(3 * time.Second)
	h.hub.PublishCanonical(streamer, autoTrack, cue)
	h.w.StopSession(streamer)
	require.Len(t, pds.calls, 2)
	require.Equal(t, pds.calls[0].Rkey, pds.calls[1].Rkey, "a retry overwrites, rather than duplicating")
	rec := pds.calls[1].Record.Val.(*placestream.CaptionTranscript)
	require.Equal(t, streamer, pds.calls[1].Repo)
	require.Equal(t, int64(1000), rec.StartMs)
	words := transcript.Decode(transcript.Compact{StartMs: rec.StartMs, Text: rec.Text, Timings: rec.Timings})
	require.Equal(t, int64(3000), words[len(words)-1].EndMs)
}

func TestDistributionRelayWritesOnlyOwnSidecar(t *testing.T) {
	h := newHarness(t, nil)
	h.w.StartSessionWithOrigin(context.Background(), streamer, t0, false)
	say(h, autoTrack, "canonical", 0, "must not write streamer repo")
	own := captions.Track{ID: "sidecar-auto-en", Language: "en", Kind: captions.KindCaptions, Source: captions.SourceAuto, Origin: captions.OriginSidecar, Author: nodeDID}
	say(h, own, "own", 1000, "node speech")
	upstream := own
	upstream.ID = "upstream"
	upstream.Author = "did:web:upstream.example"
	say(h, upstream, "other", 2000, "must not copy upstream records")
	h.w.StopSession(streamer)
	calls := h.repos.snapshot()
	require.Len(t, calls, 1)
	require.Equal(t, Target{Repo: nodeDID, Node: true}, calls[0].target)
	require.Equal(t, "node speech", calls[0].rec.Text)
}

func TestDistributionWriterKeepsActiveCanonicalCueAcrossFlush(t *testing.T) {
	h := newHarness(t, nil)
	h.now = t0.Add(2 * time.Second)
	h.w.StartSessionWithOrigin(context.Background(), streamer, t0, true)
	cue := captions.Cue{ID: "continuing", Text: "Continuing speech", Start: t0.Add(time.Second), End: t0.Add(2 * time.Second), Final: true}
	h.hub.PublishCanonical(streamer, autoTrack, cue)
	h.buffered(streamer, 2)
	h.flushed()
	require.Empty(t, h.repos.snapshot(), "the newest canonical fragment can still extend")
	cue.Start = t0.Add(2 * time.Second)
	cue.End = t0.Add(3 * time.Second)
	h.hub.PublishCanonical(streamer, autoTrack, cue)
	h.w.StopSession(streamer)
	calls := h.repos.snapshot()
	require.Len(t, calls, 1)
	words := decode(calls[0].rec)
	require.Equal(t, int64(3000), words[len(words)-1].EndMs, "a flush boundary must not truncate a canonical cue")
}

func TestDistributionWriterSettlesCanonicalCueAfterSilence(t *testing.T) {
	h := newHarness(t, nil)
	h.now = t0.Add(time.Second)
	h.w.StartSessionWithOrigin(context.Background(), streamer, t0, true)
	cue := captions.Cue{ID: "last", Text: "Last speech", Start: t0, End: t0.Add(time.Second), Final: true}
	h.hub.PublishCanonical(streamer, autoTrack, cue)
	h.buffered(streamer, 2)
	h.flushed()
	require.Empty(t, h.repos.snapshot())
	h.advance(DefaultFlushInterval)
	h.flushed()
	calls := h.repos.snapshot()
	require.Len(t, calls, 1, "silence must not indefinitely defer the last canonical cue")
	require.Equal(t, "Last speech", calls[0].rec.Text)
	h.w.StopSession(streamer)
	require.Len(t, h.repos.snapshot(), 1, "session flush does not duplicate the settled cue")
}
