package misttriggers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/mist/mistclient"

	"github.com/golang/glog"
)

type LiveTrackListPayload struct {
	StreamName string
	TrackList  map[string]mistclient.MistStreamInfoTrack
}

func (payload *LiveTrackListPayload) CountVideoTracks() int {
	res := 0
	for _, td := range payload.TrackList {
		if td.Type == "video" {
			res++
		}
	}
	return res
}

func ParseLiveTrackListPayload(payload MistTriggerBody) (LiveTrackListPayload, error) {
	lines := payload.Lines()
	if len(lines) != 2 {
		return LiveTrackListPayload{}, fmt.Errorf("expected 2 lines in LIVE_TRACK_LIST payload but got lines=%d payload=%s", len(lines), payload)
	}

	tl := map[string]mistclient.MistStreamInfoTrack{}
	err := json.Unmarshal([]byte(lines[1]), &tl)
	if err != nil {
		return LiveTrackListPayload{}, fmt.Errorf("error unmarhsaling LIVE_TRACK_LIST payload err=%s payload=%s", err, payload)
	}

	return LiveTrackListPayload{
		StreamName: lines[0],
		TrackList:  tl,
	}, nil
}

func (d *MistCallbackHandlersCollection) TriggerLiveTrackList(ctx context.Context, w http.ResponseWriter, req *http.Request, payload MistTriggerBody) {
	body, err := ParseLiveTrackListPayload(payload)
	if err != nil {
		log.Log(ctx, "Error parsing LIVE_TRACK_LIST payload", "error", err, "payload", payload)
		errors.WriteHTTPBadRequest(w, "Error parsing LIVE_TRACK_LIST payload", err)
		return
	}
	err = d.broker.TriggerLiveTrackList(ctx, &body)
	if err != nil {
		glog.Infof("Error handling LIVE_TRACK_LIST payload", "error", err, "payload", payload)
		errors.WriteHTTPInternalServerError(w, "Error handling LIVE_TRACK_LIST payload", err)
		return
	}
}
