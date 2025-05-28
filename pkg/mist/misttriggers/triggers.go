package misttriggers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
)

const (
	TRIGGER_PUSH_END        = "PUSH_END"        //nolint:all
	TRIGGER_PUSH_OUT_START  = "PUSH_OUT_START"  //nolint:all
	TRIGGER_PUSH_REWRITE    = "PUSH_REWRITE"    //nolint:all
	TRIGGER_STREAM_BUFFER   = "STREAM_BUFFER"   //nolint:all
	TRIGGER_LIVE_TRACK_LIST = "LIVE_TRACK_LIST" //nolint:all
	TRIGGER_USER_NEW        = "USER_NEW"        //nolint:all
	TRIGGER_USER_END        = "USER_END"        //nolint:all
	TRIGGER_STREAM_SOURCE   = "STREAM_SOURCE"   //nolint:all
)

var BlockingTriggers = map[string]bool{
	TRIGGER_PUSH_END:        false, //nolint:all
	TRIGGER_PUSH_OUT_START:  true,  //nolint:all
	TRIGGER_PUSH_REWRITE:    true,  //nolint:all
	TRIGGER_STREAM_BUFFER:   false, //nolint:all
	TRIGGER_LIVE_TRACK_LIST: false, //nolint:all
	TRIGGER_USER_NEW:        true,  //nolint:all
	TRIGGER_USER_END:        false, //nolint:all
	TRIGGER_STREAM_SOURCE:   true,  //nolint:all
}

type MistCallbackHandlersCollection struct {
	cli    *config.CLI
	broker TriggerBroker
}

func NewMistCallbackHandlersCollection(cli *config.CLI, b TriggerBroker) *MistCallbackHandlersCollection {
	return &MistCallbackHandlersCollection{cli: cli, broker: b}
}

func (d *MistCallbackHandlersCollection) Trigger() httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		payload, err := io.ReadAll(req.Body)
		if err != nil {
			errors.WriteHTTPBadRequest(w, "Cannot read trigger payload", err)
			return
		}

		var requestID = "MistTrigger-" + config.RandomTrailer(8)
		triggerName := req.Header.Get("X-Trigger")
		mistVersion := req.Header.Get("X-Version")
		if mistVersion == "" {
			mistVersion = req.UserAgent()
		}
		ctx := log.WithLogValues(context.Background(),
			"request_id", requestID,
			"trigger_name", triggerName,
			"mist_version", mistVersion,
		)

		body := MistTriggerBody(payload)

		switch triggerName {
		case TRIGGER_PUSH_OUT_START:
			d.TriggerPushOutStart(ctx, w, req, body)
		case TRIGGER_PUSH_END:
			d.TriggerPushEnd(ctx, w, req, body)
		case TRIGGER_STREAM_BUFFER:
			d.TriggerStreamBuffer(ctx, w, req, body)
		case TRIGGER_PUSH_REWRITE:
			d.TriggerPushRewrite(ctx, w, req, body)
		case TRIGGER_LIVE_TRACK_LIST:
			d.TriggerLiveTrackList(ctx, w, req, body)
		case TRIGGER_USER_NEW:
			d.TriggerUserNew(ctx, w, req, body)
		case TRIGGER_USER_END:
			d.TriggerUserEnd(ctx, w, req, body)
		case TRIGGER_STREAM_SOURCE:
			d.TriggerStreamSource(ctx, w, req, body)
		default:
			errors.WriteHTTPBadRequest(w, "Unsupported X-Trigger", fmt.Errorf("unknown trigger '%s'", triggerName))
			return
		}
	}
}

type MistTriggerBody string

func (b MistTriggerBody) Lines() []string {
	trimmed := strings.TrimSpace(string(b))
	lines := strings.Split(trimmed, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	return lines
}
