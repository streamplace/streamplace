package misttriggers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"stream.place/streamplace/pkg/config"
	"github.com/stretchr/testify/require"
)

// Note to anyone adding tests here: the "init" body can contain backticks,
// breaking the Go multi-line string. If you just replace the backticks with
// \u0060, everything works fine.
var liveTrackListPayload = MistTriggerBody(`
	video+3cb3cu6uq1msu78k
	{"audio_AAC_2ch_44100hz_1":{"bps":0,"channels":2,"codec":"AAC","firstms":60,"idx":1,"init":"\u0012\u0010V\u00E5\u0000","jitter":200,"lastms":1523,"maxbps":0,"rate":44100,"size":16,"trackid":2,"type":"audio"},"video_H264_256x144_30fps_2":{"bframes":1,"bps":20155,"codec":"H264","firstms":0,"fpks":30000,"height":144,"idx":2,"init":"\u0001d\u0000\f\u00FF\u00E1\u0000\u0018gd\u0000\f\u00AC\u00D9A\u0001:\u0010\u0000\u0000\u0003\u0000\u0010\u0000\u0000\u0003\u0003\u00C0\u00F1B\u0099\u0060\u0001\u0000\u0005h\u00EB\u00EC\u00B2,","jitter":200,"lastms":966,"maxbps":32107,"trackid":256,"type":"video","width":256},"video_H264_640x360_24fps_0":{"bframes":1,"bps":15797,"codec":"H264","firstms":0,"fpks":24000,"height":360,"idx":0,"init":"\u0001d\u0000\u001E\u00FF\u00E1\u0000\u001Agd\u0000\u001E\u00AC\u00D9@\u00A0/\u00F9p\u0011\u0000\u0000\u0003\u0000\u0001\u0000\u0000\u0003\u00000\u000F\u0016-\u0096\u0001\u0000\u0006h\u00EB\u00E3\u00CB\"\u00C0","jitter":200,"lastms":1541,"maxbps":15797,"trackid":1,"type":"video","width":640}}
`)

var liveTrackListPayloadNotEnoughLines = MistTriggerBody(`
	video+3cb3cu6uq1msu78k
`)

var liveTrackListPayloadInvalidJSON = MistTriggerBody(`
	video+3cb3cu6uq1msu78k
	this line is not valid JSON
`)

func TestItCanParseAValidLiveTrackListPayload(t *testing.T) {
	payload, err := ParseLiveTrackListPayload(liveTrackListPayload)
	require.NoError(t, err)
	require.Equal(t, payload.StreamName, "video+3cb3cu6uq1msu78k")
	require.Len(t, payload.TrackList, 3)

	require.Equal(t, payload.CountVideoTracks(), 2)

	// Information on video tracks
	track, ok := payload.TrackList["video_H264_256x144_30fps_2"]
	require.Equal(t, ok, true)
	require.Equal(t, track.Bframes, 1)
	require.Equal(t, track.Bps, 20155)
	require.Equal(t, track.Channels, 0)
	require.Equal(t, track.Codec, "H264")
	require.Equal(t, track.Firstms, int64(0))
	require.Equal(t, track.Fpks, 30000)
	require.Equal(t, track.Height, 144)
	require.Equal(t, track.Idx, 2)
	require.Equal(t, track.Init, "\u0001d\u0000\f\u00FF\u00E1\u0000\u0018gd\u0000\f\u00AC\u00D9A\u0001:\u0010\u0000\u0000\u0003\u0000\u0010\u0000\u0000\u0003\u0003\u00C0\u00F1B\u0099\u0060\u0001\u0000\u0005h\u00EB\u00EC\u00B2,")
	require.Equal(t, track.Lastms, int64(966))
	require.Equal(t, track.Maxbps, 32107)
	require.Equal(t, track.Rate, 0)
	require.Equal(t, track.Size, 0)
	require.Equal(t, track.Trackid, 256)
	require.Equal(t, track.Type, "video")
	require.Equal(t, track.Width, 256)

	// Just checking fields we didn't have in video
	audioTrack, ok := payload.TrackList["audio_AAC_2ch_44100hz_1"]
	require.Equal(t, ok, true)
	require.Equal(t, audioTrack.Channels, 2)
	require.Equal(t, audioTrack.Rate, 44100)
	require.Equal(t, audioTrack.Size, 16)
}

func TestItFailsToParseInvalidPayloads(t *testing.T) {
	_, err := ParseLiveTrackListPayload(liveTrackListPayloadNotEnoughLines)
	require.Error(t, err)

	_, err = ParseLiveTrackListPayload(liveTrackListPayloadInvalidJSON)
	require.Error(t, err)
}

func doLiveTrackListRequest(t *testing.T, payload MistTriggerBody, cb func(ctx context.Context, payload *LiveTrackListPayload) error) *httptest.ResponseRecorder {
	broker := NewTriggerBroker()
	broker.OnLiveTrackList(cb)
	d := NewMistCallbackHandlersCollection(&config.CLI{}, broker)
	req, err := http.NewRequest("POST", "/trigger", bytes.NewBuffer([]byte(payload)))
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	d.TriggerLiveTrackList(context.Background(), rr, req, payload)
	return rr
}

func TestItCanHandleLiveTrackListRequests(t *testing.T) {
	rr := doLiveTrackListRequest(t, liveTrackListPayload, func(ctx context.Context, prp *LiveTrackListPayload) error {
		require.Equal(t, prp.StreamName, "video+3cb3cu6uq1msu78k")
		return nil
	})
	require.Equal(t, rr.Result().StatusCode, 200)
	require.Equal(t, rr.Body.String(), "")
}

func TestItRejectsBadPayloads(t *testing.T) {
	rr := doLiveTrackListRequest(t, liveTrackListPayloadNotEnoughLines, func(ctx context.Context, prp *LiveTrackListPayload) error {
		return nil
	})
	require.Equal(t, rr.Result().StatusCode, 400)

	rr = doLiveTrackListRequest(t, liveTrackListPayloadInvalidJSON, func(ctx context.Context, prp *LiveTrackListPayload) error {
		return nil
	})
	require.Equal(t, rr.Result().StatusCode, 400)
}

func TestItHandlesFailures(t *testing.T) {
	rr := doLiveTrackListRequest(t, liveTrackListPayload, func(ctx context.Context, prp *LiveTrackListPayload) error {
		return fmt.Errorf("something went wrong")
	})
	require.Equal(t, rr.Result().StatusCode, 500)
}
