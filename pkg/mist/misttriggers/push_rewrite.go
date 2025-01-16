package misttriggers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"stream.place/streamplace/pkg/errors"
	"github.com/golang/glog"
)

type PushRewritePayload struct {
	FullURL    string
	URL        *url.URL
	Hostname   string
	StreamName string
}

func ParsePushRewritePayload(payload MistTriggerBody) (PushRewritePayload, error) {
	lines := payload.Lines()
	if len(lines) != 3 {
		return PushRewritePayload{}, fmt.Errorf("expected 3 lines in PUSH_REWRITE payload but got %d. Payload: %s", len(lines), payload)
	}

	u, err := url.Parse(lines[0])
	if err != nil {
		return PushRewritePayload{}, fmt.Errorf("unparsable URL in PUSH_REWRITE payload err=%s payload=%s", err, payload)
	}

	return PushRewritePayload{
		FullURL:    lines[0],
		URL:        u,
		Hostname:   lines[1],
		StreamName: lines[2],
	}, nil
}

func (d *MistCallbackHandlersCollection) TriggerPushRewrite(ctx context.Context, w http.ResponseWriter, req *http.Request, body MistTriggerBody) {
	payload, err := ParsePushRewritePayload(body)
	if err != nil {
		glog.Infof("Error parsing PUSH_REWRITE payload error=%q payload=%q", err, string(body))
		errors.WriteHTTPBadRequest(w, "Error parsing PUSH_REWRITE payload", err)
		return
	}
	resp, err := d.broker.TriggerPushRewrite(ctx, &payload)
	if err != nil {
		glog.Infof("Error handling PUSH_REWRITE payload error=%q payload=%q", err, string(body))
		errors.WriteHTTPInternalServerError(w, "Error handling PUSH_REWRITE payload", err)
		return
	}
	// Flushing necessary here for Mist to handle an empty response body
	flusher := w.(http.Flusher)
	flusher.Flush()
	w.Write([]byte(resp)) // nolint:errcheck
}
