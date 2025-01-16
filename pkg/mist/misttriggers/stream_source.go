package misttriggers

import (
	"context"
	"fmt"
	"net/http"

	"stream.place/streamplace/pkg/errors"
	"github.com/golang/glog"
)

type StreamSourcePayload struct {
	StreamName string
}

func ParseStreamSourcePayload(payload MistTriggerBody) (StreamSourcePayload, error) {
	lines := payload.Lines()
	if len(lines) != 1 {
		return StreamSourcePayload{}, fmt.Errorf("expected 1 line in STREAM_SOURCE payload but got %d. Payload: %s", len(lines), payload)
	}

	return StreamSourcePayload{
		StreamName: lines[0],
	}, nil
}

func (d *MistCallbackHandlersCollection) TriggerStreamSource(ctx context.Context, w http.ResponseWriter, req *http.Request, body MistTriggerBody) {
	payload, err := ParseStreamSourcePayload(body)
	if err != nil {
		glog.Infof("Error parsing STREAM_SOURCE payload error=%q payload=%q", err, string(body))
		errors.WriteHTTPBadRequest(w, "Error parsing STREAM_SOURCE payload", err)
		return
	}
	resp, err := d.broker.TriggerStreamSource(ctx, &payload)
	if err != nil {
		glog.Infof("Error handling STREAM_SOURCE payload error=%q payload=%q", err, string(body))
		errors.WriteHTTPInternalServerError(w, "Error handling STREAM_SOURCE payload", err)
		return
	}
	// Flushing necessary here for Mist to handle an empty response body
	flusher := w.(http.Flusher)
	flusher.Flush()
	w.Write([]byte(resp)) // nolint:errcheck
}
