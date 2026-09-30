package api

import (
	"context"
	"fmt"
	"strings"

	"stream.place/streamplace/pkg/log"
	placestream "stream.place/streamplace/pkg/placestream"
)

// ProblemDeprecatedIngestHost is the dashboard problem for a streamer whose
// encoder reaches us through a --deprecated-ingest-hosts name.
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
		fix := "to the RTMP server shown on your live dashboard"
		if a.XRPCServer != nil {
			if u, err := a.XRPCServer.RTMPIngestURL(ctx); err != nil {
				log.Error(ctx, "could not get RTMP ingest URL", "error", err)
			} else {
				fix = "to " + u
			}
		}
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
