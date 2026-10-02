package rtmps

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/spmetrics"
)

// IngestHosts tracks authorized Mist publishes by the hostname reported in
// PUSH_REWRITE. Deprecated-host warnings last until the pull ingest ends.
type IngestHosts struct {
	deprecated map[string]bool

	mu sync.Mutex
	// Counts are per streamer and host so overlapping publishes do not keep
	// reporting a host after the last publish using it has ended.
	streamers map[deprecatedUse]int
}

type deprecatedUse struct {
	streamer string
	host     string
}

// NewIngestHosts returns a tracker that treats hosts as deprecated.
func NewIngestHosts(hosts []string) *IngestHosts {
	h := &IngestHosts{
		deprecated: map[string]bool{},
		streamers:  map[deprecatedUse]int{},
	}
	for _, host := range hosts {
		if host = normalizeHost(host); host != "" {
			h.deprecated[host] = true
		}
	}
	return h
}

func normalizeHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

// DeprecatedHost returns the normalized hostname if it is deprecated, or "".
func (h *IngestHosts) DeprecatedHost(hostname string) string {
	if h == nil || len(h.deprecated) == 0 {
		return ""
	}
	if host := normalizeHost(hostname); h.deprecated[host] {
		return host
	}
	return ""
}

// Open records an authorized Mist publish and returns a function to call
// exactly once when its pull ingest ends. streamer is the authorized DID.
func (h *IngestHosts) Open(ctx context.Context, hostname, streamer string) (release func()) {
	host := h.DeprecatedHost(hostname)
	gauge := spmetrics.RTMPIngestConnections.WithLabelValues(strconv.FormatBool(host != ""))
	if host == "" {
		gauge.Inc()
		return gauge.Dec
	}
	log.Log(ctx, "mist publish via deprecated ingest host", "host", host, "streamer", streamer)
	use := deprecatedUse{streamer: streamer, host: host}
	h.mu.Lock()
	h.streamers[use]++
	h.mu.Unlock()
	gauge.Inc()
	return func() {
		gauge.Dec()
		h.mu.Lock()
		defer h.mu.Unlock()
		h.streamers[use]--
		if h.streamers[use] == 0 {
			delete(h.streamers, use)
		}
	}
}

// StreamerDeprecatedHost returns an active deprecated host for the streamer,
// or "" if none of their active Mist publishes uses one.
func (h *IngestHosts) StreamerDeprecatedHost(streamer string) string {
	if h == nil {
		return ""
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	host := ""
	for use := range h.streamers {
		if use.streamer == streamer && (host == "" || use.host < host) {
			host = use.host
		}
	}
	return host
}
