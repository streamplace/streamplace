package rtmps

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/pkg/spmetrics"
)

func gaugeValue(t *testing.T, g prometheus.Gauge) float64 {
	var m dto.Metric
	require.NoError(t, g.Write(&m))
	return m.GetGauge().GetValue()
}

func TestDeprecatedHost(t *testing.T) {
	h := NewIngestHosts([]string{"stream.place", " Old.Example.com. ", ""})
	for _, tc := range []struct {
		sni, tcURL, want string
	}{
		{"stream.place", "", "stream.place"},
		{"STREAM.PLACE.", "rtmps://rtmp.stream.place:1935/live", "stream.place"},
		{"", "rtmps://stream.place:1935/live", "stream.place"},
		{"rtmp.stream.place", "rtmp://stream.place/live", "stream.place"},
		{"", "rtmp://old.example.com/live", "old.example.com"},
		{"rtmp.stream.place", "rtmps://rtmp.stream.place:1935/live", ""},
		{"", "", ""},
		{"", "not a url %%", ""},
	} {
		require.Equal(t, tc.want, h.DeprecatedHost(tc.sni, tc.tcURL), "sni=%q tcUrl=%q", tc.sni, tc.tcURL)
	}

	require.Equal(t, "", NewIngestHosts(nil).DeprecatedHost("stream.place", ""))
	var nilHosts *IngestHosts
	require.Equal(t, "", nilHosts.DeprecatedHost("stream.place", ""))
}

func TestIngestHostsOpen(t *testing.T) {
	ctx := context.Background()
	h := NewIngestHosts([]string{"stream.place"})
	deprecated := spmetrics.RTMPIngestConnections.WithLabelValues(ListenerRTMP, "true")
	current := spmetrics.RTMPIngestConnections.WithLabelValues(ListenerRTMP, "false")
	baseDeprecated, baseCurrent := gaugeValue(t, deprecated), gaugeValue(t, current)

	a := h.Open(ctx, ListenerRTMP, "", "rtmp://stream.place/live", "did:plc:alice")
	b := h.Open(ctx, ListenerRTMP, "stream.place", "", "did:plc:alice")
	unresolved := h.Open(ctx, ListenerRTMP, "stream.place", "", "")
	fine := h.Open(ctx, ListenerRTMP, "", "rtmp://rtmp.stream.place/live", "did:plc:bob")

	require.Equal(t, baseDeprecated+3, gaugeValue(t, deprecated))
	require.Equal(t, baseCurrent+1, gaugeValue(t, current))
	require.Equal(t, "stream.place", h.StreamerDeprecatedHost("did:plc:alice"))
	require.Equal(t, "", h.StreamerDeprecatedHost("did:plc:bob"))

	// alice stays flagged until her last deprecated connection closes
	a()
	require.Equal(t, "stream.place", h.StreamerDeprecatedHost("did:plc:alice"))
	b()
	require.Equal(t, "", h.StreamerDeprecatedHost("did:plc:alice"))
	unresolved()
	fine()
	require.Equal(t, baseDeprecated, gaugeValue(t, deprecated))
	require.Equal(t, baseCurrent, gaugeValue(t, current))
}
