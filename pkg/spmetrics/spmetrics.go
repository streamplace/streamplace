package spmetrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var viewersByStreamer = map[string]int{}
var viewersByProtocol = map[string]int{}
var viewersLock sync.RWMutex

var Viewers = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "streamplace_viewers",
	Help: "number of current viewers per user",
}, []string{"streamer"})

var ViewersTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "streamplace_viewers_total",
	Help: "total number of viewers",
}, []string{"protocol"})

var StreamSessions = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "streamplace_stream_sessions",
	Help: "number of open stream sessions per streamer",
}, []string{"streamer"})

var SendSegmentCalls = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "streamplace_send_segment_calls",
	Help: "total number of send segment calls currently in flight",
})

var SwarmPutCalls = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "streamplace_swarm_put_calls",
	Help: "total number of swarm put calls currently in flight",
}, []string{"streamer"})

var TranscodeAttemptsTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "streamplace_transcode_attempts_total",
	Help: "total number of transcode attempts",
})

var TranscodeSuccessesTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "streamplace_transcode_successes_total",
	Help: "total number of transcode successes",
})

var TranscodeErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "streamplace_transcode_errors_total",
	Help: "total number of transcode errors",
})

var TranscodeDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "streamplace_transcode_duration_ms",
	Help:    "duration of transcode in ms",
	Buckets: []float64{0, 250, 500, 750, 1000, 1250, 1500, 2000, 2500, 3000, 3500, 4000, 4500, 5000, 10000},
}, []string{"streamer"})

var SigningDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "streamplace_signing_duration_ms",
	Help:    "duration of transcode in ms",
	Buckets: []float64{0, 250, 500, 750, 1000, 1250, 1500, 2000, 2500, 3000, 3500, 4000, 4500, 5000, 10000, 20000, 30000, 60000},
}, []string{"streamer"})

var QueuedTranscodeDuration = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "streamplace_queued_transcode_duration_ms",
	Help: "duration of transcode in ms, including time spent waiting",
}, []string{"streamer"})

var Version = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "streamplace_version",
	Help: "version of streamplace",
}, []string{"version"})

var WebsocketsOpen = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "streamplace_websockets_open",
	Help: "number of open playback websockets",
})

var ReplicationWebsocketsOpen = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "streamplace_replication_websockets_open",
	Help: "number of open replication websockets",
})

var SegmentSubscriptionsOpen = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "streamplace_segment_subscriptions_open",
	Help: "number of open new segment subscriptions",
}, []string{"streamer", "rendition"})

var LabelerFirehosesConnected = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "streamplace_labeler_firehoses_connected",
	Help: "number of currently connected labeler firehoses",
}, []string{"labeler"})

func ViewerInc(user string, protocol string) {
	go func() {
		viewersLock.Lock()
		defer viewersLock.Unlock()
		viewersByStreamer[user]++
		viewersByProtocol[protocol]++
		Viewers.WithLabelValues(user).Set(float64(viewersByStreamer[user]))
		ViewersTotal.WithLabelValues(protocol).Set(float64(viewersByProtocol[protocol]))
	}()
}

func ViewerDec(user string, protocol string) {
	go func() {
		viewersLock.Lock()
		defer viewersLock.Unlock()
		viewersByStreamer[user]--
		if viewersByStreamer[user] == 0 {
			Viewers.DeleteLabelValues(user)
		} else {
			Viewers.WithLabelValues(user).Set(float64(viewersByStreamer[user]))
		}
		viewersByProtocol[protocol]--
		if viewersByProtocol[protocol] == 0 {
			Viewers.DeleteLabelValues(protocol)
		} else {
			Viewers.WithLabelValues(protocol).Set(float64(viewersByProtocol[protocol]))
		}
	}()
}

func GetViewCount(user string) int {
	viewersLock.RLock()
	defer viewersLock.RUnlock()
	return viewersByStreamer[user]
}
