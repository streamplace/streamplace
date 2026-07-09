package s3

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/spmetrics"
)

// Spool is a bounded write-ahead buffer for live-rec segments on local disk.
// Segments land here before they're uploaded, and are deleted only after the
// S3 object containing them completes — so an upload failure (provider
// incompatibility, outage, node crash) costs lag instead of losing recorded
// user bytes. Before the spool existed the uploader's only buffer was ~one
// part of process memory, and any sustained failure dropped the stream.
//
// Layout: one file per segment, named by a monotonically increasing sequence
// number (000000000001.seg …), plus meta.jsonl recording the livestream URI
// in effect from a given seq onward (appended only when it changes). A file's
// mtime is the segment's arrival time; the uploader uses it as the honest
// started-at for objects assembled during catch-up, so late uploads still
// sort correctly in the finalize.
//
// All methods are safe for concurrent use (the appender and the upload loop
// are different goroutines).
type Spool struct {
	dir      string
	maxBytes int64

	mu         sync.Mutex
	nextSeq    int64
	seqs       []int64 // live (un-acked, un-evicted) seqs, ascending
	sizes      map[int64]int64
	totalBytes int64
	epochs     []uriEpoch // URI in effect from Seq onward; ascending by Seq
}

type uriEpoch struct {
	Seq int64  `json:"seq"`
	URI string `json:"uri"`
}

const spoolMetaFile = "meta.jsonl"

func spoolSegFile(seq int64) string { return fmt.Sprintf("%012d.seg", seq) }

// OpenSpool opens (or creates) a spool directory. Existing segment files are
// picked up, which is how salvage resumes a spool left behind by a crashed or
// failed run. maxBytes <= 0 means unbounded (used by salvage, which only
// drains).
func OpenSpool(dir string, maxBytes int64) (*Spool, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating spool dir %s: %w", dir, err)
	}
	s := &Spool{
		dir:      dir,
		maxBytes: maxBytes,
		nextSeq:  1,
		sizes:    map[int64]int64{},
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading spool dir %s: %w", dir, err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".seg") {
			continue
		}
		seq, err := strconv.ParseInt(strings.TrimSuffix(name, ".seg"), 10, 64)
		if err != nil {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		s.seqs = append(s.seqs, seq)
		s.sizes[seq] = info.Size()
		s.totalBytes += info.Size()
		if seq >= s.nextSeq {
			s.nextSeq = seq + 1
		}
	}
	sort.Slice(s.seqs, func(i, j int) bool { return s.seqs[i] < s.seqs[j] })
	if meta, err := os.ReadFile(filepath.Join(dir, spoolMetaFile)); err == nil {
		for _, line := range strings.Split(string(meta), "\n") {
			if line == "" {
				continue
			}
			var ep uriEpoch
			if err := json.Unmarshal([]byte(line), &ep); err == nil {
				s.epochs = append(s.epochs, ep)
			}
		}
	}
	spmetrics.S3LiveRecSpoolBytes.Add(float64(s.totalBytes))
	return s, nil
}

// Append durably writes one segment and returns its seq. If the spool is over
// its byte bound afterwards, the oldest segments are evicted (loudly — those
// are recorded bytes the uploader hasn't saved yet).
func (s *Spool) Append(ctx context.Context, data []byte, uri string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	seq := s.nextSeq
	if err := os.WriteFile(filepath.Join(s.dir, spoolSegFile(seq)), data, 0o644); err != nil {
		return 0, fmt.Errorf("writing spool segment %d: %w", seq, err)
	}
	s.nextSeq++
	s.seqs = append(s.seqs, seq)
	s.sizes[seq] = int64(len(data))
	s.totalBytes += int64(len(data))
	spmetrics.S3LiveRecSpoolBytes.Add(float64(len(data)))
	if len(s.epochs) == 0 || s.epochs[len(s.epochs)-1].URI != uri {
		ep := uriEpoch{Seq: seq, URI: uri}
		s.epochs = append(s.epochs, ep)
		line, _ := json.Marshal(ep)
		f, err := os.OpenFile(filepath.Join(s.dir, spoolMetaFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = f.Write(append(line, '\n'))
			_ = f.Close()
		} else {
			log.Error(ctx, "writing spool meta", "dir", s.dir, "error", err)
		}
	}
	// Enforce the bound, never evicting the segment just written.
	for s.maxBytes > 0 && s.totalBytes > s.maxBytes && len(s.seqs) > 1 {
		victim := s.seqs[0]
		victimBytes := s.sizes[victim]
		s.dropLocked(victim)
		spmetrics.S3LiveRecSpoolEvictedBytesTotal.Add(float64(victimBytes))
		spmetrics.S3LiveRecBytesDroppedTotal.Add(float64(victimBytes))
		log.Error(ctx, "live-rec spool over its bound; evicting oldest un-uploaded segment — these bytes will be missing from the recording",
			"dir", s.dir, "seq", victim, "evictedBytes", victimBytes, "spoolBytes", s.totalBytes, "maxBytes", s.maxBytes)
	}
	return seq, nil
}

// dropLocked removes one segment file and its accounting. Caller holds mu and
// is responsible for metrics/logging appropriate to why it's being dropped.
func (s *Spool) dropLocked(seq int64) {
	_ = os.Remove(filepath.Join(s.dir, spoolSegFile(seq)))
	s.totalBytes -= s.sizes[seq]
	spmetrics.S3LiveRecSpoolBytes.Sub(float64(s.sizes[seq]))
	delete(s.sizes, seq)
	for i, v := range s.seqs {
		if v == seq {
			s.seqs = append(s.seqs[:i], s.seqs[i+1:]...)
			break
		}
	}
}

// NextFrom returns the smallest live seq >= from, skipping evicted gaps.
func (s *Spool) NextFrom(from int64) (int64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := sort.Search(len(s.seqs), func(i int) bool { return s.seqs[i] >= from })
	if i == len(s.seqs) {
		return 0, false
	}
	return s.seqs[i], true
}

// Get reads a segment's bytes, arrival time, and the livestream URI in effect
// when it was appended.
func (s *Spool) Get(seq int64) ([]byte, time.Time, string, error) {
	path := filepath.Join(s.dir, spoolSegFile(seq))
	info, err := os.Stat(path)
	if err != nil {
		return nil, time.Time{}, "", fmt.Errorf("stat spool segment %d: %w", seq, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, "", fmt.Errorf("reading spool segment %d: %w", seq, err)
	}
	return data, info.ModTime(), s.uriAt(seq), nil
}

func (s *Spool) uriAt(seq int64) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	uri := ""
	for _, ep := range s.epochs {
		if ep.Seq > seq {
			break
		}
		uri = ep.URI
	}
	return uri
}

// Ack deletes every segment with seq <= upTo: they're durably inside a
// completed S3 object and no longer need local protection.
func (s *Spool) Ack(upTo int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for len(s.seqs) > 0 && s.seqs[0] <= upTo {
		s.dropLocked(s.seqs[0])
	}
}

// Bytes reports the un-acked bytes currently held.
func (s *Spool) Bytes() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.totalBytes
}

// Len reports the number of un-acked segments.
func (s *Spool) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.seqs)
}

// Destroy removes the spool directory entirely. Call only when drained (or
// when the data is intentionally being discarded).
func (s *Spool) Destroy() {
	s.mu.Lock()
	defer s.mu.Unlock()
	spmetrics.S3LiveRecSpoolBytes.Sub(float64(s.totalBytes))
	s.totalBytes = 0
	s.seqs = nil
	s.sizes = map[int64]int64{}
	_ = os.RemoveAll(s.dir)
}
