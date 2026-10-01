package records

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	glex "github.com/streamplace/glex/runtime"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/captions/transcript"
	"stream.place/streamplace/pkg/captions/webvtt"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/spid"
)

var (
	// ErrNotFound: the video is not one this node knows.
	ErrNotFound = errors.New("video not found")
	// ErrNotOwner: the caller is not the author of the video.
	ErrNotOwner = errors.New("only the video's author can import captions for it")
	// ErrInvalidCaptions: the request or the caption file is unusable.
	ErrInvalidCaptions = errors.New("invalid captions")
)

// maxWritesPerCommit is the most operations a PDS takes in one applyWrites.
const maxWritesPerCommit = 200

// ImportStore is the part of the index an Importer uses. model.Model satisfies
// it.
type ImportStore interface {
	GetVideoByURI(ctx context.Context, uri string) (*placestream.Video, error)
	GetCaptionTranscriptsForRepoSubject(ctx context.Context, repoDID, subjectURI string) ([]*model.CaptionTranscript, error)
	UpsertCaptionTranscript(ctx context.Context, rec placestream.CaptionTranscript, aturi syntax.ATURI) error
	DeleteCaptionTranscript(ctx context.Context, uri string) error
}

// ImportInput is a request to import a caption file.
type ImportInput struct {
	Video    string // AT URI of a place.stream.video
	Language string // BCP 47
	Kind     string // captions (the default) or subtitles
	Format   string // vtt or srt
	Body     string
}

// Importer turns an uploaded WebVTT or SRT file into transcript records in the
// uploader's repo.
type Importer struct {
	Store    ImportStore
	Chunking transcript.ChunkOptions
	Now      func() time.Time // for tests
}

// Import writes the records of a caption file for one of the caller's videos
// and returns their URIs. The caller's earlier imported or human-authored
// records for the same video and language are deleted in the same commit as
// the new ones are created, so the video never shows both, or neither. An
// import whose creates and deletes do not fit in one commit is rejected before
// anything is written.
//
// A VTT or SRT cue carries timing for the whole cue only, so each cue's words
// share its time evenly; see transcript.WordsFromCues.
func (im *Importer) Import(ctx context.Context, client XRPCClient, caller string, in ImportInput) ([]string, error) {
	uri, err := syntax.ParseATURI(in.Video)
	if err != nil || uri.Collection().String() != constants.PLACE_STREAM_VIDEO || uri.RecordKey() == "" {
		return nil, fmt.Errorf("%w: video must be the AT URI of a place.stream.video", ErrInvalidCaptions)
	}
	if _, err := syntax.ParseLanguage(in.Language); err != nil {
		return nil, fmt.Errorf("%w: language %q is not a BCP 47 tag", ErrInvalidCaptions, in.Language)
	}
	kind := in.Kind
	switch kind {
	case "":
		kind = string(captions.KindCaptions)
	case string(captions.KindCaptions), string(captions.KindSubtitles):
	default:
		return nil, fmt.Errorf("%w: kind must be captions or subtitles", ErrInvalidCaptions)
	}
	video, err := im.Store.GetVideoByURI(ctx, in.Video)
	if err != nil {
		return nil, fmt.Errorf("get video: %w", err)
	}
	if video == nil {
		return nil, ErrNotFound
	}
	if uri.Authority().String() != caller {
		return nil, ErrNotOwner
	}
	cues, err := parseCues(in.Format, in.Body)
	if err != nil {
		return nil, err
	}
	chunks := transcript.Chunk(transcript.WordsFromCues(cues), im.Chunking)
	if len(chunks) == 0 {
		return nil, fmt.Errorf("%w: the file has no caption text", ErrInvalidCaptions)
	}
	videoCID, err := spid.GetCID(video)
	if err != nil {
		return nil, fmt.Errorf("get video cid: %w", err)
	}
	subject := comatproto.RepoStrongRef{LexiconTypeID: "com.atproto.repo.strongRef", Uri: in.Video, Cid: videoCID.String()}

	now := time.Now()
	if im.Now != nil {
		now = im.Now()
	}
	created := now.UTC().Format("2006-01-02T15:04:05.000Z")
	type newRecord struct {
		rkey string
		rec  placestream.CaptionTranscript
	}
	news := make([]newRecord, len(chunks))
	for i, c := range chunks {
		news[i] = newRecord{
			rkey: spid.TIDClock.Next().String(),
			rec: placestream.CaptionTranscript{
				LexiconTypeID: constants.PLACE_STREAM_CAPTION_TRANSCRIPT,
				Subject:       subject,
				StartMs:       c.StartMs,
				Text:          c.Text,
				Timings:       c.Timings,
				Language:      in.Language,
				Kind:          &kind,
				Source:        string(captions.SourceImported),
				CreatedAt:     created,
			},
		}
	}

	olds, err := im.Store.GetCaptionTranscriptsForRepoSubject(ctx, caller, in.Video)
	if err != nil {
		return nil, err
	}
	var replaced []*model.CaptionTranscript
	for _, o := range olds {
		if strings.EqualFold(o.Language, in.Language) && (o.Source == string(captions.SourceImported) || o.Source == string(captions.SourceHuman)) {
			replaced = append(replaced, o)
		}
	}

	var writes []any
	for _, n := range news {
		writes = append(writes, applyWritesCreate{
			Type:       "com.atproto.repo.applyWrites#create",
			Collection: constants.PLACE_STREAM_CAPTION_TRANSCRIPT,
			Rkey:       n.rkey,
			Value:      &glex.LexiconTypeDecoder{Val: &n.rec},
		})
	}
	for _, o := range replaced {
		u, err := syntax.ParseATURI(o.URI)
		if err != nil {
			continue
		}
		writes = append(writes, applyWritesDelete{
			Type:       "com.atproto.repo.applyWrites#delete",
			Collection: constants.PLACE_STREAM_CAPTION_TRANSCRIPT,
			Rkey:       u.RecordKey().String(),
		})
	}
	if len(writes) > maxWritesPerCommit {
		return nil, fmt.Errorf("%w: replacement needs %d writes; maximum is %d", ErrInvalidCaptions, len(writes), maxWritesPerCommit)
	}
	body := applyWritesInput{Repo: caller, Writes: writes}
	var out map[string]any
	if err := client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.applyWrites", map[string]any{}, body, &out); err != nil {
		return nil, fmt.Errorf("applyWrites: %w", err)
	}

	// The firehose brings the commit back to the index, but a request that has
	// just written captions should be able to read them straight away.
	uris := make([]string, len(news))
	for i, n := range news {
		uris[i] = fmt.Sprintf("at://%s/%s/%s", caller, constants.PLACE_STREAM_CAPTION_TRANSCRIPT, n.rkey)
		if err := im.Store.UpsertCaptionTranscript(ctx, n.rec, syntax.ATURI(uris[i])); err != nil {
			log.Warn(ctx, "failed to index imported caption transcript", "uri", uris[i], "error", err)
		}
	}
	for _, o := range replaced {
		if err := im.Store.DeleteCaptionTranscript(ctx, o.URI); err != nil {
			log.Warn(ctx, "failed to drop replaced caption transcript from the index", "uri", o.URI, "error", err)
		}
	}
	return uris, nil
}

type applyWritesInput struct {
	Repo   string `json:"repo"`
	Writes []any  `json:"writes"`
}

type applyWritesCreate struct {
	Type       string `json:"$type"`
	Collection string `json:"collection"`
	Rkey       string `json:"rkey"`
	Value      any    `json:"value"`
}

type applyWritesDelete struct {
	Type       string `json:"$type"`
	Collection string `json:"collection"`
	Rkey       string `json:"rkey"`
}

// parseCues reads a caption file of the stated format into cues.
func parseCues(format, body string) ([]captions.TimedCue, error) {
	var parsed []webvtt.Cue
	var err error
	switch format {
	case "vtt":
		parsed, err = webvtt.ParseVTT([]byte(body))
	case "srt":
		parsed, err = webvtt.ParseSRT([]byte(body))
	default:
		return nil, fmt.Errorf("%w: format must be vtt or srt", ErrInvalidCaptions)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidCaptions, err.Error())
	}
	cues := make([]captions.TimedCue, len(parsed))
	for i, c := range parsed {
		cues[i] = captions.TimedCue{ID: c.ID, Start: c.Start, End: c.End, Text: c.Text}
	}
	return cues, nil
}
