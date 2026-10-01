package vod

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	upstream "github.com/streamplace/muxl/go"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/captions/records"
	"stream.place/streamplace/pkg/placestream"
)

// VideoCaptions reads archival MUXL text first, supplementing it with imported
// and node transcript tracks. A streamer's record copy of a mastered track is
// omitted; sidecars and human corrections remain distinct choices.
type VideoCaptions struct {
	Model interface {
		GetVideoByURI(context.Context, string) (*placestream.Video, error)
		GetMediaTrackByURI(context.Context, string) (*placestream.MediaTrack, error)
	}
	Store   blob.Store
	Records captions.VideoCaptions
}

type videoCaptionView struct {
	tracks []captions.Track
	cues   map[string][]captions.TimedCue
}

func (p *VideoCaptions) Tracks(ctx context.Context, video string) ([]captions.Track, error) {
	v, err := p.view(ctx, video, 0)
	if err != nil {
		return nil, err
	}
	return v.tracks, nil
}
func (p *VideoCaptions) Cues(ctx context.Context, video, track string) ([]captions.TimedCue, error) {
	v, err := p.view(ctx, video, 0)
	if err != nil {
		return nil, err
	}
	if cues, ok := v.cues[track]; ok {
		return cues, nil
	}
	return nil, records.ErrTrackNotFound
}

func (p *VideoCaptions) view(ctx context.Context, uri string, depth int) (*videoCaptionView, error) {
	if depth > 8 {
		return nil, fmt.Errorf("caption video source cycle")
	}
	out := &videoCaptionView{cues: map[string][]captions.TimedCue{}}
	rec, err := p.Model.GetVideoByURI(ctx, uri)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return out, nil
	}
	if p.Store != nil {
		if clip := rec.Source.MediaDefs_SourceClip; clip != nil {
			parent, err := p.view(ctx, clip.Video, depth+1)
			if err != nil {
				return nil, err
			}
			for _, track := range parent.tracks {
				if track.Origin != captions.OriginCanonical {
					continue
				}
				out.tracks = append(out.tracks, track)
				out.cues[track.ID] = clipCaptionCues(parent.cues[track.ID], time.Duration(clip.Start)*time.Millisecond, time.Duration(clip.End)*time.Millisecond)
			}
		} else if src := rec.Source.MediaDefs_SourceTracks; src != nil && len(src.Tracks) > 0 {
			track, err := p.Model.GetMediaTrackByURI(ctx, src.Tracks[0].Uri)
			if err != nil {
				return nil, err
			}
			if track != nil && track.Track.MediaDefs_MuxlTrack != nil {
				if err := p.readMuxl(ctx, track.Track.MediaDefs_MuxlTrack.Blob, uri, out); err != nil {
					return nil, err
				}
			}
		}
	}
	if p.Records != nil {
		tracks, err := p.Records.Tracks(ctx, uri)
		if err != nil {
			return nil, err
		}
		for _, track := range tracks {
			cues, err := p.Records.Cues(ctx, uri, track.ID)
			if err != nil {
				return nil, err
			}
			copy := false
			for _, canonical := range out.tracks {
				if canonical.Origin == captions.OriginCanonical && canonical.Language == track.Language && canonical.Kind == track.Kind && canonical.Source == track.Source && canonical.Author == track.Author && captionText(out.cues[canonical.ID]) == captionText(cues) {
					copy = true
					break
				}
			}
			if copy {
				continue
			}
			out.tracks = append(out.tracks, track)
			out.cues[track.ID] = cues
		}
	}
	sort.Slice(out.tracks, func(i, j int) bool { return out.tracks[i].ID < out.tracks[j].ID })
	return out, nil
}

func (p *VideoCaptions) readMuxl(ctx context.Context, cid, uri string, out *videoCaptionView) error {
	meta, err := readMetafile(ctx, p.Store, cid)
	if err != nil {
		return err
	}
	var reader blob.Reader
	defer func() {
		if reader != nil {
			reader.Close()
		}
	}()
	ref := captionReference(meta)
	known := map[string]bool{}
	for _, track := range out.tracks {
		known[track.ID] = true
	}
	for tid, t := range meta.Tracks {
		if t.Type != "text" {
			continue
		}
		id, err := strconv.ParseUint(tid, 10, 32)
		if err != nil {
			return err
		}
		if reader == nil {
			reader, err = p.Store.Open(ctx, BlobsPrefix+cid+".mp4")
			if err != nil {
				return err
			}
		}
		refIndex := 0
		var elapsed uint64
		for _, seg := range t.Segments {
			lang, label := t.Language, t.Label
			if seg.CaptionConfigKnown {
				lang, label = seg.CaptionLanguage, seg.CaptionLabel
			}
			track := captions.CanonicalTrack(upstream.TextTrack{TrackID: uint32(id), Language: lang, Label: label}, videoAuthor(uri))
			if !known[track.ID] {
				out.tracks = append(out.tracks, track)
				known[track.ID] = true
			}
			data, err := io.ReadAll(io.NewSectionReader(reader, meta.FlatHeaderSize+seg.Offset, seg.Size))
			if err != nil {
				return err
			}
			offset := time.Duration(seg.CaptionOffsetNanos)
			var baseOffset time.Duration
			if seg.CaptionReferenceScale != 0 {
				baseOffset = captionTicks(seg.CaptionReferenceTicks, seg.CaptionReferenceScale)
			} else {
				// Legacy indexes did not retain the containing GoP association.
				for refIndex+1 < len(ref.Segments) && ref.Segments[refIndex+1].Offset <= seg.Offset {
					elapsed += ref.Segments[refIndex].DurationTicks
					refIndex++
				}
				if len(ref.Segments) == 0 || ref.Timescale == 0 {
					return fmt.Errorf("caption reference AV track missing")
				}
				reference := ref.Segments[refIndex]
				base := reference.FirstDecodeTicks
				if !reference.DecodeTimeKnown {
					av, err := io.ReadAll(io.NewSectionReader(reader, meta.FlatHeaderSize+reference.Offset, reference.Size))
					if err != nil {
						return err
					}
					var ok bool
					base, ok = firstTFDT(av)
					if !ok {
						return fmt.Errorf("caption reference AV clock missing")
					}
				}
				offset = captionTicks(elapsed, ref.Timescale)
				baseOffset = captionTicks(base, ref.Timescale)
			}
			cues, err := captions.ReadTextCues(ctx, bytes.NewReader(data), uint32(id))
			if err != nil {
				return err
			}
			for _, c := range cues {
				out.cues[track.ID] = append(out.cues[track.ID], captions.TimedCue{ID: c.ID, Text: c.Text, Start: offset + time.Duration(c.Start)*time.Millisecond - baseOffset, End: offset + time.Duration(c.End)*time.Millisecond - baseOffset})
			}
		}
	}
	for id, list := range out.cues {
		sort.SliceStable(list, func(i, j int) bool { return list[i].Start < list[j].Start })
		joined := list[:0]
		for _, cue := range list {
			if len(joined) > 0 {
				last := &joined[len(joined)-1]
				if last.ID == cue.ID && last.Text == cue.Text && last.End == cue.Start {
					last.End = cue.End
					continue
				}
			}
			joined = append(joined, cue)
		}
		out.cues[id] = joined
	}
	return nil
}

func videoAuthor(uri string) string {
	// All callers have an indexed AT URI; the authority is the signing owner.
	for i := 5; i < len(uri); i++ {
		if uri[i] == '/' {
			return uri[5:i]
		}
	}
	return ""
}

func clipCaptionCues(cues []captions.TimedCue, start, end time.Duration) []captions.TimedCue {
	var out []captions.TimedCue
	for _, cue := range cues {
		if cue.End <= start || cue.Start >= end {
			continue
		}
		cue.Start = max(cue.Start, start) - start
		cue.End = min(cue.End, end) - start
		out = append(out, cue)
	}
	return out
}

var _ captions.VideoCaptions = (*VideoCaptions)(nil)

// captionReference is the same reference clock used by live extraction:
// the canonical video track, or audio when the video has none.
func captionReference(meta *Metafile) MetafileTrack {
	keys := make([]string, 0, len(meta.Tracks))
	for id := range meta.Tracks {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	var audio MetafileTrack
	for _, id := range keys {
		t := meta.Tracks[id]
		if t.Type == "video" {
			return t
		}
		if t.Type == "audio" && audio.Timescale == 0 {
			audio = t
		}
	}
	return audio
}

func captionTicks(ticks uint64, scale uint32) time.Duration {
	return time.Duration(ticks/uint64(scale))*time.Second + time.Duration(ticks%uint64(scale))*time.Second/time.Duration(scale)
}

func captionText(cues []captions.TimedCue) string {
	var text strings.Builder
	for _, cue := range cues {
		for _, word := range strings.Fields(cue.Text) {
			if text.Len() > 0 {
				text.WriteByte(' ')
			}
			text.WriteString(word)
		}
	}
	return text.String()
}
