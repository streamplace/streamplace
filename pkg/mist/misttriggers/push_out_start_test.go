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

var pushOutStartPayload = MistTriggerBody(`
	video+c447r0acdmqhhhpb
	rtmp://rtmp.livepeer.com/live/stream-key?video=maxbps&audio=maxbps
`)

var pushOutStartPayloadInvalidLines = MistTriggerBody(`
	video+c447r0acdmqhhhpb
`)

func TestItCanParseAValidPushOutStartPayload(t *testing.T) {
	p, err := ParsePushOutStartPayload(pushOutStartPayload)
	require.NoError(t, err)
	require.Equal(t, p.URL, "rtmp://rtmp.livepeer.com/live/stream-key?video=maxbps&audio=maxbps")
	require.Equal(t, p.StreamName, "video+c447r0acdmqhhhpb")
}

func ItCanRejectABadPushOutStartPayload(t *testing.T) {
	_, err := ParsePushOutStartPayload(pushOutStartPayloadInvalidLines)
	require.Error(t, err)
}

func doPushOutStartRequest(t *testing.T, payload MistTriggerBody, cb func(ctx context.Context, p *PushOutStartPayload) (string, error)) *httptest.ResponseRecorder {
	broker := NewTriggerBroker()
	broker.OnPushOutStart(cb)
	d := NewMistCallbackHandlersCollection(&config.CLI{}, broker)
	req, err := http.NewRequest("POST", "/trigger", bytes.NewBuffer([]byte(payload)))
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	d.TriggerPushOutStart(context.Background(), rr, req, payload)
	return rr
}

func TestItCanHandlePushOutStartRequests(t *testing.T) {
	rr := doPushOutStartRequest(t, pushOutStartPayload, func(ctx context.Context, p *PushOutStartPayload) (string, error) {
		require.Equal(t, p.URL, "rtmp://rtmp.livepeer.com/live/stream-key?video=maxbps&audio=maxbps")
		return "rtmp://example.com/live/funky-stream", nil
	})
	require.Equal(t, rr.Result().StatusCode, 200)
	require.Equal(t, rr.Body.String(), "rtmp://example.com/live/funky-stream")
}

func TestItCanRejectPushOutStartRequests(t *testing.T) {
	rr := doPushOutStartRequest(t, pushOutStartPayload, func(ctx context.Context, p *PushOutStartPayload) (string, error) {
		// Proper way to reject Mist
		return "", nil
	})
	require.Equal(t, rr.Result().StatusCode, 200)
	require.Equal(t, rr.Body.String(), "")
}

func TestPushOutStartCan500(t *testing.T) {
	rr := doPushOutStartRequest(t, pushOutStartPayload, func(ctx context.Context, p *PushOutStartPayload) (string, error) {
		return "", fmt.Errorf("something went wrong")
	})
	require.Equal(t, rr.Result().StatusCode, 500)
}

func TestItCanErrorPushOutStartRequests(t *testing.T) {
	rr := doPushOutStartRequest(t, pushOutStartPayloadInvalidLines, func(ctx context.Context, p *PushOutStartPayload) (string, error) {
		require.Fail(t, "test should be failing before it gets to me")
		return "", nil
	})
	require.Equal(t, rr.Result().StatusCode, 400)
}
