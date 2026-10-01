package records

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/captions/transcript"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
)

const (
	alice    = "did:plc:alice"
	bob      = "did:plc:bob"
	videoURI = "at://did:plc:alice/place.stream.video/vod1"
	live1URI = "at://did:plc:alice/place.stream.livestream/live1"
	live2URI = "at://did:plc:alice/place.stream.livestream/live2"
)

type fixture struct {
	t *testing.T
	m model.Model
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	m, err := model.MakeDB(":memory:")
	require.NoError(t, err)
	return &fixture{t: t, m: m}
}

func (f *fixture) video(uri string, mutate func(*placestream.Video)) {
	f.t.Helper()
	v := placestream.Video{
		LexiconTypeID: "place.stream.video",
		Title:         "a video",
		Source: placestream.Video_Source{
			MediaDefs_SourceTracks: &placestream.MediaDefs_SourceTracks{
				LexiconTypeID: "place.stream.media.defs#sourceTracks",
				Tracks:        []comatproto.RepoStrongRef{},
			},
		},
	}
	if mutate != nil {
		mutate(&v)
	}
	require.NoError(f.t, f.m.UpsertVideo(context.Background(), v, syntax.ATURI(uri)))
}

func connections(uris ...string) func(*placestream.Video) {
	return func(v *placestream.Video) {
		for _, u := range uris {
			v.Connections = append(v.Connections, placestream.Video_Connections_Elem{
				Video_Connection: &placestream.Video_Connection{
					LexiconTypeID: "place.stream.video#connection",
					Ref:           &comatproto.RepoStrongRef{LexiconTypeID: "com.atproto.repo.strongRef", Uri: u, Cid: "bafy"},
				},
			})
		}
	}
}

type chunk struct {
	repo, rkey, subject, lang, kind, source string
	mediaStart                              string
	words                                   []transcript.Word
}

func (f *fixture) transcript(c chunk) {
	f.t.Helper()
	enc := transcript.Encode(c.words)
	rec := placestream.CaptionTranscript{
		LexiconTypeID: "place.stream.caption.transcript",
		Subject:       comatproto.RepoStrongRef{LexiconTypeID: "com.atproto.repo.strongRef", Uri: c.subject, Cid: "bafy"},
		StartMs:       enc.StartMs,
		Text:          enc.Text,
		Timings:       enc.Timings,
		Language:      c.lang,
		Source:        c.source,
		CreatedAt:     "2026-09-30T12:00:00.000Z",
	}
	if c.kind != "" {
		rec.Kind = &c.kind
	}
	if c.mediaStart != "" {
		rec.MediaStart = &c.mediaStart
	}
	uri := "at://" + c.repo + "/place.stream.caption.transcript/" + c.rkey
	require.NoError(f.t, f.m.UpsertCaptionTranscript(context.Background(), rec, syntax.ATURI(uri)))
}

func (f *fixture) provider(mutate func(*Provider)) *Provider {
	p := &Provider{Store: f.m, NodeDID: nodeDID}
	if mutate != nil {
		mutate(p)
	}
	return p
}

func words(texts ...string) []transcript.Word {
	var out []transcript.Word
	var at int64
	for _, t := range texts {
		out = append(out, transcript.Word{Text: t, StartMs: at, EndMs: at + 400})
		at += 400
	}
	return out
}

func shifted(ws []transcript.Word, ms int64) []transcript.Word {
	out := make([]transcript.Word, len(ws))
	for i, w := range ws {
		out[i] = transcript.Word{Text: w.Text, StartMs: w.StartMs + ms, EndMs: w.EndMs + ms}
	}
	return out
}

func cueTimes(cues []captions.TimedCue) map[string][2]int64 {
	out := map[string][2]int64{}
	for _, c := range cues {
		out[c.Text] = [2]int64{c.Start.Milliseconds(), c.End.Milliseconds()}
	}
	return out
}

func TestProviderTracks(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.video(videoURI, nil)

	// Alice's own captions, in two chunks, make one track.
	f.transcript(chunk{repo: alice, rkey: "a1", subject: videoURI, lang: "en", source: "human", words: words("hello", "world")})
	f.transcript(chunk{repo: alice, rkey: "a2", subject: videoURI, lang: "en", source: "human", words: shifted(words("again"), 60_000)})
	// Another source, another language, and subtitles are each their own track.
	f.transcript(chunk{repo: alice, rkey: "a3", subject: videoURI, lang: "en", source: "imported", words: words("imported")})
	f.transcript(chunk{repo: alice, rkey: "a4", subject: videoURI, lang: "es", kind: "subtitles", source: "human", words: words("hola")})
	// The node's captions of her video are shown too.
	f.transcript(chunk{repo: nodeDID, rkey: "n1", subject: videoURI, lang: "en", source: "auto", words: words("robot")})
	// A stranger's records naming her video are not.
	f.transcript(chunk{repo: bob, rkey: "b1", subject: videoURI, lang: "en", source: "human", words: words("spam")})
	// Records of other videos are not.
	f.transcript(chunk{repo: alice, rkey: "x1", subject: "at://did:plc:alice/place.stream.video/other", lang: "fr", source: "human", words: words("non")})

	p := f.provider(nil)
	tracks, err := p.Tracks(ctx, videoURI)
	require.NoError(t, err)

	got := map[string]captions.Track{}
	for _, tr := range tracks {
		got[tr.ID] = tr
	}
	require.Len(t, tracks, 4)
	require.Len(t, got, 4, "ids are unique")

	human := got[TrackID(alice, "en", "captions", "human")]
	require.Equal(t, captions.Track{
		ID: human.ID, Language: "en", Kind: captions.KindCaptions, Source: captions.SourceHuman,
		Origin: captions.OriginRecord, Author: alice, Label: "en (human)",
	}, human)
	require.Regexp(t, `^record-human-en-[0-9a-f]{8}$`, human.ID)

	require.Contains(t, got, TrackID(alice, "en", "captions", "imported"))
	require.Equal(t, captions.KindSubtitles, got[TrackID(alice, "es", "subtitles", "human")].Kind)
	require.Equal(t, nodeDID, got[TrackID(nodeDID, "en", "captions", "auto")].Author)

	again, err := p.Tracks(ctx, videoURI)
	require.NoError(t, err)
	require.Equal(t, tracks, again, "ids and order are stable")
}

func TestTrackIDDependsOnWhoWroteIt(t *testing.T) {
	a := TrackID(alice, "en", "captions", "human")
	require.Equal(t, a, TrackID(alice, "en", "captions", "human"))
	require.Equal(t, a, TrackID(alice, "EN", "captions", "human"), "the language case does not matter")
	require.NotEqual(t, a, TrackID(bob, "en", "captions", "human"))
	require.NotEqual(t, a, TrackID(alice, "en", "subtitles", "human"))
	require.Contains(t, TrackID(alice, "pt-BR", "captions", "auto"), "record-auto-pt-br-")
}

func TestProviderTrackModelFromGenerator(t *testing.T) {
	f := newFixture(t)
	f.video(videoURI, nil)
	rec := placestream.CaptionTranscript{
		LexiconTypeID: "place.stream.caption.transcript",
		Subject:       comatproto.RepoStrongRef{Uri: videoURI, Cid: "bafy"},
		Text:          "hi", Timings: []int64{100}, Language: "en", Source: "auto", CreatedAt: "2026-09-30T12:00:00.000Z",
	}
	model := "whisper-small-q5_1"
	rec.Generator = &placestream.CaptionTranscript_Generator{Model: &model}
	require.NoError(t, f.m.UpsertCaptionTranscript(context.Background(), rec, syntax.ATURI("at://"+alice+"/place.stream.caption.transcript/g")))
	tracks, err := f.provider(nil).Tracks(context.Background(), videoURI)
	require.NoError(t, err)
	require.Len(t, tracks, 1)
	require.Equal(t, "whisper-small-q5_1", tracks[0].Model)
}

func TestProviderCues(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.video(videoURI, nil)
	f.transcript(chunk{repo: alice, rkey: "late", subject: videoURI, lang: "en", source: "human", words: shifted(words("Second", "bit."), 60_000)})
	f.transcript(chunk{repo: alice, rkey: "early", subject: videoURI, lang: "en", source: "human", words: shifted(words("First", "bit."), 1000)})
	p := f.provider(nil)
	id := TrackID(alice, "en", "captions", "human")

	cues, err := p.Cues(ctx, videoURI, id)
	require.NoError(t, err)
	require.Equal(t, []captions.TimedCue{
		{ID: "1", Start: 1000 * time.Millisecond, End: 2000 * time.Millisecond, Text: "First bit."},
		{ID: "2", Start: 60_000 * time.Millisecond, End: 61_000 * time.Millisecond, Text: "Second bit."},
	}, cues, "chunks are merged in time order and grouped into cues of at least a second")

	_, err = p.Cues(ctx, videoURI, "record-human-en-00000000")
	require.ErrorIs(t, err, ErrTrackNotFound)
	_, err = p.Cues(ctx, "at://did:plc:alice/place.stream.video/nope", id)
	require.ErrorIs(t, err, ErrTrackNotFound)
	tracks, err := p.Tracks(ctx, "at://did:plc:alice/place.stream.video/nope")
	require.NoError(t, err)
	require.Empty(t, tracks)
}

func TestProviderToleratesBrokenRecords(t *testing.T) {
	f := newFixture(t)
	f.video(videoURI, nil)
	// More durations than words, and no durations for a word: both decode to
	// what they can.
	rec := placestream.CaptionTranscript{
		LexiconTypeID: "place.stream.caption.transcript",
		Subject:       comatproto.RepoStrongRef{Uri: videoURI, Cid: "bafy"},
		Text:          "one two three", Timings: []int64{500, 500, -100, 500, 500, 500}, StartMs: 0,
		Language: "en", Source: "human", CreatedAt: "2026-09-30T12:00:00.000Z",
	}
	require.NoError(t, f.m.UpsertCaptionTranscript(context.Background(), rec, syntax.ATURI("at://"+alice+"/place.stream.caption.transcript/odd")))
	cues, err := f.provider(nil).Cues(context.Background(), videoURI, TrackID(alice, "en", "captions", "human"))
	require.NoError(t, err)
	require.Len(t, cues, 1)
	require.Equal(t, "one two three", cues[0].Text)
}

// The recording of a livestream VOD: two objects with ten minutes between them
// that the VOD does not have.
type fakeRecording struct{ spans []Span }

func (r fakeRecording) Spans(context.Context, []string) ([]Span, error) { return r.spans, nil }

var liveStart = time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

func at(min, sec int) time.Time {
	return liveStart.Add(time.Duration(min)*time.Minute + time.Duration(sec)*time.Second)
}

func TestProviderPlacesLiveCaptionsOnTheVideo(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.video(videoURI, connections(live1URI))
	mediaStart := "2026-09-30T12:00:00.000Z"
	f.transcript(chunk{
		repo: alice, rkey: "l1", subject: live1URI, lang: "en", source: "auto", mediaStart: mediaStart,
		words: []transcript.Word{
			{Text: "At", StartMs: 5000, EndMs: 5400}, {Text: "start.", StartMs: 5400, EndMs: 6400},
			// Nothing was being recorded at 15 minutes.
			{Text: "Unrecorded.", StartMs: 15 * 60_000, EndMs: 15*60_000 + 1000},
			// 21 minutes in, which is after the gap in the recording.
			{Text: "After", StartMs: 21 * 60_000, EndMs: 21*60_000 + 400}, {Text: "gap.", StartMs: 21*60_000 + 400, EndMs: 21*60_000 + 1400},
		},
	})
	id := TrackID(alice, "en", "captions", "auto")

	t.Run("without recording spans the video starts at mediaStart", func(t *testing.T) {
		cues, err := f.provider(nil).Cues(ctx, videoURI, id)
		require.NoError(t, err)
		require.Equal(t, map[string][2]int64{
			"At start.":   {5000, 6400},
			"Unrecorded.": {15 * 60_000, 15*60_000 + 1000},
			"After gap.":  {21 * 60_000, 21*60_000 + 1000 + 400},
		}, cueTimes(cues))
	})

	t.Run("with spans, time between recordings is cut out", func(t *testing.T) {
		rec := fakeRecording{spans: []Span{
			// This node began writing 1s after the first segment's start time.
			{Start: at(0, 1), End: at(10, 1)},
			{Start: at(20, 1), End: at(30, 1)},
		}}
		cues, err := f.provider(func(p *Provider) { p.Recording = rec }).Cues(ctx, videoURI, id)
		require.NoError(t, err)
		require.Equal(t, map[string][2]int64{
			"At start.": {5000, 6400},
			// Speech in the gap lands where the next recording begins.
			"Unrecorded.": {600_000, 601_000},
			// 12:21:00 is 60s into the second object (12:20:01 less the 1s head start),
			// which starts at 600s on the VOD.
			"After gap.": {660_000, 661_400},
		}, cueTimes(cues))
	})
}

func TestProviderWithSeveralLivestreams(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.video(videoURI, connections(live1URI, live2URI))
	// The same session's captions, on the second livestream record after a title change.
	f.transcript(chunk{repo: alice, rkey: "l1", subject: live1URI, lang: "en", source: "auto", mediaStart: "2026-09-30T12:00:00.000Z",
		words: []transcript.Word{{Text: "One.", StartMs: 1000, EndMs: 2000}}})
	f.transcript(chunk{repo: alice, rkey: "l2", subject: live2URI, lang: "en", source: "auto", mediaStart: "2026-09-30T12:00:00.000Z",
		words: []transcript.Word{{Text: "Two.", StartMs: 3_000_000, EndMs: 3_001_000}}})
	id := TrackID(alice, "en", "captions", "auto")

	t.Run("without the recording there is no telling where each livestream starts", func(t *testing.T) {
		tracks, err := f.provider(nil).Tracks(ctx, videoURI)
		require.NoError(t, err)
		require.Empty(t, tracks, "a track that cannot be placed is not offered")
	})

	t.Run("with it, one timeline covers both", func(t *testing.T) {
		rec := fakeRecording{spans: []Span{
			{Start: at(0, 0), End: at(30, 0)},
			{Start: at(50, 0), End: at(60, 0)}, // the second livestream, after 20 minutes off
		}}
		p := f.provider(func(p *Provider) { p.Recording = rec })
		cues, err := p.Cues(ctx, videoURI, id)
		require.NoError(t, err)
		// 3,000s in the session is 12:50:00, 1,800s of the first object plus 0 into the second.
		require.Equal(t, map[string][2]int64{"One.": {1000, 2000}, "Two.": {1_800_000, 1_801_000}}, cueTimes(cues))
	})
}

func TestProviderRecordsOfTheVideoBeatLiveRecords(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.video(videoURI, connections(live1URI))
	f.transcript(chunk{repo: alice, rkey: "live", subject: live1URI, lang: "en", source: "auto", mediaStart: "2026-09-30T12:00:00.000Z", words: words("from", "live")})
	f.transcript(chunk{repo: alice, rkey: "vod", subject: videoURI, lang: "en", source: "auto", words: shifted(words("from", "the", "video"), 2000)})
	// A different source on the livestream is still offered.
	f.transcript(chunk{repo: alice, rkey: "ing", subject: live1URI, lang: "en", source: "ingest", mediaStart: "2026-09-30T12:00:00.000Z", words: words("cea")})

	p := f.provider(nil)
	tracks, err := p.Tracks(ctx, videoURI)
	require.NoError(t, err)
	require.Len(t, tracks, 2)

	cues, err := p.Cues(ctx, videoURI, TrackID(alice, "en", "captions", "auto"))
	require.NoError(t, err)
	require.Len(t, cues, 1)
	require.Equal(t, "from the video", cues[0].Text)
	require.Equal(t, 2*time.Second, cues[0].Start)
}

func TestProviderClipsUseTheirSourceVideosCaptions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	const parent = "at://did:plc:alice/place.stream.video/parent"
	const clip = "at://did:plc:alice/place.stream.video/clip"
	f.video(parent, nil)
	f.video(clip, func(v *placestream.Video) {
		v.Source = placestream.Video_Source{MediaDefs_SourceClip: &placestream.MediaDefs_SourceClip{
			LexiconTypeID: "place.stream.media.defs#sourceClip", Video: parent, Start: 10_000, End: 20_000,
		}}
	})
	f.transcript(chunk{repo: alice, rkey: "p", subject: parent, lang: "en", source: "human", words: []transcript.Word{
		{Text: "before", StartMs: 2000, EndMs: 3000},
		{Text: "straddles", StartMs: 9_500, EndMs: 10_500}, // half in
		{Text: "inside.", StartMs: 12_000, EndMs: 13_000},
		{Text: "outside", StartMs: 25_000, EndMs: 26_000},
	}})
	cues, err := f.provider(nil).Cues(ctx, clip, TrackID(alice, "en", "captions", "human"))
	require.NoError(t, err)
	require.Equal(t, map[string][2]int64{
		"straddles": {0, 1000},
		"inside.":   {2000, 3000},
	}, cueTimes(cues), "shifted by the clip's start and cut to its length")

	// The parent is unaffected.
	pc, err := f.provider(nil).Cues(ctx, parent, TrackID(alice, "en", "captions", "human"))
	require.NoError(t, err)
	require.NotEmpty(t, pc)
}

func TestTimelineOffset(t *testing.T) {
	spans := []Span{{Start: at(0, 2), End: at(10, 2)}, {Start: at(20, 0), End: at(25, 0)}, {Start: at(26, 0), End: at(27, 0)}}
	tl := newTimeline(spans, at(0, 0)) // the first segment began 2s before the first object was opened
	tests := []struct {
		name string
		at   time.Time
		want time.Duration
	}{
		{"the first segment is the start of the video", at(0, 0), 0},
		{"inside the first object", at(3, 30), 3*time.Minute + 30*time.Second},
		{"before the first segment is negative", at(0, 0).Add(-3 * time.Second), -3 * time.Second},
		{"the end of the first object", at(10, 0), 10 * time.Minute},
		{"a gap lands on the next object", at(15, 0), 10 * time.Minute},
		{"inside the second object", at(22, 0), 10*time.Minute + 2*time.Minute + 2*time.Second},
		{"a second gap", at(25, 30), 10*time.Minute + 5*time.Minute},
		{"inside the third object", at(26, 30), 15*time.Minute + 32*time.Second},
		{"after the last object keeps counting", at(40, 0), 15*time.Minute + 14*time.Minute + 2*time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tl.offset(tt.at))
		})
	}
}

func TestProviderStoreErrorsSurface(t *testing.T) {
	p := &Provider{Store: failingStore{}}
	_, err := p.Tracks(context.Background(), videoURI)
	require.Error(t, err)
	_, err = p.Tracks(context.Background(), "not a uri")
	require.Error(t, err)
}

type failingStore struct{}

func (failingStore) GetVideoByURI(context.Context, string) (*placestream.Video, error) {
	return nil, errors.New("db is down")
}

func (failingStore) GetCaptionTranscriptsBySubject(context.Context, string) ([]*model.CaptionTranscript, error) {
	return nil, errors.New("db is down")
}
