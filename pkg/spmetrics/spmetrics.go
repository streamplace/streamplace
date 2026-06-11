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

// SegmentDeliveryDuration measures wall time from when the muxer hands a
// fresh segment to the sign callback through the end of validation —
// i.e. the latency the user actually waits on before the segment is
// available downstream. Wraps per-segment signing + ValidateMP4.
var SegmentDeliveryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "streamplace_segment_delivery_duration_ms",
	Help:    "duration of sign + validate in ms (the user-visible per-segment latency)",
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

// FirehoseRelaysConnected is 1 while a relay's subscribeRepos websocket is
// connected and 0 while it is reconnecting, labeled by relay host. With
// multi-relay support this shows at a glance how many of the configured
// relays are currently feeding us.
var FirehoseRelaysConnected = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "streamplace_firehose_relays_connected",
	Help: "1 if the relay's firehose websocket is currently connected, else 0",
}, []string{"relay"})

// FirehoseEventsDedupedTotal counts events dropped because the same commit
// (or identity update) already arrived from another relay. Labeled by event
// kind ("commit" / "identity"). A high count is expected and healthy — it is
// the redundant traffic we are paying for resilience.
var FirehoseEventsDedupedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "streamplace_firehose_events_deduped_total",
	Help: "firehose events dropped as cross-relay duplicates, by kind",
}, []string{"kind"})

// --- isolated ingest workers ------------------------------------------------

// IngestWorkerStarts counts isolated ingest worker subprocesses spawned, by
// transport ("mkv-fd" = fd-4 fallback, "mkv" = detached, "whip" = detached WHIP).
var IngestWorkerStarts = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "streamplace_ingest_worker_starts_total",
	Help: "isolated ingest worker subprocesses spawned, by transport",
}, []string{"transport"})

// IngestWorkerExits counts isolated ingest worker exits by transport and outcome
// ("clean" | "crash"). A rising crash rate is the signal that a stream is
// repeatedly faulting — the contained fault the node now survives but which would
// otherwise be invisible. A worker left running across a main shutdown is not an
// exit and is not counted; "resumed" is a worker reattached after a restart.
var IngestWorkerExits = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "streamplace_ingest_worker_exits_total",
	Help: "isolated ingest worker exits, by transport and outcome (clean|crash)",
}, []string{"transport", "outcome"})

// --- VOD processing ---------------------------------------------------------

// VODProcessAttemptsTotal increments once per task dequeued for VOD
// processing, labeled by the input storage backend (file/s3). Pair with
// VODProcessSuccessesTotal / VODProcessErrorsTotal to compute success
// rate.
var VODProcessAttemptsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "streamplace_vod_process_attempts_total",
	Help: "total number of VOD processing attempts (one per dequeued task)",
}, []string{"backend"})

var VODProcessSuccessesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "streamplace_vod_process_successes_total",
	Help: "total number of VOD processing runs that produced a content-addressed object",
}, []string{"backend"})

// VODProcessErrorsTotal is labeled by the stage that failed so we can
// see whether the bulk of failures are codec issues, S3 issues, etc.
// Stage values: open_source, gstreamer_pipeline, muxl_drain,
// s3_complete, content_address_copy.
var VODProcessErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "streamplace_vod_process_errors_total",
	Help: "total number of VOD processing failures, by stage",
}, []string{"stage"})

// VODProcessDurationMS is the end-to-end wall time of one ProcessVOD
// call, from task dequeue to final CopyObject. The wide buckets cover
// realistic upload sizes (small phone clips through multi-hour
// long-form videos).
var VODProcessDurationMS = promauto.NewHistogram(prometheus.HistogramOpts{
	Name:    "streamplace_vod_process_duration_ms",
	Help:    "end-to-end wall time of one VOD processing run",
	Buckets: []float64{500, 1000, 2500, 5000, 10000, 30000, 60000, 120000, 300000, 600000, 1800000, 3600000},
})

// VODInputBytes / VODOutputBytes are observed once per successful run.
// Comparing them gives a transcode ratio (output/input) — e.g. opus->aac
// usually produces a modestly larger AAC payload, while many users will
// have inputs with surplus container overhead that gets trimmed.
var VODInputBytes = promauto.NewHistogram(prometheus.HistogramOpts{
	Name:    "streamplace_vod_input_bytes",
	Help:    "size in bytes of the user upload being processed",
	Buckets: prometheus.ExponentialBuckets(1<<20, 4, 8), // 1 MB ... 16 GB
})

var VODOutputBytes = promauto.NewHistogram(prometheus.HistogramOpts{
	Name:    "streamplace_vod_output_bytes",
	Help:    "size in bytes of the content-addressed fMP4 produced",
	Buckets: prometheus.ExponentialBuckets(1<<20, 4, 8),
})

// S3 helpers — instrumented for both the VOD pipeline and any future
// reusers of pkg/s3.ReaderAt / pkg/s3.MultipartWriter.

var S3ReaderAtOpensTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "streamplace_s3_readerat_opens_total",
	Help: "total number of ranged GetObject opens issued by pkg/s3.ReaderAt",
})

var S3ReaderAtReadsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "streamplace_s3_readerat_reads_total",
	Help: "total ReadAt calls, by whether they hit the open body (sequential) or forced a reopen",
}, []string{"kind"}) // "sequential" or "seek"

var S3MultipartPartsUploadedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "streamplace_s3_multipart_parts_uploaded_total",
	Help: "total number of parts uploaded across all multipart uploads",
})

var S3MultipartBytesUploadedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "streamplace_s3_multipart_bytes_uploaded_total",
	Help: "total bytes written via pkg/s3.MultipartWriter",
})

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
			ViewersTotal.DeleteLabelValues(protocol)
		} else {
			ViewersTotal.WithLabelValues(protocol).Set(float64(viewersByProtocol[protocol]))
		}
	}()
}

func GetViewCount(user string) int {
	viewersLock.RLock()
	defer viewersLock.RUnlock()
	return viewersByStreamer[user]
}
