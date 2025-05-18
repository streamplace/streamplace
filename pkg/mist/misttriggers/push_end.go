package misttriggers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/golang/glog"
	"stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/mist/mistclient"
)

type PushEndPayload struct {
	PushID            int
	StreamName        string
	Destination       string
	ActualDestination string
	Last10LogLines    string
	PushStatus        *mistclient.MistPushStats
}

func ParsePushEndPayload(body MistTriggerBody) (PushEndPayload, error) {
	lines := body.Lines()
	if len(lines) != 6 {
		return PushEndPayload{}, fmt.Errorf("expected 6 lines in PUSH_END payload but got %d. Payload: %s", len(lines), body)
	}

	pushID, err := strconv.Atoi(lines[0])
	if err != nil {
		return PushEndPayload{}, fmt.Errorf("error converting pushID to number pushID=%s err=%w", lines[0], err)
	}

	stats := &mistclient.MistPushStats{}
	err = json.Unmarshal([]byte(lines[5]), stats)
	if err != nil {
		return PushEndPayload{}, fmt.Errorf("error unmarhsaling PushStatus: %w", err)
	}

	return PushEndPayload{
		PushID:            pushID,
		StreamName:        lines[1],
		Destination:       lines[2],
		ActualDestination: lines[3],
		Last10LogLines:    lines[4],
		PushStatus:        stats,
	}, nil
}

// TriggerPushEnd responds to PUSH_END trigger
// This trigger is run whenever an outgoing push stops, for any reason.
// This trigger is stream-specific and non-blocking. The payload for this trigger is multiple lines,
// each separated by a single newline character (without an ending newline), containing data:
//
//	push ID (integer)
//	stream name (string)
//	target URI, before variables/triggers affected it (string)
//	target URI, afterwards, as actually used (string)
//	last 10 log messages (JSON array string)
//	most recent push status (JSON object string)
func (d *MistCallbackHandlersCollection) TriggerPushEnd(ctx context.Context, w http.ResponseWriter, req *http.Request, body MistTriggerBody) {
	payload, err := ParsePushEndPayload(body)
	if err != nil {
		errors.WriteHTTPBadRequest(w, "Error parsing PUSH_END payload", err)
		return
	}
	err = d.broker.TriggerPushEnd(ctx, &payload)
	if err != nil {
		glog.Infof("Error handling PUSH_END payload error=%q payload=%q", err, string(body))
		errors.WriteHTTPInternalServerError(w, "Error handling PUSH_END payload", err)
		return
	}
}
