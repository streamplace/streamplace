package muxl

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
)

// MuxlEvent represents a CBOR event from the muxl segmenter.
//
// Wire format (one CBOR map per event):
//
//	{"type": "init", "data": h'<ftyp+moov bytes>', "catalog": {"video": {...}, "audio": {...}}}
//	{"type": "segment", "tracks": {"1": h'<video>', "2": h'<audio>'},
//	 "durations": {"1": 60000, "2": 48000}, "sample_counts": {"1": 60, "2": 50}}
type MuxlEvent struct {
	Type         string            `cbor:"type"`
	Data         []byte            `cbor:"data,omitempty"`
	Catalog      *Catalog          `cbor:"catalog,omitempty"`
	TrackInits   map[string][]byte `cbor:"track_inits,omitempty"`
	Tracks       map[string][]byte `cbor:"tracks,omitempty"`
	Durations    map[string]uint64 `cbor:"durations,omitempty"`
	SampleCounts map[string]uint32 `cbor:"sample_counts,omitempty"`
}

// Catalog describes all tracks in a presentation.
type Catalog struct {
	Video map[string]VideoTrackConfig `cbor:"video"`
	Audio map[string]AudioTrackConfig `cbor:"audio"`
}

// VideoTrackConfig is the configuration for a video track.
type VideoTrackConfig struct {
	Codec       string `cbor:"codec"`
	Description []byte `cbor:"description"`
	CodedWidth  uint32 `cbor:"coded_width"`
	CodedHeight uint32 `cbor:"coded_height"`
	TrackID     uint32 `cbor:"track_id"`
	Timescale   uint32 `cbor:"timescale"`
}

// AudioTrackConfig is the configuration for an audio track.
type AudioTrackConfig struct {
	Codec            string `cbor:"codec"`
	Description      []byte `cbor:"description"`
	SampleRate       uint32 `cbor:"sample_rate"`
	NumberOfChannels uint32 `cbor:"number_of_channels"`
	TrackID          uint32 `cbor:"track_id"`
	Timescale        uint32 `cbor:"timescale"`
}

// PlaylistGenerator builds HLS CMAF playlists incrementally from a stream
// of MuxlEvents. It is safe for concurrent use: call HandleEvent from one
// goroutine while serving playlists from HTTP handlers in others.
//
// Segment URIs follow the convention "init.mp4" for the initialization
// segment and "{trackID}/{segmentNumber}.m4s" for media segments. The
// consumer's HTTP handler maps these URIs to the data stored via InitData
// and TrackSegmentData.
type PlaylistGenerator struct {
	mu sync.RWMutex

	catalog        *Catalog
	initData       []byte
	tracks         map[string]*trackState
	sortedTrackIDs []string

	// done is set when the stream ends (via Close), adding #EXT-X-ENDLIST.
	done bool
}

type trackState struct {
	segments []hlsSegment
}

type hlsSegment struct {
	data         []byte
	durationSecs float64
	sampleCount  uint32
}

// NewPlaylistGenerator creates a PlaylistGenerator ready to receive events.
func NewPlaylistGenerator() *PlaylistGenerator {
	return &PlaylistGenerator{
		tracks: map[string]*trackState{},
	}
}

// HandleEvent processes a single MuxlEvent, updating internal state.
func (p *PlaylistGenerator) HandleEvent(ev MuxlEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch ev.Type {
	case "init":
		if ev.Catalog == nil {
			return fmt.Errorf("init event missing catalog")
		}
		p.catalog = ev.Catalog
		p.initData = ev.Data

		// Build sorted track ID list (matches archive byte order).
		ids := []string{}
		for _, v := range p.catalog.Video {
			ids = append(ids, fmt.Sprintf("%d", v.TrackID))
		}
		for _, a := range p.catalog.Audio {
			ids = append(ids, fmt.Sprintf("%d", a.TrackID))
		}
		sort.Strings(ids)
		p.sortedTrackIDs = ids

		for _, tid := range ids {
			if _, ok := p.tracks[tid]; !ok {
				p.tracks[tid] = &trackState{}
			}
		}

	case "segment":
		if p.catalog == nil {
			return fmt.Errorf("segment event before init")
		}
		for tid, data := range ev.Tracks {
			ts := p.tracks[tid]
			if ts == nil {
				ts = &trackState{}
				p.tracks[tid] = ts
			}
			timescale := p.findTimescale(tid)
			dur := float64(ev.Durations[tid]) / float64(timescale)
			ts.segments = append(ts.segments, hlsSegment{
				data:         data,
				durationSecs: dur,
				sampleCount:  ev.SampleCounts[tid],
			})
		}
	}
	return nil
}

// Close marks the stream as finished, causing playlists to include
// #EXT-X-ENDLIST on subsequent calls.
func (p *PlaylistGenerator) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.done = true
}

// InitData returns the raw init segment bytes (ftyp+moov), or nil if no
// init event has been received yet.
func (p *PlaylistGenerator) InitData() []byte {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.initData
}

// TrackSegmentData returns the raw moof+mdat bytes for a given track and
// segment number (0-based). Returns nil if the segment doesn't exist yet.
func (p *PlaylistGenerator) TrackSegmentData(trackID string, segmentNumber int) []byte {
	p.mu.RLock()
	defer p.mu.RUnlock()
	ts := p.tracks[trackID]
	if ts == nil || segmentNumber >= len(ts.segments) {
		return nil
	}
	return ts.segments[segmentNumber].data
}

// MasterPlaylist returns the HLS master playlist, or empty string if no
// init event has been received yet.
func (p *PlaylistGenerator) MasterPlaylist() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.catalog == nil {
		return ""
	}

	var b strings.Builder
	fmt.Fprintln(&b, "#EXTM3U")
	fmt.Fprintln(&b, "#EXT-X-VERSION:6")
	fmt.Fprintln(&b)

	// Audio renditions
	for rendName, audio := range p.catalog.Audio {
		tid := fmt.Sprintf("%d", audio.TrackID)
		fmt.Fprintf(&b, "#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",NAME=%q,DEFAULT=YES,AUTOSELECT=YES,CHANNELS=%q,URI=%q\n",
			rendName, fmt.Sprintf("%d", audio.NumberOfChannels), fmt.Sprintf("audio-%s.m3u8", tid))
	}
	fmt.Fprintln(&b)

	// Video variants
	for _, video := range p.catalog.Video {
		tid := fmt.Sprintf("%d", video.TrackID)

		// Compute average bandwidth from all segments so far
		avgBandwidth := 0
		frameRate := 0.0
		if ts := p.tracks[tid]; ts != nil && len(ts.segments) > 0 {
			totalBytes := 0
			totalDur := 0.0
			for _, seg := range ts.segments {
				totalBytes += len(seg.data)
				totalDur += seg.durationSecs
			}
			if totalDur > 0 {
				avgBandwidth = int(float64(totalBytes) * 8 / totalDur)
			}
			// Frame rate from first segment
			if ts.segments[0].durationSecs > 0 {
				frameRate = float64(ts.segments[0].sampleCount) / ts.segments[0].durationSecs
			}
		}

		// Codec string: video + all audio codecs
		codecs := video.Codec
		for _, audio := range p.catalog.Audio {
			codecs += "," + audio.Codec
		}

		fmt.Fprintf(&b, "#EXT-X-STREAM-INF:AUDIO=\"audio\",AVERAGE-BANDWIDTH=%d,CODECS=%q,RESOLUTION=%dx%d,FRAME-RATE=%.3f\n",
			avgBandwidth, codecs, video.CodedWidth, video.CodedHeight, frameRate)
		fmt.Fprintf(&b, "video-%s.m3u8\n", tid)
	}

	return b.String()
}

// MediaPlaylist returns the HLS media playlist for a given track ID, or
// empty string if the track doesn't exist.
func (p *PlaylistGenerator) MediaPlaylist(trackID string) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ts := p.tracks[trackID]
	if ts == nil || p.catalog == nil {
		return "", fmt.Errorf("track %s not found", trackID)
	}

	// Compute target duration
	maxDur := 0.0
	for _, seg := range ts.segments {
		if seg.durationSecs > maxDur {
			maxDur = seg.durationSecs
		}
	}
	targetDuration := int(math.Ceil(maxDur))
	if targetDuration < 1 {
		targetDuration = 1
	}

	var b strings.Builder
	fmt.Fprintln(&b, "#EXTM3U")
	fmt.Fprintln(&b, "#EXT-X-VERSION:6")
	if p.done {
		fmt.Fprintln(&b, "#EXT-X-PLAYLIST-TYPE:VOD")
	} else {
		fmt.Fprintln(&b, "#EXT-X-PLAYLIST-TYPE:EVENT")
	}
	fmt.Fprintln(&b, "#EXT-X-INDEPENDENT-SEGMENTS")
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", targetDuration)
	fmt.Fprintln(&b, "#EXT-X-MEDIA-SEQUENCE:0")
	fmt.Fprintf(&b, "#EXT-X-MAP:URI=%q\n", "init.mp4")
	fmt.Fprintln(&b)

	for i, seg := range ts.segments {
		fmt.Fprintf(&b, "#EXTINF:%.6f,\n", seg.durationSecs)
		fmt.Fprintf(&b, "%s/%d.m4s\n", trackID, i)
	}

	if p.done {
		fmt.Fprintln(&b, "#EXT-X-ENDLIST")
	}

	return b.String(), nil
}

// TrackIDs returns the sorted list of track IDs, or nil if no init event
// has been received yet.
func (p *PlaylistGenerator) TrackIDs() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sortedTrackIDs
}

// SegmentCount returns the number of segments received so far for a track.
func (p *PlaylistGenerator) SegmentCount(trackID string) int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if ts := p.tracks[trackID]; ts != nil {
		return len(ts.segments)
	}
	return 0
}

func (p *PlaylistGenerator) findTimescale(trackID string) uint32 {
	for _, v := range p.catalog.Video {
		if fmt.Sprintf("%d", v.TrackID) == trackID {
			return v.Timescale
		}
	}
	for _, a := range p.catalog.Audio {
		if fmt.Sprintf("%d", a.TrackID) == trackID {
			return a.Timescale
		}
	}
	return 1
}
