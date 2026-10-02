package rtmps

import (
	"context"
	"sync"
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
		hostname, want string
	}{
		{"stream.place", "stream.place"},
		{"STREAM.PLACE.", "stream.place"},
		{" Old.Example.Com. ", "old.example.com"},
		{"rtmp.stream.place", ""},
		{"sub.stream.place", ""},
		{"stream.place.example.com", ""},
		{"", ""},
	} {
		require.Equal(t, tc.want, h.DeprecatedHost(tc.hostname), "hostname=%q", tc.hostname)
	}

	require.Equal(t, "", NewIngestHosts(nil).DeprecatedHost("stream.place"))
	var nilHosts *IngestHosts
	require.Equal(t, "", nilHosts.DeprecatedHost("stream.place"))
}

func TestIngestHostsOpen(t *testing.T) {
	ctx := context.Background()
	h := NewIngestHosts([]string{"stream.place"})
	deprecated := spmetrics.RTMPIngestConnections.WithLabelValues("true")
	current := spmetrics.RTMPIngestConnections.WithLabelValues("false")
	baseDeprecated, baseCurrent := gaugeValue(t, deprecated), gaugeValue(t, current)

	a := sync.OnceFunc(h.Open(ctx, "stream.place", "did:plc:alice"))
	b := sync.OnceFunc(h.Open(ctx, "STREAM.PLACE.", "did:plc:alice"))
	fine := sync.OnceFunc(h.Open(ctx, "rtmp.stream.place", "did:plc:alice"))
	bob := sync.OnceFunc(h.Open(ctx, "rtmp.stream.place", "did:plc:bob"))
	t.Cleanup(a)
	t.Cleanup(b)
	t.Cleanup(fine)
	t.Cleanup(bob)

	require.Equal(t, baseDeprecated+2, gaugeValue(t, deprecated))
	require.Equal(t, baseCurrent+2, gaugeValue(t, current))
	require.Equal(t, "stream.place", h.StreamerDeprecatedHost("did:plc:alice"))
	require.Equal(t, "", h.StreamerDeprecatedHost("did:plc:bob"))

	// alice stays flagged until her last deprecated connection closes
	a()
	require.Equal(t, "stream.place", h.StreamerDeprecatedHost("did:plc:alice"))
	b()
	require.Equal(t, "", h.StreamerDeprecatedHost("did:plc:alice"))
	fine()
	bob()
	require.Equal(t, baseDeprecated, gaugeValue(t, deprecated))
	require.Equal(t, baseCurrent, gaugeValue(t, current))
}

func TestIngestHostsOverlappingDeprecatedHosts(t *testing.T) {
	ctx := context.Background()
	h := NewIngestHosts([]string{"stream.place", "old.example.com"})
	const alice = "did:plc:alice"
	const bob = "did:plc:bob"
	oldA := sync.OnceFunc(h.Open(ctx, "stream.place", alice))
	oldB := sync.OnceFunc(h.Open(ctx, "old.example.com", alice))
	other := sync.OnceFunc(h.Open(ctx, "old.example.com", bob))
	t.Cleanup(oldA)
	t.Cleanup(oldB)
	t.Cleanup(other)

	// Close whichever host the warning currently reports. Its replacement
	// must name the remaining active host, not the one that just closed.
	host := h.StreamerDeprecatedHost(alice)
	switch host {
	case "stream.place":
		oldA()
		require.Equal(t, "old.example.com", h.StreamerDeprecatedHost(alice))
		oldB()
	case "old.example.com":
		oldB()
		require.Equal(t, "stream.place", h.StreamerDeprecatedHost(alice))
		oldA()
	default:
		t.Fatalf("no active deprecated hostname reported: %q", host)
	}
	require.Equal(t, "", h.StreamerDeprecatedHost(alice))
	require.Equal(t, "old.example.com", h.StreamerDeprecatedHost(bob))
	other()
	require.Equal(t, "", h.StreamerDeprecatedHost(bob))
}
