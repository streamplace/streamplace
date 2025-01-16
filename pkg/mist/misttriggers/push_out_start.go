package misttriggers

import (
	"context"
	"fmt"
	"net/http"

	"stream.place/streamplace/pkg/errors"
	"github.com/golang/glog"
)

type PushOutStartPayload struct {
	StreamName string
	URL        string
}

func ParsePushOutStartPayload(body MistTriggerBody) (PushOutStartPayload, error) {
	lines := body.Lines()
	if len(lines) != 2 {
		return PushOutStartPayload{}, fmt.Errorf("expected 2 lines in PUSH_OUT_START payload but lines=%d payload=%s", len(lines), body)
	}

	return PushOutStartPayload{
		StreamName: lines[0],
		URL:        lines[1],
	}, nil
}

// TriggerPushOutStart responds to PUSH_OUT_START trigger
// This trigger is run right before an outgoing push is started. This trigger is stream-specific and must be blocking.
// The payload for this trigger is multiple lines, each separated by a single newline character (without an ending newline), containing data:
//
// stream name (string)
// push target URI (string)
func (d *MistCallbackHandlersCollection) TriggerPushOutStart(ctx context.Context, w http.ResponseWriter, req *http.Request, body MistTriggerBody) {
	payload, err := ParsePushOutStartPayload(body)
	if err != nil {
		glog.Infof("Error parsing PUSH_OUT_START payload error=%q payload=%q", err, string(body))
		errors.WriteHTTPBadRequest(w, "Error parsing PUSH_OUT_START payload", err)
		return
	}
	resp, err := d.broker.TriggerPushOutStart(ctx, &payload)
	if err != nil {
		glog.Infof("Error handling PUSH_OUT_START payload error=%q payload=%q", err, string(body))
		errors.WriteHTTPInternalServerError(w, "Error handling PUSH_OUT_START payload", err)
		return
	}
	// Flushing necessary here for Mist to handle an empty response body
	flusher := w.(http.Flusher)
	flusher.Flush()
	w.Write([]byte(resp)) // nolint:errcheck
}
