package mistconfig

import (
	"crypto/md5"
	"encoding/json"
	"fmt"

	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/mist/misttriggers"
)

var StreamName = "stream"

// LiveSegmentTargetMS keeps the source GOP target below the first-RTP budget.
// Mist may extend a segment to a complete keyframe, so the upstream encoder's
// keyframe interval must be no larger than this target as well.
const LiveSegmentTargetMS = 500

func Generate(cli *config.CLI) ([]byte, error) {
	triggers := map[string][]map[string]any{}
	for name, blocking := range misttriggers.BlockingTriggers {
		triggers[name] = []map[string]any{{
			"handler": fmt.Sprintf("%s/mist-trigger", cli.OwnInternalURL()),
			"streams": []string{},
			"sync":    blocking,
		}}
	}
	conf := map[string]any{
		"account": map[string]any{
			// doesn't need to be secure, will only ever be exposed on localhost
			"streamplace": map[string]any{
				"password": md5.Sum([]byte("streamplace")),
			},
		},
		"bandwidth": map[string]any{
			"exceptions": []string{
				"::1",
				"127.0.0.0/8",
				"10.0.0.0/8",
				"192.168.0.0/16",
				"172.16.0.0/12",
			},
		},
		"config": map[string]any{
			"accesslog":  "LOG",
			"prometheus": "streamplace",
			"protocols": []map[string]any{
				{"connector": "AAC"},
				{"connector": "CMAF"},
				{"connector": "EBML"},
				{"connector": "FLAC"},
				{"connector": "FLV"},
				{"connector": "H264"},
				{"connector": "HDS"},
				{"connector": "HLS"},
				{"connector": "HTTPTS"},
				{"connector": "JSON"},
				{"connector": "MP3"},
				{"connector": "MP4"},
				{"connector": "OGG"},
				{"connector": "WAV"},
				{
					"connector": "RTMP",
					"interface": "127.0.0.1",
					"port":      cli.MistRTMPPort,
				},
				{
					"connector": "HTTP",
					"interface": "127.0.0.1",
					"port":      cli.MistHTTPPort,
				},
				{
					"connector":     "WebRTC",
					"jitterlog":     false,
					"mergesessions": false,
					"nackdisable":   false,
					"packetlog":     false,
				},
			},
			"sessionInputMode":       15,
			"sessionOutputMode":      15,
			"sessionStreamInfoMode":  1,
			"sessionUnspecifiedMode": 0,
			"sessionViewerMode":      14,
			"tknMode":                15,
			"triggers":               triggers,
			"trustedproxy":           []string{},
		},
		"streams": map[string]map[string]any{
			StreamName: {
				"name":          StreamName,
				"segmentsize":   LiveSegmentTargetMS,
				"source":        "push://",
				"stop_sessions": false,
			},
		},
		"ui_settings": map[string]any{
			"HTTPUrl": "http://localhost:8082/",
		},
	}
	return json.Marshal(conf)
}
