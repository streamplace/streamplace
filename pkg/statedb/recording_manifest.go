package statedb

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/google/uuid"
	"stream.place/streamplace/pkg/s3"
	"stream.place/streamplace/pkg/stationshard"
)

const (
	recordingRole     = "record"
	recordingPageSize = 32
)

var (
	errRecordingLeaseRequired = errors.New("sharded recording requires a bound record lease")
	errRecordingUnchanged     = errors.New("recording role unchanged")
)

// Renewal only carries the open object and the head of the completed log.
type recordingControl struct {
	Open *recordingObject `json:"open,omitempty"`
	Head string           `json:"head,omitempty"`
}

type recordingFence struct {
	Owner       string `json:"owner"`
	Incarnation string `json:"incarnation"`
	Epoch       uint64 `json:"epoch"`
}

func recordingWriterFence(lease stationshard.Lease) recordingFence {
	return recordingFence{Owner: lease.Owner, Incarnation: lease.Incarnation, Epoch: lease.Epoch}
}

type recordingObject struct {
	ID     string         `json:"id"`
	Fence  recordingFence `json:"fence"`
	URI    string         `json:"uri,omitempty"`
	Bucket string         `json:"bucket"`
	Key    string         `json:"key"`
	Start  time.Time      `json:"start"`
	Opened time.Time      `json:"opened"`
}

func (obj recordingObject) segment(did string) S3Segment {
	return S3Segment{
		ID: obj.ID, RepoDID: did, LivestreamURI: obj.URI, Bucket: obj.Bucket, Key: obj.Key,
		StartedAt: obj.Start, CreatedAt: obj.Opened, UpdatedAt: obj.Opened,
	}
}

type recordingEntry struct {
	recordingObject
	EndMS int64 `json:"end_ms"`
}

type recordingPage struct {
	Prev    string           `json:"prev,omitempty"`
	Entries []recordingEntry `json:"entries"`
}

type recordingIndex struct {
	CompletedAt time.Time            `json:"completed_at"`
	Parts       int32                `json:"parts"`
	Size        int64                `json:"size"`
	Segments    []s3.RecordedSegment `json:"segments"`
}

func recordingIndexKey(id string) string { return "recording/index/" + id }

func recordingPageKey(data []byte) string {
	return fmt.Sprintf("recording/pages/%x", sha256.Sum256(data))
}

func recordingAuthority(uri string) (string, error) {
	parsed, err := syntax.ParseATURI(uri)
	if err != nil {
		return "", fmt.Errorf("parse recording chapter URI %q: %w", uri, err)
	}
	if parsed.Collection() != "place.stream.livestream" || parsed.RecordKey() == "" {
		return "", fmt.Errorf("not a livestream record URI: %q", uri)
	}
	did, err := parsed.Authority().AsDID()
	if err != nil {
		return "", fmt.Errorf("recording chapter URI %q needs a DID authority: %w", uri, err)
	}
	return did.String(), nil
}

func recordingContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func decodeRecordingControl(data []byte) (recordingControl, error) {
	var control recordingControl
	if len(data) != 0 {
		if err := json.Unmarshal(data, &control); err != nil {
			return recordingControl{}, fmt.Errorf("decode recording role: %w", err)
		}
	}
	return control, nil
}

func readRecordingRole(ctx context.Context, store *stationshard.Store, did string) (recordingControl, error) {
	data, active, err := store.ReadRoleState(recordingContext(ctx), did, recordingRole)
	if errors.Is(err, stationshard.ErrNotFound) {
		return recordingControl{}, nil
	}
	if err != nil {
		return recordingControl{}, fmt.Errorf("read recording role for %s: %w", did, err)
	}
	control, err := decodeRecordingControl(data)
	// Only the active epoch can finish an open object. Its predecessor's
	// bytes must not block a clip or automatic publish after lease expiry.
	if control.Open != nil && control.Open.Fence != recordingWriterFence(active) {
		control.Open = nil
	}
	return control, err
}

func readRecordingPage(ctx context.Context, store *stationshard.Store, did, key string) (recordingPage, error) {
	data, err := store.GetImmutable(ctx, did, key)
	if err != nil {
		return recordingPage{}, fmt.Errorf("read recording page %q: %w", key, err)
	}
	if recordingPageKey(data) != key {
		return recordingPage{}, fmt.Errorf("recording page %q has the wrong digest", key)
	}
	var page recordingPage
	if err := json.Unmarshal(data, &page); err != nil {
		return recordingPage{}, fmt.Errorf("decode recording page %q: %w", key, err)
	}
	if len(page.Entries) == 0 || len(page.Entries) > recordingPageSize {
		return recordingPage{}, fmt.Errorf("recording page %q has %d entries", key, len(page.Entries))
	}
	return page, nil
}

func readRecordingIndex(ctx context.Context, store *stationshard.Store, did, id string) (recordingIndex, error) {
	data, err := store.GetImmutable(ctx, did, recordingIndexKey(id))
	if err != nil {
		return recordingIndex{}, fmt.Errorf("read recording index for %s: %w", id, err)
	}
	var idx recordingIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return recordingIndex{}, fmt.Errorf("decode recording index for %s: %w", id, err)
	}
	return idx, nil
}

// NewRecordingWriter binds every mutation to the supplied lease, not to a
// lease that may subsequently be acquired by the same node.
func (state *StatefulDB) NewRecordingWriter(ctx context.Context, lease stationshard.Lease) (s3.Recorder, error) {
	if state.StationShards == nil {
		return state, nil
	}
	if lease.Role != recordingRole {
		return nil, errRecordingLeaseRequired
	}
	writer := &recordingWriter{store: state.StationShards, lease: lease}
	fence := recordingWriterFence(lease)
	if err := writer.update(ctx, func(control *recordingControl) error {
		if control.Open != nil && control.Open.Fence != fence {
			control.Open = nil
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("activate recording writer for %s: %w", lease.DID, err)
	}
	return writer, nil
}

type recordingWriter struct {
	store *stationshard.Store
	lease stationshard.Lease
}

// edit may replace Open, but must not modify its contents or do external I/O.
func (w *recordingWriter) update(ctx context.Context, edit func(*recordingControl) error) error {
	err := w.store.UpdateRole(recordingContext(ctx), w.lease, func(data []byte) ([]byte, error) {
		control, err := decodeRecordingControl(data)
		if err != nil {
			return nil, err
		}
		before := control
		if err := edit(&control); err != nil {
			return nil, err
		}
		if control == before {
			return nil, errRecordingUnchanged
		}
		return json.Marshal(control)
	})
	if errors.Is(err, errRecordingUnchanged) {
		return nil
	}
	return err
}

type recordingAlreadyOpen string

func (id recordingAlreadyOpen) Error() string { return "recording already open: " + string(id) }

func (w *recordingWriter) RecordStart(ctx context.Context, did, bucket, key, uri string, started time.Time) (string, error) {
	if did != w.lease.DID {
		return "", fmt.Errorf("start recording for %s with lease for %s: %w", did, w.lease.DID, stationshard.ErrFenced)
	}
	if uri != "" {
		authority, err := recordingAuthority(uri)
		if err != nil {
			return "", err
		}
		if authority != did {
			return "", fmt.Errorf("recording chapter belongs to %s, not %s", authority, did)
		}
	}
	if len(bucket) > 255 || len(key) > 1024 {
		return "", fmt.Errorf("recording bucket or object key is too long")
	}
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	obj := recordingObject{
		ID: id.String(), Fence: recordingWriterFence(w.lease), URI: uri, Bucket: bucket, Key: key,
		Start: started.UTC(), Opened: time.Now().UTC(),
	}
	if err := w.update(ctx, func(control *recordingControl) error {
		if open := control.Open; open != nil {
			if open.Fence == obj.Fence && open.URI == uri && open.Bucket == bucket && open.Key == key && open.Start.Equal(started) {
				return recordingAlreadyOpen(open.ID)
			}
			return stationshard.ErrConflict
		}
		control.Open = &obj
		return nil
	}); err != nil {
		var existing recordingAlreadyOpen
		if errors.As(err, &existing) {
			return string(existing), nil
		}
		return "", fmt.Errorf("open recording %q: %w", key, err)
	}
	return obj.ID, nil
}

func (w *recordingWriter) ownsOpen(obj *recordingObject, id string) bool {
	return obj != nil && obj.ID == id && obj.Fence == recordingWriterFence(w.lease)
}

func (w *recordingWriter) RecordAbort(ctx context.Context, id string) error {
	if err := w.update(ctx, func(control *recordingControl) error {
		if !w.ownsOpen(control.Open, id) {
			return stationshard.ErrConflict
		}
		control.Open = nil
		return nil
	}); err != nil {
		return fmt.Errorf("abort recording %s: %w", id, err)
	}
	return nil
}

func (w *recordingWriter) RecordComplete(ctx context.Context, id string, parts int32, size int64, segments []s3.RecordedSegment) error {
	ctx = recordingContext(ctx)
	control, err := readRecordingRole(ctx, w.store, w.lease.DID)
	if err != nil {
		return err
	}
	open := w.ownsOpen(control.Open, id)
	// Check the fence before staging. A takeover during staging can leave
	// unlinked objects, but cannot publish them.
	if err := w.update(ctx, func(current *recordingControl) error {
		if current.Head != control.Head || (open && !w.ownsOpen(current.Open, id)) {
			return stationshard.ErrConflict
		}
		return nil
	}); err != nil {
		return fmt.Errorf("prepare recording completion %s: %w", id, err)
	}
	index := recordingIndex{CompletedAt: time.Now().UTC(), Parts: parts, Size: size, Segments: segments}
	if !open {
		return w.checkCompleted(ctx, control.Head, id, index)
	}
	if err := w.stageIndex(ctx, id, index); err != nil {
		return err
	}

	entry := recordingEntry{recordingObject: *control.Open, EndMS: control.Open.Start.UnixMilli()}
	if len(segments) > 0 {
		first, last := segments[0].StartMs, segments[0].StartMs
		for _, seg := range segments[1:] {
			first, last = min(first, seg.StartMs), max(last, seg.StartMs)
		}
		entry.Start, entry.EndMS = time.UnixMilli(first).UTC(), last
	}
	var page recordingPage
	if control.Head != "" {
		page, err = readRecordingPage(ctx, w.store, w.lease.DID, control.Head)
		if err != nil {
			return err
		}
		if len(page.Entries) == recordingPageSize {
			page = recordingPage{Prev: control.Head}
		}
	}
	page.Entries = append(page.Entries, entry)
	data, err := json.Marshal(page)
	if err != nil {
		return fmt.Errorf("encode recording page for %s: %w", id, err)
	}
	head := recordingPageKey(data)
	if err := w.store.PutImmutable(ctx, w.lease.DID, head, data); err != nil {
		return fmt.Errorf("stage recording page for %s: %w", id, err)
	}
	if err := w.update(ctx, func(current *recordingControl) error {
		if current.Head == head {
			return nil // An identical completion already published this page.
		}
		if current.Head != control.Head || !w.ownsOpen(current.Open, id) {
			return stationshard.ErrConflict
		}
		current.Head, current.Open = head, nil
		return nil
	}); err != nil {
		return fmt.Errorf("publish recording completion %s: %w", id, err)
	}
	return nil
}

func (w *recordingWriter) stageIndex(ctx context.Context, id string, idx recordingIndex) error {
	data, err := json.Marshal(idx)
	if err != nil {
		return fmt.Errorf("encode recording index for %s: %w", id, err)
	}
	err = w.store.PutImmutable(ctx, w.lease.DID, recordingIndexKey(id), data)
	if errors.Is(err, stationshard.ErrConflict) {
		// Preserve the timestamp from an earlier attempt, including a PUT
		// that committed but whose response was lost.
		return w.checkIndex(ctx, id, idx)
	}
	if err != nil {
		return fmt.Errorf("stage recording index for %s: %w", id, err)
	}
	return nil
}

func (w *recordingWriter) checkIndex(ctx context.Context, id string, want recordingIndex) error {
	got, err := readRecordingIndex(ctx, w.store, w.lease.DID, id)
	if err != nil {
		return err
	}
	if got.Parts != want.Parts || got.Size != want.Size || !slices.Equal(got.Segments, want.Segments) {
		return fmt.Errorf("recording completion data differs for %s: %w", id, stationshard.ErrConflict)
	}
	return nil
}

func (w *recordingWriter) checkCompleted(ctx context.Context, head, id string, idx recordingIndex) error {
	for head != "" {
		page, err := readRecordingPage(ctx, w.store, w.lease.DID, head)
		if err != nil {
			return err
		}
		for _, entry := range page.Entries {
			if entry.ID == id && entry.Fence == recordingWriterFence(w.lease) {
				return w.checkIndex(ctx, id, idx)
			}
		}
		head = page.Prev
	}
	return fmt.Errorf("no matching open or completed recording %s: %w", id, stationshard.ErrConflict)
}

func (state *StatefulDB) listRecordings(ctx context.Context, did string, match func(recordingEntry) bool) ([]S3Segment, error) {
	ctx = recordingContext(ctx)
	control, err := readRecordingRole(ctx, state.StationShards, did)
	if err != nil {
		return nil, err
	}
	var segs []S3Segment
	for head := control.Head; head != ""; {
		page, err := readRecordingPage(ctx, state.StationShards, did, head)
		if err != nil {
			return nil, err
		}
		for _, entry := range page.Entries {
			if !match(entry) {
				continue
			}
			idx, err := readRecordingIndex(ctx, state.StationShards, did, entry.ID)
			if err != nil {
				return nil, err
			}
			seg := entry.segment(did)
			seg.CompletedAt, seg.UpdatedAt = &idx.CompletedAt, idx.CompletedAt
			seg.Size, seg.PartCount, seg.Segments = idx.Size, idx.Parts, idx.Segments
			segs = append(segs, seg)
		}
		head = page.Prev
	}
	slices.SortFunc(segs, func(a, b S3Segment) int {
		if order := a.StartedAt.Compare(b.StartedAt); order != 0 {
			return order
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return segs, nil
}
