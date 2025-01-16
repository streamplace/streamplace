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

var userNewPayload = MistTriggerBody(`
	video+c447r0acdmqhhhpb
	127.0.0.1
	2251870645
	HLS
	http://localhost:8080/hls/video%20c447r0acdmqhhhpb/index.m3u8?stream=video%2bc447r0acdmqhhhpb
	073458ebf34cb051d3baea5f82263d0643c4d3aa425b5ac53e08cf9c1e70e7fd
`)

var userNewPayloadBadLines = MistTriggerBody(`
	video+c447r0acdmqhhhpb
`)

var userNewPayloadBadURL = MistTriggerBody(`
	video+c447r0acdmqhhhpb
	127.0.0.1
	2251870645
	HLS
	http://hostname with spaces.com
	073458ebf34cb051d3baea5f82263d0643c4d3aa425b5ac53e08cf9c1e70e7fd
`)

func TestItCanParseAValidUserNewPayload(t *testing.T) {
	p, err := ParseUserNewPayload(userNewPayload)
	require.NoError(t, err)
	require.Equal(t, p.StreamName, "video+c447r0acdmqhhhpb")
	require.Equal(t, p.Hostname, "127.0.0.1")
	require.Equal(t, p.ConnectionID, "2251870645")
	require.Equal(t, p.Protocol, "HLS")
	require.Equal(t, p.URL.Scheme, "http")
	require.Equal(t, p.URL.Host, "localhost:8080")
	require.Equal(t, p.URL.Path, "/hls/video c447r0acdmqhhhpb/index.m3u8")
	require.Equal(t, p.URL.Query().Get("stream"), "video+c447r0acdmqhhhpb")
	require.Equal(t, p.FullURL, "http://localhost:8080/hls/video%20c447r0acdmqhhhpb/index.m3u8?stream=video%2bc447r0acdmqhhhpb")
	require.Equal(t, p.SessionID, "073458ebf34cb051d3baea5f82263d0643c4d3aa425b5ac53e08cf9c1e70e7fd")
}

func TestItCanRejectABadUserNewPayload(t *testing.T) {
	_, err := ParseUserNewPayload(userNewPayloadBadLines)
	require.Error(t, err)
	_, err = ParseUserNewPayload(userNewPayloadBadURL)
	require.Error(t, err)
}

func doUserNewRequest(t *testing.T, payload MistTriggerBody, cb func(ctx context.Context, p *UserNewPayload) (bool, error)) *httptest.ResponseRecorder {
	broker := NewTriggerBroker()
	broker.OnUserNew(cb)
	d := NewMistCallbackHandlersCollection(&config.CLI{}, broker)
	req, err := http.NewRequest("POST", "/trigger", bytes.NewBuffer([]byte(payload)))
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	d.TriggerUserNew(context.Background(), rr, req, payload)
	return rr
}

func TestItCanHandleUserNewRequests(t *testing.T) {
	rr := doUserNewRequest(t, userNewPayload, func(ctx context.Context, p *UserNewPayload) (bool, error) {
		require.Equal(t, p.StreamName, "video+c447r0acdmqhhhpb")
		return true, nil
	})
	require.Equal(t, rr.Result().StatusCode, 200)
	require.Equal(t, rr.Body.String(), "true")
}

func TestItCanRejectUserNewRequests(t *testing.T) {
	rr := doUserNewRequest(t, userNewPayload, func(ctx context.Context, p *UserNewPayload) (bool, error) {
		return false, nil
	})
	require.Equal(t, rr.Result().StatusCode, 200)
	require.Equal(t, rr.Body.String(), "false")
}

func TestItCanHandleFailureToHandle(t *testing.T) {
	// should always return false on error no matter what we return
	for _, state := range []bool{true, false} {
		rr := doUserNewRequest(t, userNewPayload, func(ctx context.Context, p *UserNewPayload) (bool, error) {
			return state, fmt.Errorf("something went wrong")
		})
		require.Equal(t, rr.Result().StatusCode, 500)
		require.Equal(t, rr.Body.String(), "false")
	}
}

func TestItCanErrorUserNewRequests(t *testing.T) {
	for _, testCase := range []MistTriggerBody{userNewPayloadBadLines, userNewPayloadBadURL} {
		rr := doUserNewRequest(t, testCase, func(ctx context.Context, p *UserNewPayload) (bool, error) {
			require.Fail(t, "test should be failing before it gets to me")
			return false, nil
		})
		require.Equal(t, rr.Result().StatusCode, 400)
	}
}
