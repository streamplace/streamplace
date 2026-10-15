package moq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	official "moq.dev/moq"
	"stream.place/streamplace/pkg/muxl"
)

// The wire convention (docs/moq.md): a broadcast is a streamer's DID, its
// "source" track carries the stream's signed source segments, and each
// transcoded rendition travels on a track of its own named after the MUXL
// track id the rendition is minted under (media.RenditionTrackID). Every
// group is one segment; its single frame is the bare canonical MUXL bytes.
const (
	SourceTrack = "source"
	// CatalogSection is the application section of the broadcast's catalog
	// that names the tracks (see Catalog).
	CatalogSection = "streamplace"

	renditionTrackPrefix = "rendition-"
)

// RenditionTrack names the track carrying the rendition minted under id.
func RenditionTrack(id uint32) string {
	return renditionTrackPrefix + strconv.FormatUint(uint64(id), 10)
}

// ParseRenditionTrack is RenditionTrack's inverse.
func ParseRenditionTrack(name string) (uint32, bool) {
	rest, ok := strings.CutPrefix(name, renditionTrackPrefix)
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseUint(rest, 10, 32)
	if err != nil || strconv.FormatUint(id, 10) != rest {
		return 0, false
	}
	return uint32(id), true
}

// Catalog names independently signed rendition tracks. SourceSequence is scoped
// to this origin; never reuse its FETCH cursor against another origin.
type Catalog struct {
	// SourceSequence is the sequence of the newest source group the
	// publisher holds; nil before the first segment.
	SourceSequence *uint64 `json:"sourceSequence,omitempty"`
	// Renditions lists the rendition tracks, in track-id order.
	Renditions []Rendition `json:"renditions"`
}

// Rendition is one rendition track in the Catalog.
type Rendition struct {
	Track string `json:"track"`
	ID    uint32 `json:"id"`
}

// JSON encodes the section for BroadcastProducer.SetCatalogSection.
func (c Catalog) JSON() (string, error) {
	if c.Renditions == nil {
		c.Renditions = []Rendition{}
	}
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ParseCatalog reads the Streamplace section out of a received catalog;
// ok is false when the broadcast carries none (not a Streamplace node).
func ParseCatalog(cat *official.Catalog) (c Catalog, ok bool, err error) {
	if cat == nil {
		return c, false, nil
	}
	raw, ok := cat.Sections[CatalogSection]
	if !ok {
		return c, false, nil
	}
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return c, true, fmt.Errorf("moq: decoding %s catalog section: %w", CatalogSection, err)
	}
	return c, true, nil
}

// IsMuxl recognises a bare canonical segment: it leads with the c2pa or
// muxl uuid box, never with ftyp/moov (a presentation MP4) or anything
// else. The transport carries nothing but these.
func IsMuxl(b []byte) bool {
	return len(b) >= 8 && string(b[4:8]) == "uuid"
}

// RenditionPiece is one rendition's signed bytes out of an addendum: every
// [uuid][uuid][moof][mdat] run minted under the track id, in order, so it
// is itself a bare canonical segment that verifies and folds into a live
// window on its own.
type RenditionPiece struct {
	ID      uint32
	Payload []byte
}

// SplitRenditions uses MUXL's canonical event parser and preserves signed bytes.
func SplitRenditions(ctx context.Context, addendum []byte) ([]RenditionPiece, error) {
	events := make(chan *muxl.MuxlEvent, 8)
	done := make(chan error, 1)
	go func() {
		err := muxl.RunMuxlUnwrapEvents(ctx, bytes.NewReader(addendum), events)
		close(events)
		done <- err
	}()
	tracks := map[uint32][]byte{}
	var parseErr error
	for ev := range events {
		if ev.Type != "segment" && ev.Type != "signed-segment" {
			continue
		}
		for name, payload := range ev.Tracks {
			id, err := strconv.ParseUint(name, 10, 32)
			if err != nil {
				parseErr = fmt.Errorf("invalid MUXL track id %q", name)
				continue
			}
			key := uint32(id)
			if previous, ok := tracks[key]; ok {
				tracks[key] = append(previous, payload...)
			} else {
				tracks[key] = payload
			}
		}
	}
	if err := <-done; err != nil {
		return nil, err
	}
	if parseErr != nil {
		return nil, parseErr
	}
	pieces := make([]RenditionPiece, 0, len(tracks))
	for id, payload := range tracks {
		pieces = append(pieces, RenditionPiece{ID: id, Payload: payload})
	}
	sort.Slice(pieces, func(i, j int) bool { return pieces[i].ID < pieces[j].ID })
	return pieces, nil
}
