package records

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/captions"
)

type applyCall struct {
	method string
	input  applyWritesInput
}

type fakePDS struct {
	mu    sync.Mutex
	calls []applyCall
	err   error
}

func (c *fakePDS) Do(ctx context.Context, method, contentType, path string, params map[string]any, body any, out any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	in, ok := body.(applyWritesInput)
	if !ok {
		return fmt.Errorf("unexpected body %T", body)
	}
	c.calls = append(c.calls, applyCall{path, in})
	return c.err
}

func (c *fakePDS) creates() (n int) {
	for _, call := range c.calls {
		for _, w := range call.input.Writes {
			if _, ok := w.(applyWritesCreate); ok {
				n++
			}
		}
	}
	return n
}

func (c *fakePDS) deletedRkeys() []string {
	var out []string
	for _, call := range c.calls {
		for _, w := range call.input.Writes {
			if d, ok := w.(applyWritesDelete); ok {
				out = append(out, d.Rkey)
			}
		}
	}
	return out
}

const sampleVTT = `WEBVTT

00:00:01.000 --> 00:00:03.000
Hello there, friend.

00:00:05.000 --> 00:00:07.500
General <i>Kenobi</i>!
`

func TestImportWritesRecordsAndReplacesEarlierOnes(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.video(videoURI, nil)

	// What the caller has already: imported and human English (replaced), and
	// the things that must survive.
	f.transcript(chunk{repo: alice, rkey: "oldimp", subject: videoURI, lang: "en", source: "imported", words: words("old", "import")})
	f.transcript(chunk{repo: alice, rkey: "oldhuman", subject: videoURI, lang: "en", source: "human", words: words("old", "human")})
	f.transcript(chunk{repo: alice, rkey: "auto", subject: videoURI, lang: "en", source: "auto", words: words("robot")})
	f.transcript(chunk{repo: alice, rkey: "spanish", subject: videoURI, lang: "es", source: "imported", words: words("hola")})
	f.transcript(chunk{repo: alice, rkey: "othervideo", subject: "at://did:plc:alice/place.stream.video/other", lang: "en", source: "imported", words: words("elsewhere")})
	f.transcript(chunk{repo: nodeDID, rkey: "node", subject: videoURI, lang: "en", source: "human", words: words("not", "hers")})

	pds := &fakePDS{}
	imp := &Importer{Store: f.m}
	uris, err := imp.Import(ctx, pds, alice, ImportInput{Video: videoURI, Language: "en", Format: "vtt", Body: sampleVTT})
	require.NoError(t, err)
	require.Len(t, uris, 1)
	require.Regexp(t, `^at://did:plc:alice/place\.stream\.caption\.transcript/[a-z0-9]+$`, uris[0])

	// One atomic commit: the new chunk and the two deletions.
	require.Len(t, pds.calls, 1)
	require.Equal(t, "com.atproto.repo.applyWrites", pds.calls[0].method)
	require.Equal(t, alice, pds.calls[0].input.Repo)
	require.Equal(t, 1, pds.creates())
	require.ElementsMatch(t, []string{"oldimp", "oldhuman"}, pds.deletedRkeys())

	rows, err := f.m.GetCaptionTranscriptsForRepoSubject(ctx, alice, videoURI)
	require.NoError(t, err)
	var kept []string
	for _, r := range rows {
		kept = append(kept, r.URI[strings.LastIndex(r.URI, "/")+1:])
	}
	require.ElementsMatch(t, []string{"auto", "spanish", uris[0][strings.LastIndex(uris[0], "/")+1:]}, kept, "the index reflects the change at once")

	created := pds.calls[0].input.Writes[0].(applyWritesCreate)
	require.Equal(t, "place.stream.caption.transcript", created.Collection)
	require.Equal(t, uris[0], "at://"+alice+"/"+created.Collection+"/"+created.Rkey)

	// The video now offers the imported track, with the file's cues.
	p := f.provider(nil)
	cues, err := p.Cues(ctx, videoURI, TrackID(alice, "en", "captions", "imported"))
	require.NoError(t, err)
	require.Len(t, cues, 2)
	require.Equal(t, "Hello there, friend.", cues[0].Text)
	require.InDelta(t, 1000, cues[0].Start.Milliseconds(), 1)
	require.InDelta(t, 3000, cues[0].End.Milliseconds(), 1)
	require.Equal(t, "General Kenobi!", cues[1].Text)
	require.InDelta(t, 5000, cues[1].Start.Milliseconds(), 1)
	require.InDelta(t, 7500, cues[1].End.Milliseconds(), 1)

	tracks, err := p.Tracks(ctx, videoURI)
	require.NoError(t, err)
	var sources []captions.Source
	for _, tr := range tracks {
		sources = append(sources, tr.Source)
	}
	require.ElementsMatch(t, []captions.Source{captions.SourceImported, captions.SourceImported, captions.SourceAuto, captions.SourceHuman}, sources,
		"imported en (new) and es, the auto track, and the node's human track")
}

func TestImportRecordFields(t *testing.T) {
	f := newFixture(t)
	f.video(videoURI, nil)
	pds := &fakePDS{}
	_, err := (&Importer{Store: f.m}).Import(context.Background(), pds, alice, ImportInput{
		Video: videoURI, Language: "pt-BR", Kind: "subtitles", Format: "srt",
		Body: "1\n00:00:01,000 --> 00:00:02,000\nOlá mundo\n\n2\n00:01:00,000 --> 00:01:02,000\nTchau\n",
	})
	require.NoError(t, err)
	rows, err := f.m.GetCaptionTranscriptsBySubject(context.Background(), videoURI)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	rec, err := rows[0].ToRecord()
	require.NoError(t, err)
	require.Equal(t, "Olá mundo Tchau", rec.Text)
	require.Equal(t, int64(1000), rec.StartMs)
	require.Equal(t, []int64{500, 500, -58000, 2000}, rec.Timings)
	require.Equal(t, "pt-BR", rec.Language)
	require.Equal(t, "subtitles", *rec.Kind)
	require.Equal(t, "imported", rec.Source)
	require.Nil(t, rec.MediaStart, "video subjects count from the start of the video")
	require.Equal(t, videoURI, rec.Subject.Uri)
	require.NotEmpty(t, rec.Subject.Cid)
	require.Nil(t, rec.Generator)
}

func TestImportRefusals(t *testing.T) {
	f := newFixture(t)
	f.video(videoURI, nil)
	tests := []struct {
		name   string
		caller string
		in     ImportInput
		want   error
	}{
		{"someone else's video", bob, ImportInput{Video: videoURI, Language: "en", Format: "vtt", Body: sampleVTT}, ErrNotOwner},
		{"a video that does not exist", alice, ImportInput{Video: "at://did:plc:alice/place.stream.video/none", Language: "en", Format: "vtt", Body: sampleVTT}, ErrNotFound},
		{"not a video uri", alice, ImportInput{Video: live1URI, Language: "en", Format: "vtt", Body: sampleVTT}, ErrInvalidCaptions},
		{"garbage uri", alice, ImportInput{Video: "hello", Language: "en", Format: "vtt", Body: sampleVTT}, ErrInvalidCaptions},
		{"bad language", alice, ImportInput{Video: videoURI, Language: "english please", Format: "vtt", Body: sampleVTT}, ErrInvalidCaptions},
		{"bad kind", alice, ImportInput{Video: videoURI, Language: "en", Kind: "lyrics", Format: "vtt", Body: sampleVTT}, ErrInvalidCaptions},
		{"bad format", alice, ImportInput{Video: videoURI, Language: "en", Format: "ass", Body: sampleVTT}, ErrInvalidCaptions},
		{"vtt without its header", alice, ImportInput{Video: videoURI, Language: "en", Format: "vtt", Body: "00:00:01.000 --> 00:00:02.000\nhi\n"}, ErrInvalidCaptions},
		{"a file with no cues", alice, ImportInput{Video: videoURI, Language: "en", Format: "srt", Body: "nothing to see here"}, ErrInvalidCaptions},
		{"cues without text", alice, ImportInput{Video: videoURI, Language: "en", Format: "vtt", Body: "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\n<i></i>\n"}, ErrInvalidCaptions},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pds := &fakePDS{}
			uris, err := (&Importer{Store: f.m}).Import(context.Background(), pds, tt.caller, tt.in)
			require.ErrorIs(t, err, tt.want)
			require.Empty(t, uris)
			require.Empty(t, pds.calls, "nothing is written")
		})
	}
}

func TestImportLeavesTheIndexAloneWhenThePDSRefuses(t *testing.T) {
	f := newFixture(t)
	f.video(videoURI, nil)
	f.transcript(chunk{repo: alice, rkey: "keep", subject: videoURI, lang: "en", source: "human", words: words("keep", "me")})
	pds := &fakePDS{err: errors.New("PDS says no")}
	_, err := (&Importer{Store: f.m}).Import(context.Background(), pds, alice, ImportInput{Video: videoURI, Language: "en", Format: "vtt", Body: sampleVTT})
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrInvalidCaptions)
	rows, err := f.m.GetCaptionTranscriptsBySubject(context.Background(), videoURI)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "at://did:plc:alice/place.stream.caption.transcript/keep", rows[0].URI)
}

func TestImportRejectsReplacementAboveAtomicWriteLimit(t *testing.T) {
	f := newFixture(t)
	f.video(videoURI, nil)
	f.transcript(chunk{repo: alice, rkey: "old", subject: videoURI, lang: "en", source: "human", words: words("old")})

	pds := &fakePDS{}
	uris, err := (&Importer{Store: f.m}).Import(context.Background(), pds, alice, ImportInput{
		Video: videoURI, Language: "en", Format: "vtt", Body: longVTT(400),
	})

	require.ErrorIs(t, err, ErrInvalidCaptions)
	require.ErrorContains(t, err, "replacement needs 201 writes; maximum is 200")
	require.Empty(t, uris)
	require.Empty(t, pds.calls, "an oversized replacement must not partially write")
	rows, getErr := f.m.GetCaptionTranscriptsForRepoSubject(context.Background(), alice, videoURI)
	require.NoError(t, getErr)
	require.Len(t, rows, 1)
	require.Equal(t, "at://did:plc:alice/place.stream.caption.transcript/old", rows[0].URI)
}

func TestImportAppliesReplacementAtAtomicWriteLimitInOneCommit(t *testing.T) {
	f := newFixture(t)
	f.video(videoURI, nil)
	f.transcript(chunk{repo: alice, rkey: "old", subject: videoURI, lang: "en", source: "human", words: words("old")})

	pds := &fakePDS{}
	uris, err := (&Importer{Store: f.m}).Import(context.Background(), pds, alice, ImportInput{
		Video: videoURI, Language: "en", Format: "vtt", Body: longVTT(398),
	})

	require.NoError(t, err)
	require.Len(t, uris, 199)
	require.Len(t, pds.calls, 1)
	require.Len(t, pds.calls[0].input.Writes, maxWritesPerCommit)
	require.Equal(t, 199, pds.creates())
	require.Equal(t, []string{"old"}, pds.deletedRkeys())
}

// With the default chunking, two of these widely spaced cues form each record.
func longVTT(cues int) string {
	var b strings.Builder
	b.WriteString("WEBVTT\n\n")
	for i := range cues {
		start := int64(i) * 330_000
		fmt.Fprintf(&b, "%s --> %s\nLine number %d.\n\n", vttTime(start), vttTime(start+2000), i)
	}
	return b.String()
}

func vttTime(ms int64) string {
	return fmt.Sprintf("%02d:%02d:%02d.%03d", ms/3_600_000, ms/60_000%60, ms/1000%60, ms%1000)
}
