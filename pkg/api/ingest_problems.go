package api

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"stream.place/streamplace/pkg/log"
	placestream "stream.place/streamplace/pkg/placestream"
)

// ProblemDeprecatedIngestHost is the dashboard problem for a streamer whose
// Mist publish uses a --deprecated-ingest-hosts name.
const ProblemDeprecatedIngestHost = "deprecated_ingest_host"

const deprecatedIngestHostLink = "https://stream.place/docs/guides/start-streaming/obs/#2b-stream-settings"

// ingestProblems is what the livestream websocket reports to a streamer's
// dashboard about how they're connected to this node, as a
// place.stream.ingest.defs#problems message.
func (a *StreamplaceAPI) ingestProblems(ctx context.Context, repoDID string) placestream.IngestDefs_Problems {
	out := placestream.IngestDefs_Problems{
		LexiconTypeID: "place.stream.ingest.defs#problems",
		Problems:      []placestream.IngestDefs_Problem{},
	}
	if host := a.IngestHosts.StreamerDeprecatedHost(repoDID); host != "" {
		var ingestURL string
		if a.XRPCServer != nil {
			u, err := a.XRPCServer.RTMPIngestURL(ctx)
			if err != nil {
				log.Error(ctx, "could not get RTMP ingest URL", "error", err)
			}
			ingestURL = u
		}
		fix := a.moveTo(ctx, ingestURL)
		link := deprecatedIngestHostLink
		out.Problems = append(out.Problems, placestream.IngestDefs_Problem{
			LexiconTypeID: "place.stream.ingest.defs#problem",
			Code:          ProblemDeprecatedIngestHost,
			Severity:      "warning",
			Message: fmt.Sprintf("Your encoder is streaming to %s, an old address that will stop accepting streams soon. "+
				"Change the Server in your encoder's stream settings (in OBS: Settings → Stream) %s.", host, fix),
			Link: &link,
		})
	}
	return out
}

// moveTo says where to point the encoder instead: the node's advertised RTMP
// ingest URL, unless there is none or it's itself on a deprecated host (no
// --ingests override naming the new one), which would send the streamer from
// one retired name to another.
func (a *StreamplaceAPI) moveTo(ctx context.Context, ingestURL string) string {
	if ingestURL == "" {
		return "to the RTMP server shown on your live dashboard"
	}
	u, err := url.Parse(ingestURL)
	if err == nil && a.IngestHosts.DeprecatedHost(u.Hostname()) != "" {
		log.Debug(ctx, "advertised RTMP ingest URL uses a deprecated ingest host", "url", ingestURL)
		return "to the RTMP server shown on your live dashboard"
	}
	return "to " + ingestURL
}

// problemsKey identifies a set of problems, to tell when it has changed.
func problemsKey(p placestream.IngestDefs_Problems) string {
	var b strings.Builder
	for _, prob := range p.Problems {
		b.WriteString(prob.Code)
		b.WriteByte(0)
		b.WriteString(prob.Message)
		b.WriteByte(0)
	}
	return b.String()
}
