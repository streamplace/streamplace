package media

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/bus"
)

type trackWrite struct {
	data       []byte
	start, end time.Time
}

type recordingTrack struct {
	mu     sync.Mutex
	delay  time.Duration
	writes []trackWrite
}

func (rt *recordingTrack) WriteSample(sample media.Sample) error {
	start := time.Now()
	time.Sleep(rt.delay)
	rt.mu.Lock()
	rt.writes = append(rt.writes, trackWrite{data: sample.Data, start: start, end: time.Now()})
	rt.mu.Unlock()
	return nil
}

func (rt *recordingTrack) recorded() []trackWrite {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return append([]trackWrite(nil), rt.writes...)
}

// writePacketizedSegment must not return until every sample of the segment is
// written: the playback loop pulls the next segment the moment it returns, so
// an early return interleaves two GoPs' frames into one RTP track. (The old
// loop only awaited the audio writer, so a segment with video but no audio
// samples overlapped the next segment's writes.)
func TestWritePacketizedSegmentReturnsAfterAllWrites(t *testing.T) {
	ctx := context.Background()
	video := &recordingTrack{delay: 10 * time.Millisecond}
	audio := &recordingTrack{delay: 10 * time.Millisecond}
	packet := &bus.PacketizedSegment{
		Video:    [][]byte{[]byte("v0"), []byte("v1"), []byte("v2")},
		Duration: 3 * time.Millisecond,
	}
	err := writePacketizedSegment(ctx, packet, video, audio, false, 1.0)
	require.NoError(t, err)
	require.Len(t, video.recorded(), 3, "video writes still in flight at return")
	require.Len(t, audio.recorded(), 0)

	// a second segment's writes must start after the first segment's ended
	err = writePacketizedSegment(ctx, packet, video, audio, false, 1.0)
	require.NoError(t, err)
	writes := video.recorded()
	require.Len(t, writes, 6)
	require.False(t, writes[3].start.Before(writes[2].end), "segments overlapped on the video track")
	for i, w := range writes {
		require.Equal(t, []byte{byte('v'), byte('0' + i%3)}, w.data)
	}
}

// With both tracks present, video and audio write concurrently but both
// complete before return.
func TestWritePacketizedSegmentWaitsForBothTracks(t *testing.T) {
	ctx := context.Background()
	video := &recordingTrack{delay: 10 * time.Millisecond}
	audio := &recordingTrack{delay: 10 * time.Millisecond}
	packet := &bus.PacketizedSegment{
		Video:    [][]byte{[]byte("v0"), []byte("v1")},
		Audio:    [][]byte{[]byte("a0"), []byte("a1"), []byte("a2")},
		Duration: 2 * time.Millisecond,
	}
	err := writePacketizedSegment(ctx, packet, video, audio, false, 1.0)
	require.NoError(t, err)
	require.Len(t, video.recorded(), 2)
	require.Len(t, audio.recorded(), 3)
}

func TestWritePacketizedSegmentAudioOnly(t *testing.T) {
	ctx := context.Background()
	audio := &recordingTrack{}
	packet := &bus.PacketizedSegment{
		Audio:    [][]byte{[]byte("a0")},
		Duration: time.Millisecond,
	}
	err := writePacketizedSegment(ctx, packet, nil, audio, true, 1.0)
	require.NoError(t, err)
	require.Len(t, audio.recorded(), 1)
}
