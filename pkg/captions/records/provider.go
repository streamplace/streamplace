package records

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/captions/transcript"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
)

// ErrTrackNotFound is returned by Provider.Cues for a track id the video does
// not have.
var ErrTrackNotFound = errors.New("captions/records: no such caption track")

// Store is the part of the index a Provider reads. model.Model satisfies it.
type Store interface {
	GetVideoByURI(ctx context.Context, uri string) (*placestream.Video, error)
	GetCaptionTranscriptsBySubject(ctx context.Context, subjectURI string) ([]*model.CaptionTranscript, error)
}

// Provider is a captions.VideoCaptions backed by indexed
// place.stream.caption.transcript records.
//
// # Which records belong to a video
//
// In decreasing order of precedence, for each (author, language, kind, source):
//
//  1. records whose subject is the video itself, which count from the start of
//     the video;
//  2. for a clip (a video whose source is another video's time range), records
//     whose subject is the clipped video, moved onto the clip's timeline and
//     cut to it;
//  3. for a video recorded from livestreams (the livestreams are in its
//     `connections`), records whose subject is one of those livestreams, which
//     count from their mediaStart wall-clock instant. A track that has records
//     at a higher level uses only those.
//
// Only the video owner's records and this node's own (the sidecar captions of
// its server repo) are shown; anyone can write a transcript record that names
// someone else's video, and a viewer should not get a stranger's captions on a
// streamer's video unannounced.
//
// # Placing live captions on the video
//
// A live-derived VOD starts at the first recorded object's first segment. When
// the node knows the recording objects (Recording) the mapping is exact within
// the objects' clock error: wall-clock time between objects (a disconnect) is
// cut out, as it is from the VOD. Without them, a video that connects exactly
// one livestream is assumed to run from the session's mediaStart, and a video
// of several livestreams has its livestream captions left out, because there
// is no way to tell where each one begins.
type Provider struct {
	Store   Store
	NodeDID string
	// Recording is optional; see above.
	Recording Recording
	// CueOptions sets how words are grouped into display cues; zero is the
	// default.
	CueOptions transcript.CueOptions
}

// subject classes, in decreasing precedence.
const (
	classVideo = iota
	classClip
	classLive
)

type group struct {
	track captions.Track
	rows  []*model.CaptionTranscript
	class int
}

// videoView is what Tracks and Cues work out about a video.
type videoView struct {
	groups []*group
	// clip is the part of a clipped video that records of the parent map to.
	clipStart, clipEnd time.Duration
	// live lists the livestream URIs the video was recorded from.
	live []string
	// spans are the recording objects of live, when known.
	spans []Span
}

func (p *Provider) Tracks(ctx context.Context, video string) ([]captions.Track, error) {
	v, err := p.view(ctx, video)
	if err != nil || v == nil {
		return nil, err
	}
	tracks := make([]captions.Track, len(v.groups))
	for i, g := range v.groups {
		tracks[i] = g.track
	}
	return tracks, nil
}

func (p *Provider) Cues(ctx context.Context, video, trackID string) ([]captions.TimedCue, error) {
	v, err := p.view(ctx, video)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, ErrTrackNotFound
	}
	for _, g := range v.groups {
		if g.track.ID != trackID {
			continue
		}
		words, err := p.words(ctx, v, g)
		if err != nil {
			return nil, err
		}
		return transcript.Cues(words, p.CueOptions), nil
	}
	return nil, ErrTrackNotFound
}

// view loads the records of a video and groups them into tracks. It returns
// nil for a video that is not indexed.
func (p *Provider) view(ctx context.Context, videoURI string) (*videoView, error) {
	uri, err := syntax.ParseATURI(videoURI)
	if err != nil {
		return nil, fmt.Errorf("invalid video uri: %w", err)
	}
	rec, err := p.Store.GetVideoByURI(ctx, videoURI)
	if err != nil {
		return nil, fmt.Errorf("get video: %w", err)
	}
	if rec == nil {
		return nil, nil
	}
	trusted := map[string]bool{uri.Authority().String(): true}
	if p.NodeDID != "" {
		trusted[p.NodeDID] = true
	}

	v := &videoView{}
	subjects := []struct {
		uri   string
		class int
	}{{videoURI, classVideo}}
	if clip := rec.Source.MediaDefs_SourceClip; clip != nil {
		if parent, err := syntax.ParseATURI(clip.Video); err == nil {
			trusted[parent.Authority().String()] = true
			v.clipStart = time.Duration(clip.Start) * time.Millisecond
			v.clipEnd = time.Duration(clip.End) * time.Millisecond
			subjects = append(subjects, struct {
				uri   string
				class int
			}{clip.Video, classClip})
		}
	}
	for _, c := range rec.Connections {
		ref := c.Video_Connection
		if ref == nil || ref.Ref == nil {
			continue
		}
		if u, err := syntax.ParseATURI(ref.Ref.Uri); err == nil && u.Collection().String() == constants.PLACE_STREAM_LIVESTREAM {
			v.live = append(v.live, ref.Ref.Uri)
			subjects = append(subjects, struct {
				uri   string
				class int
			}{ref.Ref.Uri, classLive})
		}
	}

	type key struct{ author, language, kind, source string }
	byKey := map[key]*group{}
	for _, s := range subjects {
		rows, err := p.Store.GetCaptionTranscriptsBySubject(ctx, s.uri)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if !trusted[row.RepoDID] {
				continue
			}
			k := key{row.RepoDID, row.Language, row.Kind, row.Source}
			g := byKey[k]
			// Subjects are visited in precedence order, so a track that was
			// first seen at one class ignores rows of the lower classes.
			switch {
			case g == nil:
				g = &group{track: trackOf(row), class: s.class}
				byKey[k] = g
			case s.class > g.class:
				continue
			}
			g.rows = append(g.rows, row)
		}
	}
	for _, g := range byKey {
		v.groups = append(v.groups, g)
	}
	if err := p.placeLive(ctx, v); err != nil {
		return nil, err
	}
	sort.Slice(v.groups, func(i, j int) bool { return v.groups[i].track.ID < v.groups[j].track.ID })
	return v, nil
}

// placeLive loads the recording spans the livestream tracks of a video are
// placed with, and drops those tracks when they cannot be placed: with no
// spans, only a video of exactly one livestream is known to start at that
// livestream's mediaStart.
func (p *Provider) placeLive(ctx context.Context, v *videoView) error {
	hasLive := false
	for _, g := range v.groups {
		hasLive = hasLive || g.class == classLive
	}
	if !hasLive {
		return nil
	}
	if p.Recording != nil {
		spans, err := p.Recording.Spans(ctx, v.live)
		if err != nil {
			return fmt.Errorf("recording spans: %w", err)
		}
		v.spans = spans
	}
	if len(v.spans) > 0 || len(v.live) == 1 {
		return nil
	}
	kept := v.groups[:0]
	for _, g := range v.groups {
		if g.class != classLive {
			kept = append(kept, g)
		}
	}
	v.groups = kept
	return nil
}

// TrackID is the id of the record track of an author's captions of one
// language, kind, and source: record-<source>-<language>-<8 hex of
// sha256(author, kind)>. It is derived only from what makes the track what it
// is, so it is the same on every node and for every request.
func TrackID(author, language, kind, source string) string {
	sum := sha256.Sum256([]byte(author + "\x00" + kind))
	return fmt.Sprintf("record-%s-%s-%s", source, strings.ToLower(language), hex.EncodeToString(sum[:4]))
}

func trackOf(row *model.CaptionTranscript) captions.Track {
	t := captions.Track{
		ID:       TrackID(row.RepoDID, row.Language, row.Kind, row.Source),
		Language: row.Language,
		Kind:     captions.Kind(row.Kind),
		Source:   captions.Source(row.Source),
		Origin:   captions.OriginRecord,
		Author:   row.RepoDID,
	}
	t.Label = label(t)
	if t.Source == captions.SourceAuto {
		if rec, err := row.ToRecord(); err == nil && rec.Generator != nil && rec.Generator.Model != nil {
			t.Model = *rec.Generator.Model
		}
	}
	return t
}

func label(t captions.Track) string {
	what := map[captions.Source]string{
		captions.SourceAuto:     "auto-generated",
		captions.SourceIngest:   "from the streamer",
		captions.SourceHuman:    "human",
		captions.SourceImported: "imported",
	}[t.Source]
	if t.Kind == captions.KindSubtitles {
		what = "subtitles, " + what
	}
	return t.Language + " (" + what + ")"
}

// words decodes every record of a track into words on the video's timeline,
// in time order.
func (p *Provider) words(ctx context.Context, v *videoView, g *group) ([]transcript.Word, error) {
	var tl *timeline
	var out []transcript.Word
	for _, row := range g.rows {
		rec, err := row.ToRecord()
		if err != nil {
			log.Warn(ctx, "skipping unreadable caption transcript", "uri", row.URI, "error", err)
			continue
		}
		ws := transcript.Decode(transcript.Compact{Text: rec.Text, StartMs: rec.StartMs, Timings: rec.Timings})
		switch g.class {
		case classVideo:
			out = append(out, ws...)
		case classClip:
			out = append(out, shiftWords(ws, -v.clipStart.Milliseconds(), (v.clipEnd-v.clipStart).Milliseconds())...)
		case classLive:
			if row.MediaStart == nil {
				continue
			}
			if tl == nil {
				tl = v.timelineFor(g)
			}
			out = append(out, liveWords(ws, row.MediaStart.UTC(), tl)...)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].StartMs < out[j].StartMs })
	return out, nil
}

// shiftWords moves words by delta ms and cuts them to [0, limit).
func shiftWords(ws []transcript.Word, delta, limit int64) []transcript.Word {
	var out []transcript.Word
	for _, w := range ws {
		w.StartMs += delta
		w.EndMs += delta
		if w.EndMs <= 0 || w.StartMs >= limit {
			continue
		}
		w.StartMs = max(w.StartMs, 0)
		out = append(out, w)
	}
	return out
}

// timelineFor is the mapping of a track's livestream records onto the video:
// through the recording spans when there are any, pinned so that the earliest
// mediaStart of the track is the first span's start; otherwise an empty
// timeline, which stands for a video that starts at mediaStart.
func (v *videoView) timelineFor(g *group) *timeline {
	if len(v.spans) == 0 {
		return &timeline{}
	}
	var anchor *time.Time
	for _, row := range g.rows {
		if row.MediaStart != nil && (anchor == nil || row.MediaStart.Before(*anchor)) {
			anchor = row.MediaStart
		}
	}
	if anchor == nil {
		return &timeline{}
	}
	return newTimeline(v.spans, anchor.UTC())
}

// liveWords puts the words of one livestream record, whose offsets count from
// mediaStart, on the video's timeline.
func liveWords(ws []transcript.Word, mediaStart time.Time, tl *timeline) []transcript.Word {
	out := make([]transcript.Word, 0, len(ws))
	for _, w := range ws {
		start := w.StartMs
		if len(tl.spans) > 0 {
			start = tl.offset(mediaStart.Add(time.Duration(w.StartMs) * time.Millisecond)).Milliseconds()
		}
		end := start + (w.EndMs - w.StartMs)
		if end <= 0 {
			continue
		}
		out = append(out, transcript.Word{Text: w.Text, StartMs: max(start, 0), EndMs: end})
	}
	return out
}
