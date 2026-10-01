package media

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/captions"
	"stream.place/streamplace/pkg/captions/fmp4"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/muxl"
)

func signedMediaDate(t *testing.T, ctx context.Context, data []byte) time.Time {
	t.Helper()
	output, err := muxl.RunMuxlVerify(ctx, bytes.NewReader(data))
	require.NoError(t, err)
	var doc struct {
		Segments []struct {
			Manifest struct {
				Assertions []struct {
					Label string                     `json:"label"`
					Data  map[string]json.RawMessage `json:"data"`
				} `json:"assertions"`
			} `json:"manifest"`
		} `json:"segments"`
	}
	require.NoError(t, json.Unmarshal([]byte(output), &doc))
	for _, segment := range doc.Segments {
		for _, a := range segment.Manifest.Assertions {
			if a.Label == "cawg.metadata" {
				var date string
				require.NoError(t, json.Unmarshal(a.Data["dc:date"], &date))
				parsed, err := time.Parse(time.RFC3339Nano, date)
				require.NoError(t, err)
				return parsed
			}
		}
	}
	t.Fatal("signed segment has no media date")
	return time.Time{}
}
func TestCaptionMasterSignedDatesFollowMediaNotSigning(t *testing.T) {
	ctx := context.Background()
	fixture, err := os.ReadFile(getFixture("h264-opus-frag.mp4"))
	require.NoError(t, err)
	tracks, err := fmp4.Tracks(fixture)
	require.NoError(t, err)
	var timescale uint32
	for _, track := range tracks {
		if track.ID == 1 {
			timescale = track.Timescale
		}
	}
	mm := NewOffline(&config.CLI{})
	ms := newBareSegmentSigner(t)
	ms.PrebuiltManifest = captionManifest("off")
	events := make(chan *muxl.MuxlEvent, 16)
	done := make(chan error, 1)
	input := newIngestByteBuffer()
	wall := time.Now()
	source := bytes.NewReader(fixture)
	var moof []byte
	for source.Len() > 0 {
		box, kind, err := readCaptionBox(source)
		require.NoError(t, err)
		at := wall
		if kind == "moof" {
			moof = box
		}
		if kind == "mdat" && len(moof) > 0 {
			fragments, err := fmp4.Fragments(append(moof, box...))
			require.NoError(t, err)
			for _, fragment := range fragments {
				for _, track := range tracks {
					if fragment.TrackID == track.ID {
						at = wall.Add(time.Duration(fragment.BaseDecodeTime) * time.Second / time.Duration(track.Timescale))
						break
					}
				}
				break
			}
		}
		_, err = input.writeAt(box, at)
		require.NoError(t, err)
	}
	require.NoError(t, input.Close())
	var signed []*muxl.MuxlEvent
	go func() { done <- mm.SignOriginStream(ctx, ms, input, events); close(events) }()
	for event := range events {
		if event.Type == "signed-segment" {
			signed = append(signed, event)
		}
	}
	require.NoError(t, <-done)
	require.Len(t, signed, 2)
	first := signedMediaDate(t, ctx, signed[0].Tracks["1"])
	second := signedMediaDate(t, ctx, signed[1].Tracks["1"])
	expected := time.Duration((signed[1].FirstDecodeTimes["1"]-signed[0].FirstDecodeTimes["1"])*1000/uint64(timescale)) * time.Millisecond
	require.Equal(t, expected, second.Sub(first), "queued GoPs must retain their media-clock spacing")
}

func TestCaptionMasterClockReanchorsPushAndSignedTimeTogether(t *testing.T) {
	ctx := context.Background()
	m := newCaptionMaster(ctx, "streamer", &config.CLI{}, nil)
	m.setManifest(captionManifest("ingest"))
	m.mediaFinished = true
	wall := time.UnixMilli(1800000000000)
	m.clockAt(time.UnixMilli(0), wall)
	m.closeGopAt(1000, wall.Add(time.Second))
	require.Equal(t, wall.Add(time.Second), m.segmentTime(1000))
	m.closeGopAt(2000, wall.Add(5*time.Second))
	segmentWall := m.segmentTime(2000)
	require.Equal(t, wall.Add(5*time.Second), segmentWall)
	start, end := segmentWall.Add(200*time.Millisecond), segmentWall.Add(400*time.Millisecond)
	require.NoError(t, m.push(masterTrack(captions.SourceHuman), []captions.Cue{{ID: "reanchored", Start: start, End: end, Text: "after stall", Final: true}}))
	attached, err := m.text(ctx, muxl.TextRequest{StartMs: 2000, EndMs: 3000})
	require.NoError(t, err)
	require.Len(t, attached.Tracks, 1)
	require.Len(t, attached.Tracks[0].Cues, 1)
	cue := attached.Tracks[0].Cues[0]
	require.Equal(t, start, segmentWall.Add(time.Duration(cue.Start-2000)*time.Millisecond))
	require.Equal(t, end, segmentWall.Add(time.Duration(cue.End-2000)*time.Millisecond))
}
