package vod

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/statedb"
)

// Station catalog entries live alongside (but not under the public prefix of)
// the media. The first approved source is immutable; a separate tombstone wins
// over every publication or backfill, including a concurrent one. Neither an
// OAuth session nor a station repo round trip is needed to read these facts.
type stationOrigin struct {
	CID        string `json:"cid"`
	URI        string `json:"uri"`
	Size       int64  `json:"size"`
	Withdrawn  bool   `json:"-"`
	terminal   bool
	generation int
	sources    []string
}

func stationOriginPrefix(did string) string {
	return fmt.Sprintf("station/origins/v1/%x/", sha256.Sum256([]byte(did)))
}

func stationMarkerExists(ctx context.Context, store blob.Store, key string) (bool, error) {
	_, err := existingBlobSize(ctx, store, key)
	if errors.Is(err, blob.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func writeStationMarker(ctx context.Context, store blob.Store, key string, data []byte) error {
	return blob.CreateOnce(ctx, store, key, data)
}

func validStationCID(cid string) bool {
	if cid == "" {
		return false
	}
	_, err := bdasl.Parse(cid)
	return err == nil
}

func stationOriginState(ctx context.Context, store blob.Store, did, cid string) (stationOrigin, bool, error) {
	if !validStationCID(cid) {
		return stationOrigin{}, false, nil
	}
	prefix := stationOriginPrefix(did)
	withdrawn, err := stationMarkerExists(ctx, store, prefix+"withdrawn/"+cid)
	if withdrawn || err != nil {
		return stationOrigin{CID: cid, Withdrawn: withdrawn, terminal: withdrawn}, withdrawn, err
	}
	r, err := store.Open(ctx, prefix+"public/"+cid)
	if errors.Is(err, blob.ErrNotFound) {
		return stationOrigin{}, false, nil
	}
	if err != nil {
		return stationOrigin{}, false, err
	}
	defer r.Close()
	var entry stationOrigin
	if r.Size() > 16384 {
		return entry, false, fmt.Errorf("oversized station origin entry")
	}
	if err := json.NewDecoder(io.NewSectionReader(r, 0, r.Size())).Decode(&entry); err != nil {
		return entry, false, err
	}
	if entry.CID != cid || entry.Size <= 0 {
		return entry, false, fmt.Errorf("invalid station origin entry")
	}
	if _, err := parseVideoURI(entry.URI); err != nil {
		return entry, false, err
	}
	// An approval and its retirement are independent immutable facts. One
	// source cannot revoke another concurrent writer's public video.
	sourcePrefix := prefix + "sources/" + cid + "/"
	keys, err := store.List(ctx, sourcePrefix)
	if err != nil {
		return entry, false, err
	}
	entry.generation = len(keys)
	retired := make(map[string]bool)
	for _, key := range keys {
		if hash, ok := strings.CutPrefix(key, sourcePrefix+"retired/"); ok {
			retired[hash] = true
		}
	}
	first := fmt.Sprintf("%x", sha256.Sum256([]byte(entry.URI)))
	for _, key := range keys {
		hash, ok := strings.CutPrefix(key, sourcePrefix+"approved/")
		if !ok || retired[hash] {
			continue
		}
		uri := entry.URI
		if hash != first {
			data, err := readWholeBlob(ctx, store, key)
			if err != nil {
				return entry, false, err
			}
			uri = string(data)
		}
		entry.sources = append(entry.sources, uri)
	}
	entry.Withdrawn = len(entry.sources) == 0
	return entry, true, nil
}

var ErrStationOriginWithdrawn = errors.New("station origin is withdrawn")

// AdvertiseStationVideo is called only after a public, eligible creator record
// has been established, never at processing completion. A metafile is the VOD
// pipeline's completion marker. Require it, its primary bytes, and its init
// segments before acknowledging a serving promise.
func AdvertiseStationVideo(ctx context.Context, store blob.Store, did, cid, videoURI string) error {
	if !validStationCID(cid) {
		return fmt.Errorf("invalid station origin CID")
	}
	if _, err := parseVideoURI(videoURI); err != nil {
		return err
	}
	prefix := stationOriginPrefix(did)
	source := fmt.Sprintf("%x", sha256.Sum256([]byte(videoURI)))
	retracted, err := stationMarkerExists(ctx, store, prefix+"deleted-video/"+source)
	if err != nil {
		return err
	}
	if retracted {
		return ErrStationOriginWithdrawn
	}
	entry, known, err := stationOriginState(ctx, store, did, cid)
	if err != nil {
		return err
	}
	if entry.terminal {
		return ErrStationOriginWithdrawn
	}
	sourcePrefix := prefix + "sources/" + cid + "/"
	retired, err := stationMarkerExists(ctx, store, sourcePrefix+"retired/"+source)
	if err != nil {
		return err
	}
	if retired {
		return ErrStationOriginWithdrawn
	}
	for _, u := range entry.sources {
		if u == videoURI {
			return nil
		}
	}
	size := entry.Size
	if !known {
		size, err = checkStationMedia(ctx, store, prefix, cid)
		if err != nil {
			return err
		}
	}
	if err := writeStationMarker(ctx, store, prefix+"by-video/"+source+"/"+cid, nil); err != nil {
		return err
	}
	if err := writeStationMarker(ctx, store, sourcePrefix+"approved/"+source, []byte(videoURI)); err != nil {
		return err
	}
	data, err := json.Marshal(stationOrigin{CID: cid, URI: videoURI, Size: size})
	if err != nil {
		return err
	}
	if err := writeStationMarker(ctx, store, prefix+"public/"+cid, data); err != nil {
		return err
	}
	retracted, err = stationMarkerExists(ctx, store, prefix+"deleted-video/"+source)
	if err != nil || !retracted {
		return err
	}
	if err := retractStationVideo(ctx, store, did, videoURI, ""); err != nil {
		return err
	}
	return ErrStationOriginWithdrawn
}

func checkStationMedia(ctx context.Context, store blob.Store, prefix, cid string) (int64, error) {
	meta, err := readMetafile(ctx, store, cid)
	if err != nil {
		return 0, err
	}
	size, err := existingBlobSize(ctx, store, BlobsPrefix+cid+".mp4")
	if err != nil {
		return 0, err
	}
	// The flat-MP4 metafile counts the canonical fragments, not the prepended
	// presentation header. Older MUXL metafiles have a zero flat header.
	if size <= 0 || meta.FlatHeaderSize < 0 || meta.FlatHeaderSize >= size || meta.BlobCID != cid || meta.BlobSize != size-meta.FlatHeaderSize || len(meta.Tracks) == 0 {
		return 0, fmt.Errorf("incomplete playback metadata for %s", cid)
	}
	for _, tr := range meta.Tracks {
		if !validStationCID(tr.InitCID) || tr.InitCID == cid {
			return 0, fmt.Errorf("invalid init segment for %s", cid)
		}
		if n, err := existingBlobSize(ctx, store, BlobsPrefix+tr.InitCID+".mp4"); err != nil {
			return 0, err
		} else if n == 0 {
			return 0, fmt.Errorf("empty init segment for %s", cid)
		}
		// Init segments carry codec initialization only, not media samples.
		if err := writeStationMarker(ctx, store, prefix+"init/"+tr.InitCID, nil); err != nil {
			return 0, err
		}
	}
	return size, nil
}

func advertisePublishedVideo(ctx context.Context, state *statedb.StatefulDB, store blob.Store, cid, uri string) error {
	if state.CLI.StationDID != "" {
		blocked, err := stationOriginModerated(state.StationOriginLabels, uri)
		if err != nil {
			return err
		}
		if blocked {
			return fmt.Errorf("station policy forbids advertising this video")
		}
		return AdvertiseStationVideo(ctx, store, state.CLI.StationDID, cid, uri)
	}
	size, err := existingBlobSize(ctx, store, BlobsPrefix+cid+".mp4")
	if err != nil {
		return err
	}
	if err := PublishNodeOrigin(ctx, state.CLI, cid, size); err != nil {
		return err
	}
	return state.IndexOwnMediaOrigin(ctx, cid, size, transferMimeType)
}

// WithdrawStationOrigin persists the local serving decision before any attempt
// to delete the public PDS record. A migration must never reverse this action.
func WithdrawStationOrigin(ctx context.Context, store blob.Store, did, cid string) error {
	if !validStationCID(cid) {
		return fmt.Errorf("invalid station origin CID")
	}
	return writeStationMarker(ctx, store, stationOriginPrefix(did)+"withdrawn/"+cid, nil)
}

// A video's retraction cannot withdraw a CID whose approved public source
// belongs to another record, even if it refers to the same bytes.
func retractStationVideo(ctx context.Context, store blob.Store, did, uri, keepCID string) error {
	prefix := stationOriginPrefix(did)
	source := fmt.Sprintf("%x", sha256.Sum256([]byte(uri)))
	if keepCID == "" {
		if err := writeStationMarker(ctx, store, prefix+"deleted-video/"+source, nil); err != nil {
			return err
		}
	}
	sourcePrefix := prefix + "by-video/" + source + "/"
	keys, err := store.List(ctx, sourcePrefix)
	if err != nil {
		return err
	}
	for _, key := range keys {
		cid := strings.TrimPrefix(key, sourcePrefix)
		if cid == keepCID {
			continue
		}
		if err := writeStationMarker(ctx, store, prefix+"sources/"+cid+"/retired/"+source, nil); err != nil {
			return err
		}
	}
	return nil
}

// UpdateStationVideoSource does not grant hosting based on an external edit.
// A clip/empty source retires every old CID without deleting the record.
func UpdateStationVideoSource(ctx context.Context, store blob.Store, did, uri, currentCID string) error {
	if currentCID == "" {
		currentCID = "-" // no blob can have this record key
	}
	return retractStationVideo(ctx, store, did, uri, currentCID)
}

// RetractStationRecord consumes creator and station PDS deletes. Unrelated
// creators generate no station state; a clip never withdraws its parent.
func RetractStationRecord(ctx context.Context, store blob.Store, did, rawURI string) error {
	uri, err := syntax.ParseATURI(rawURI)
	if err != nil {
		return err
	}
	switch uri.Collection().String() {
	case constants.PLACE_STREAM_VIDEO:
		prefix := stationOriginPrefix(did) + fmt.Sprintf("by-video/%x/", sha256.Sum256([]byte(rawURI)))
		keys, err := store.List(ctx, prefix)
		if err != nil || len(keys) == 0 {
			return err
		}
		return retractStationVideo(ctx, store, did, rawURI, "")
	case constants.PLACE_STREAM_MEDIA_ORIGIN:
		cid := uri.RecordKey().String()
		if uri.Authority().String() != did || !validStationCID(cid) {
			return nil
		}
		entry, known, err := stationOriginState(ctx, store, did, cid)
		if err != nil || (known && entry.Withdrawn) {
			return err
		}
		return WithdrawStationOrigin(ctx, store, did, cid)
	}
	return nil
}

// StationVideoAllowed consults the replicated AppView's moderation decisions
// on the author and the record itself. Errors are not treated as permission.
func StationVideoAllowed(mod model.Model, uri string) (bool, error) {
	if uri == "" {
		return false, nil
	}
	blocked, err := stationOriginModerated(mod.GetActiveLabels, uri)
	return err == nil && !blocked, err
}

// StationVideoVisible checks the approval for this exact creator record, not
// just some other video using the same bytes. Clips inherit their parent's
// hosting approval but still have to pass their own moderation checks.
func StationVideoVisible(ctx context.Context, store blob.Store, mod model.Model, did, uri, cid string) (bool, error) {
	if store == nil || !validStationCID(cid) {
		return false, nil
	}
	entry, known, err := stationOriginState(ctx, store, did, cid)
	if err != nil || !known || entry.Withdrawn {
		return false, err
	}
	for _, source := range entry.sources {
		if source == uri {
			return stationSourcePlayable(ctx, mod, source, cid)
		}
	}
	r, err := mod.GetVideoByURI(ctx, uri)
	if err != nil || r == nil || r.Source.MediaDefs_SourceClip == nil {
		return false, err
	}
	ok, err := StationVideoAllowed(mod, uri)
	if err != nil || !ok {
		return false, err
	}
	for _, source := range entry.sources {
		if source == r.Source.MediaDefs_SourceClip.Video {
			return stationSourcePlayable(ctx, mod, source, cid)
		}
	}
	return false, nil
}

// StationCanServe answers from authoritative bucket intent, never a firehose
// origin. Even on a cold edge the author cannot be substituted by a caller-
// supplied DID, nor can an unknown content CID masquerade as an init segment.
func StationCanServe(ctx context.Context, store blob.Store, mod model.Model, did, cid string, allowInit bool) (bool, error) {
	if store == nil || !validStationCID(cid) {
		return false, nil
	}
	entry, known, err := stationOriginState(ctx, store, did, cid)
	if err != nil || entry.Withdrawn {
		return false, err
	}
	if !known {
		if allowInit {
			return stationMarkerExists(ctx, store, stationOriginPrefix(did)+"init/"+cid)
		}
		return false, nil
	}
	for _, source := range entry.sources {
		ok, err := stationSourcePlayable(ctx, mod, source, cid)
		if err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

func stationSourcePlayable(ctx context.Context, mod model.Model, source, cid string) (bool, error) {
	allowed, err := StationVideoAllowed(mod, source)
	if !allowed || err != nil {
		return false, err
	}
	// A cold index is not affirmative evidence of eligibility: the approved
	// public record may have been deleted or reassigned to another blob.
	uri := syntax.ATURI(source)
	r, err := mod.GetVideoByURI(ctx, source)
	if err != nil || r == nil || r.Source.MediaDefs_SourceTracks == nil {
		return false, err
	}
	refs := r.Source.MediaDefs_SourceTracks.Tracks
	if len(refs) == 0 {
		return false, nil
	}
	trackURI, err := syntax.ParseATURI(refs[0].Uri)
	if err != nil || trackURI.Authority() != uri.Authority() || trackURI.Collection().String() != constants.PLACE_STREAM_MEDIA_TRACK {
		return false, nil
	}
	tracks, err := mod.GetMediaTracksByBlob(ctx, cid)
	if err != nil {
		return false, err
	}
	found := false
	for _, t := range tracks {
		if t.URI == refs[0].Uri && t.CID == refs[0].Cid {
			found = true
		}
		if t.RepoDID != uri.Authority().String() {
			labels, err := mod.GetActiveLabels(t.RepoDID)
			if err != nil || atproto.IsBanned(labels...) {
				return false, err
			}
		}
	}
	return found, nil
}
