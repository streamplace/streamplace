package misttriggers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/config"
)

var pushEndPayload = MistTriggerBody(`
	2596
	video+c447r0acdmqhhhpb
	rtmp://rtmp.livepeer.com/live/stream-key?video=maxbps&audio=maxbps
	rtmp://rtmp.livepeer.com/live/stream-key-2?video=minbps&audio=maxbps
	[[1688427369,"INFO","Automatically seeking to resume playback","video+c447r0acdmqhhhpb"],[1688427369,"INFO","Track selection changed - resending headers and continuing","video+c447r0acdmqhhhpb"],[1688427369,"FAIL","Opening file '/maxbps' failed: No such file or directory","video+c447r0acdmqhhhpb"],[1688427369,"INFO","Could not parse audio parameter maxbps. Skipping...","video+c447r0acdmqhhhpb"],[1688427372,"INFO","Switching UDP socket from IPv6 to IPv4","video+c447r0acdmqhhhpb"],[1688427377,"INFO","Switching UDP socket from IPv6 to IPv4","video+c447r0acdmqhhhpb"],[1688427382,"INFO","Switching UDP socket from IPv6 to IPv4","video+c447r0acdmqhhhpb"],[1688427387,"INFO","Switching UDP socket from IPv6 to IPv4","video+c447r0acdmqhhhpb"],[1688427389,"INFO","Received signal Terminated (15) from process 2550","video+c447r0acdmqhhhpb"],[1688427389,"INFO","Client handler shutting down, exit reason: signal Terminated (15) from process 2550","video+c447r0acdmqhhhpb"]]
	{"active_seconds":24,"bytes":2388218,"mediatime":26693,"tracks":[0,1]}
`)

var pushEndPayloadInvalidLines = MistTriggerBody(`
	2596
`)

var pushEndPayloadInvalidNumber = MistTriggerBody(`
	nope
	x
	x
	x
	x
	x
`)

var pushEndPayloadInvalidJSON = MistTriggerBody(`
	1234
	x
	x
	x
	x
	x
`)

var badPushEndCases = []MistTriggerBody{
	pushEndPayloadInvalidLines,
	pushEndPayloadInvalidNumber,
	pushEndPayloadInvalidJSON,
}

func TestItCanParseValidPushEndPayload(t *testing.T) {
	payload, err := ParsePushEndPayload(pushEndPayload)
	require.NoError(t, err)
	require.Equal(t, payload.StreamName, "video+c447r0acdmqhhhpb")
	require.Equal(t, payload.Destination, "rtmp://rtmp.livepeer.com/live/stream-key?video=maxbps&audio=maxbps")
	require.Equal(t, payload.ActualDestination, "rtmp://rtmp.livepeer.com/live/stream-key-2?video=minbps&audio=maxbps")
	require.Equal(t, payload.PushStatus.ActiveSeconds, int64(24))
	require.Equal(t, payload.PushStatus.Bytes, int64(2388218))
	require.Equal(t, payload.PushStatus.MediaTime, int64(26693))
	require.Equal(t, payload.PushStatus.Tracks, []int{0, 1})
}

func TestItCanRejectInvalidPushEndPayload(t *testing.T) {
	for _, testCase := range badPushEndCases {
		_, err := ParsePushEndPayload(testCase)
		require.Error(t, err)
	}
}

func doPushEndRequest(t *testing.T, payload MistTriggerBody, cb func(ctx context.Context, payload *PushEndPayload) error) *httptest.ResponseRecorder {
	broker := NewTriggerBroker()
	broker.OnPushEnd(cb)
	d := NewMistCallbackHandlersCollection(&config.CLI{}, broker)
	req, err := http.NewRequest("POST", "/trigger", bytes.NewBuffer([]byte(payload)))
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	d.TriggerPushEnd(context.Background(), rr, req, payload)
	return rr
}

func TestItCanHandlePushEndRequests(t *testing.T) {
	rr := doPushEndRequest(t, pushEndPayload, func(ctx context.Context, payload *PushEndPayload) error {
		require.Equal(t, payload.StreamName, "video+c447r0acdmqhhhpb")
		return nil
	})
	require.Equal(t, rr.Result().StatusCode, 200)
	require.Equal(t, rr.Body.String(), "")
}

func TestItRejectsBadPushEndPayloads(t *testing.T) {
	for _, testCase := range badPushEndCases {
		rr := doPushEndRequest(t, testCase, func(ctx context.Context, prp *PushEndPayload) error {
			return nil
		})
		require.Equal(t, rr.Result().StatusCode, 400)
	}
}

func TestPushEndHandlesFailures(t *testing.T) {
	rr := doPushEndRequest(t, pushEndPayload, func(ctx context.Context, prp *PushEndPayload) error {
		return fmt.Errorf("something went wrong")
	})
	require.Equal(t, rr.Result().StatusCode, 500)
}
