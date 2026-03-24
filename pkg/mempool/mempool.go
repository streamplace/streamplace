// Package mempool holds live MUXL segment data and playlists in memory.
//
// Each active stream gets a Mempool that receives structured MuxlEvents,
// stores per-track segment data, and generates HLS CMAF playlists on demand.
// The mempool is the segment index cache described in docs/architecture/vod-dvr.md —
// a derived structure, rebuildable from the MUXL archive data.
package mempool

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"stream.place/streamplace/pkg/muxl"
)

// Mempool holds live segment data and playlists for a single stream.
// Safe for concurrent use: one goroutine feeds events via HandleEvent
// while HTTP handlers read playlists and segment data concurrently.
type Mempool struct {
	mu sync.RWMutex

	catalog        *muxl.Catalog
	initData       []byte
	tracks         map[string]*trackState
	sortedTrackIDs []string
	done           bool
}

type trackState struct {
	segments []segment
}

type segment struct {
	data         []byte
	durationSecs float64
	sampleCount  uint32
	byteLength   int
}

// New creates a Mempool ready to receive events.
func New() *Mempool {
	return &Mempool{
		tracks: map[string]*trackState{},
	}
}

// HandleEvent processes a single MuxlEvent, updating the index and storing
// segment data. Call this from the goroutine reading MUXL CBOR events.
func (m *Mempool) HandleEvent(ev muxl.MuxlEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch ev.Type {
	case "init":
		if ev.Catalog == nil {
			return fmt.Errorf("init event missing catalog")
		}
		m.catalog = ev.Catalog
		m.initData = ev.Data

		ids := []string{}
		for _, v := range m.catalog.Video {
			ids = append(ids, fmt.Sprintf("%d", v.TrackID))
		}
		for _, a := range m.catalog.Audio {
			ids = append(ids, fmt.Sprintf("%d", a.TrackID))
		}
		sort.Strings(ids)
		m.sortedTrackIDs = ids

		for _, tid := range ids {
			if _, ok := m.tracks[tid]; !ok {
				m.tracks[tid] = &trackState{}
			}
		}

	case "segment":
		if m.catalog == nil {
			return fmt.Errorf("segment event before init")
		}
		for tid, data := range ev.Tracks {
			ts := m.tracks[tid]
			if ts == nil {
				ts = &trackState{}
				m.tracks[tid] = ts
			}
			timescale := m.findTimescale(tid)
			dur := float64(ev.Durations[tid]) / float64(timescale)
			ts.segments = append(ts.segments, segment{
				data:         data,
				durationSecs: dur,
				sampleCount:  ev.SampleCounts[tid],
				byteLength:   len(data),
			})
		}
	}
	return nil
}

// Close marks the stream as finished. Playlists will include #EXT-X-ENDLIST.
func (m *Mempool) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.done = true
}

// Catalog returns the track configuration, or nil if no init event received yet.
func (m *Mempool) Catalog() *muxl.Catalog {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.catalog
}

// InitData returns the raw init segment bytes (ftyp+moov).
func (m *Mempool) InitData() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.initData
}

// TrackSegmentData returns the raw moof+mdat bytes for a track and segment
// number (0-based). Returns nil if the segment doesn't exist yet.
func (m *Mempool) TrackSegmentData(trackID string, segmentNumber int) []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ts := m.tracks[trackID]
	if ts == nil || segmentNumber >= len(ts.segments) {
		return nil
	}
	return ts.segments[segmentNumber].data
}

// TrackIDs returns the sorted track IDs.
func (m *Mempool) TrackIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sortedTrackIDs
}

// SegmentCount returns the number of segments received for a track.
func (m *Mempool) SegmentCount(trackID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if ts := m.tracks[trackID]; ts != nil {
		return len(ts.segments)
	}
	return 0
}

// MasterPlaylist returns the HLS master playlist.
func (m *Mempool) MasterPlaylist() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.catalog == nil {
		return ""
	}

	var b strings.Builder
	fmt.Fprintln(&b, "#EXTM3U")
	fmt.Fprintln(&b, "#EXT-X-VERSION:6")
	fmt.Fprintln(&b)

	for rendName, audio := range m.catalog.Audio {
		tid := fmt.Sprintf("%d", audio.TrackID)
		fmt.Fprintf(&b, "#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",NAME=%q,DEFAULT=YES,AUTOSELECT=YES,CHANNELS=%q,URI=%q\n",
			rendName, fmt.Sprintf("%d", audio.NumberOfChannels), fmt.Sprintf("audio-%s.m3u8", tid))
	}
	fmt.Fprintln(&b)

	for _, video := range m.catalog.Video {
		tid := fmt.Sprintf("%d", video.TrackID)

		avgBandwidth := 0
		frameRate := 0.0
		if ts := m.tracks[tid]; ts != nil && len(ts.segments) > 0 {
			totalBytes := 0
			totalDur := 0.0
			for _, seg := range ts.segments {
				totalBytes += seg.byteLength
				totalDur += seg.durationSecs
			}
			if totalDur > 0 {
				avgBandwidth = int(float64(totalBytes) * 8 / totalDur)
			}
			if ts.segments[0].durationSecs > 0 {
				frameRate = float64(ts.segments[0].sampleCount) / ts.segments[0].durationSecs
			}
		}

		codecs := video.Codec
		for _, audio := range m.catalog.Audio {
			codecs += "," + audio.Codec
		}

		fmt.Fprintf(&b, "#EXT-X-STREAM-INF:AUDIO=\"audio\",AVERAGE-BANDWIDTH=%d,CODECS=%q,RESOLUTION=%dx%d,FRAME-RATE=%.3f\n",
			avgBandwidth, codecs, video.CodedWidth, video.CodedHeight, frameRate)
		fmt.Fprintf(&b, "video-%s.m3u8\n", tid)
	}

	return b.String()
}

// MediaPlaylist returns an HLS media playlist for a track. If endMs <= 0,
// produces an EVENT playlist (live DVR). Otherwise produces a VOD playlist
// covering the requested time range.
func (m *Mempool) MediaPlaylist(trackID string, startMs, endMs int64) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ts := m.tracks[trackID]
	if ts == nil || m.catalog == nil {
		return "", fmt.Errorf("track %s not found", trackID)
	}

	// Determine which segments fall in the requested time range.
	// Cumulative time is computed from segment durations.
	type indexedSeg struct {
		index        int
		seg          segment
		startTimeSec float64
	}
	var selected []indexedSeg
	cumTime := 0.0
	startSec := float64(startMs) / 1000.0
	endSec := float64(endMs) / 1000.0
	isLive := endMs <= 0

	for i, seg := range ts.segments {
		segEnd := cumTime + seg.durationSecs
		inRange := true
		if !isLive {
			// VOD: include segments that overlap with [startSec, endSec]
			if cumTime >= endSec || segEnd <= startSec {
				inRange = false
			}
		} else {
			// Live: include segments from startSec onward
			if segEnd <= startSec {
				inRange = false
			}
		}
		if inRange {
			selected = append(selected, indexedSeg{index: i, seg: seg, startTimeSec: cumTime})
		}
		cumTime = segEnd
	}

	// Compute target duration from selected segments
	maxDur := 0.0
	for _, s := range selected {
		if s.seg.durationSecs > maxDur {
			maxDur = s.seg.durationSecs
		}
	}
	targetDuration := int(math.Ceil(maxDur))
	if targetDuration < 1 {
		targetDuration = 1
	}

	isDone := m.done || !isLive

	var b strings.Builder
	fmt.Fprintln(&b, "#EXTM3U")
	fmt.Fprintln(&b, "#EXT-X-VERSION:6")
	if isDone {
		fmt.Fprintln(&b, "#EXT-X-PLAYLIST-TYPE:VOD")
	} else {
		fmt.Fprintln(&b, "#EXT-X-PLAYLIST-TYPE:EVENT")
	}
	fmt.Fprintln(&b, "#EXT-X-INDEPENDENT-SEGMENTS")
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", targetDuration)
	fmt.Fprintln(&b, "#EXT-X-MEDIA-SEQUENCE:0")
	fmt.Fprintf(&b, "#EXT-X-MAP:URI=%q\n", "init.mp4")
	fmt.Fprintln(&b)

	for _, s := range selected {
		fmt.Fprintf(&b, "#EXTINF:%.6f,\n", s.seg.durationSecs)
		fmt.Fprintf(&b, "%s/%d.m4s\n", trackID, s.index)
	}

	if isDone {
		fmt.Fprintln(&b, "#EXT-X-ENDLIST")
	}

	return b.String(), nil
}

func (m *Mempool) findTimescale(trackID string) uint32 {
	for _, v := range m.catalog.Video {
		if fmt.Sprintf("%d", v.TrackID) == trackID {
			return v.Timescale
		}
	}
	for _, a := range m.catalog.Audio {
		if fmt.Sprintf("%d", a.TrackID) == trackID {
			return a.Timescale
		}
	}
	return 1
}
