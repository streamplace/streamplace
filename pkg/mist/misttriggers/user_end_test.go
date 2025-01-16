package misttriggers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"stream.place/streamplace/pkg/config"
	"github.com/stretchr/testify/require"
)

var userEndPayload = MistTriggerBody(`
	3132396757
	video+788dip9jqar876kl
	HLS
	154.47.98.190
	15
	1049826
	10173
	[UA:VLC/3.0.18 LibVLC/3.0.18]
	22
	22
	22
	6ea6ddaf2565aaee726fa802a5ecc28210b345a4a875a3c8122fe985943e6a8d
`)

var userEndPayloadBadLines = MistTriggerBody(`
	too
	few	
	lines
`)

func TestItCanParseAValidUserEndPayload(t *testing.T) {
	p, err := ParseUserEndPayload(userEndPayload, "example-uuid")
	require.NoError(t, err)
	require.Equal(t, 1, len(p.StreamNames))
	require.Equal(t, "video+788dip9jqar876kl", p.StreamNames[0])
}

func TestItCanRejectABadUserEndPayload(t *testing.T) {
	_, err := ParseUserEndPayload(userEndPayloadBadLines, "example-uuid")
	require.ErrorContains(t, err, "expected 12 lines in USER_NEW payload")
}

func doUserEndRequest(t *testing.T, payload MistTriggerBody, cb func(ctx context.Context, p *UserEndPayload) error) *httptest.ResponseRecorder {
	broker := NewTriggerBroker()
	broker.OnUserEnd(cb)
	d := NewMistCallbackHandlersCollection(&config.CLI{}, broker)
	req, err := http.NewRequest("POST", "/trigger", bytes.NewBuffer([]byte(payload)))
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	d.TriggerUserEnd(context.Background(), rr, req, payload)
	return rr
}

func TestItCanHandleUserEndRequests(t *testing.T) {
	rr := doUserEndRequest(t, userEndPayload, func(ctx context.Context, p *UserEndPayload) error {
		require.Equal(t, 1, len(p.StreamNames))
		require.Equal(t, "video+788dip9jqar876kl", p.StreamNames[0])
		return nil
	})
	require.Equal(t, rr.Result().StatusCode, 200)
}

func TestItCanErrorUserEndRequests(t *testing.T) {
	rr := doUserEndRequest(t, userEndPayloadBadLines, func(ctx context.Context, p *UserEndPayload) error {
		require.Fail(t, "test should be failing before it gets to me")
		return nil
	})
	require.Equal(t, rr.Result().StatusCode, 400)
}
