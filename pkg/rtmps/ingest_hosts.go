package rtmps

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/spmetrics"
)

// Listener names for the RTMPIngestConnections metric.
const (
	ListenerRTMP      = "rtmp"
	ListenerRTMPS     = "rtmps"
	ListenerRTMPSMist = "rtmps_mist"
)

// IngestHosts tracks open RTMP(S) publish connections by the hostname the
// encoder was pointed at, so streamers still using a hostname we're retiring
// (--deprecated-ingest-hosts) can be warned to move. A connection counts as
// deprecated if either its TLS server name (SNI) or the host in its RTMP
// connect tcUrl is on the list: both come from the server URL typed into the
// encoder, but SNI only exists where we terminate TLS and tcUrl survives any
// TLS terminator in front of us.
type IngestHosts struct {
	deprecated map[string]bool

	mu sync.Mutex
	// open deprecated-host connections per streamer DID, and the host each
	// streamer's most recent one used
	streamers map[string]*deprecatedUse
}

type deprecatedUse struct {
	conns int
	host  string
}

// NewIngestHosts returns a tracker that treats hosts as deprecated.
func NewIngestHosts(hosts []string) *IngestHosts {
	h := &IngestHosts{
		deprecated: map[string]bool{},
		streamers:  map[string]*deprecatedUse{},
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

// DeprecatedHost returns whichever of the TLS server name and the tcUrl's
// host is a deprecated ingest host, or "" if neither is. Either may be empty.
func (h *IngestHosts) DeprecatedHost(sni, tcURL string) string {
	if h == nil || len(h.deprecated) == 0 {
		return ""
	}
	if host := normalizeHost(sni); h.deprecated[host] {
		return host
	}
	if host := tcURLHost(tcURL); h.deprecated[host] {
		return host
	}
	return ""
}

// tcURLHost is the normalized hostname of an RTMP tcUrl, or "".
func tcURLHost(tcURL string) string {
	u, err := url.Parse(tcURL)
	if err != nil {
		return ""
	}
	return normalizeHost(u.Hostname())
}

// Open records a publish connection that has reached the given listener, and
// returns a func to call exactly once when it closes. streamer may be empty when the
// connection's stream key couldn't be resolved; it is still counted.
func (h *IngestHosts) Open(ctx context.Context, listener, sni, tcURL, streamer string) (release func()) {
	host := h.DeprecatedHost(sni, tcURL)
	gauge := spmetrics.RTMPIngestConnections.WithLabelValues(listener, strconv.FormatBool(host != ""))
	gauge.Inc()
	if host == "" {
		return gauge.Dec
	}
	log.Log(ctx, "rtmp publish via deprecated ingest host",
		"listener", listener, "host", host, "sni", sni, "tcUrlHost", tcURLHost(tcURL), "streamer", streamer)
	if streamer != "" {
		h.mu.Lock()
		use := h.streamers[streamer]
		if use == nil {
			use = &deprecatedUse{}
			h.streamers[streamer] = use
		}
		use.conns++
		use.host = host
		h.mu.Unlock()
	}
	return func() {
		gauge.Dec()
		if streamer == "" {
			return
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		if use := h.streamers[streamer]; use != nil {
			use.conns--
			if use.conns <= 0 {
				delete(h.streamers, streamer)
			}
		}
	}
}

// StreamerDeprecatedHost returns the deprecated host a streamer is currently
// publishing through, or "" if they have no open connection through one.
func (h *IngestHosts) StreamerDeprecatedHost(streamer string) string {
	if h == nil {
		return ""
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if use := h.streamers[streamer]; use != nil {
		return use.host
	}
	return ""
}
