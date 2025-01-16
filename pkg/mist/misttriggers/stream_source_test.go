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

var streamSourcePayload = MistTriggerBody(`
	video+c447r0acdmqhhhpb
`)

var streamSourcePayloadBadLines = MistTriggerBody(`
	video+c447r0acdmqhhhpb
	more-info
`)

func TestItCanParseAValidStreamSourcePayload(t *testing.T) {
	p, err := ParseStreamSourcePayload(streamSourcePayload)
	require.NoError(t, err)
	require.Equal(t, p.StreamName, "video+c447r0acdmqhhhpb")
}

func TestItCanRejectABadStreamSourcePayload(t *testing.T) {
	_, err := ParseStreamSourcePayload(streamSourcePayloadBadLines)
	require.Error(t, err)
}

func doStreamSourceRequest(t *testing.T, payload MistTriggerBody, cb func(ctx context.Context, prp *StreamSourcePayload) (string, error)) *httptest.ResponseRecorder {
	broker := NewTriggerBroker()
	broker.OnStreamSource(cb)
	d := NewMistCallbackHandlersCollection(&config.CLI{}, broker)
	req, err := http.NewRequest("POST", "/trigger", bytes.NewBuffer([]byte(payload)))
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	d.TriggerStreamSource(context.Background(), rr, req, payload)
	return rr
}

func TestStreamSourceCanHandleStreamSourceRequests(t *testing.T) {
	rr := doStreamSourceRequest(t, streamSourcePayload, func(ctx context.Context, prp *StreamSourcePayload) (string, error) {
		require.Equal(t, prp.StreamName, "video+c447r0acdmqhhhpb")
		return "dtsc://example.com/funky-stream", nil
	})
	require.Equal(t, rr.Result().StatusCode, 200)
	require.Equal(t, rr.Body.String(), "dtsc://example.com/funky-stream")
}

func TestStreamSourceCanRejectStreamSourceRequests(t *testing.T) {
	rr := doStreamSourceRequest(t, streamSourcePayload, func(ctx context.Context, prp *StreamSourcePayload) (string, error) {
		// Proper way to reject Mist
		return "", nil
	})
	require.Equal(t, rr.Result().StatusCode, 200)
	require.Equal(t, rr.Body.String(), "")
}

func TestStreamSourceCanHandleFailureToHandle(t *testing.T) {
	rr := doStreamSourceRequest(t, streamSourcePayload, func(ctx context.Context, prp *StreamSourcePayload) (string, error) {
		return "", fmt.Errorf("something went wrong")
	})
	require.Equal(t, rr.Result().StatusCode, 500)
}

func TestStreamSourceCanErrorStreamSourceRequests(t *testing.T) {
	rr := doStreamSourceRequest(t, streamSourcePayloadBadLines, func(ctx context.Context, prp *StreamSourcePayload) (string, error) {
		require.Fail(t, "test should be failing before it gets to me")
		return "", nil
	})
	require.Equal(t, rr.Result().StatusCode, 400)
}
