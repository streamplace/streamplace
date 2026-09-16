package spxrpc

import (
	"context"

	"github.com/bluesky-social/indigo/atproto/syntax"
	glex "github.com/streamplace/glex/runtime"

	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
)

// The replay of a livestream inherits the livestream's views. A video's
// connections to place.stream.livestream records of its own author name the
// stream(s) it is the recording of (the app's Livestreams tab and the
// operator's finalize both connect them), and every playback session those
// streams had on this station (statedb's running totals) counts toward the
// replay, beside the VOD's own aggregated views. Records of other authors
// are ignored: connecting to someone else's popular stream is not a way to
// inflate a count.

// livestreamConnections is the URIs of the video's connected livestream
// records that belong to authorDID.
func livestreamConnections(video *placestream.Video, authorDID string) []string {
	if video == nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, c := range video.Connections {
		if c.Video_Connection == nil || c.Video_Connection.Ref == nil {
			continue
		}
		uri, err := syntax.ParseATURI(c.Video_Connection.Ref.Uri)
		if err != nil || uri.Collection().String() != "place.stream.livestream" || uri.Authority().String() != authorDID {
			continue
		}
		if u := uri.String(); !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

// addLivestreamViews adds the connected livestreams' view totals to the
// view's count. Best effort: a failure leaves the count as aggregated.
func (s *Server) addLivestreamViews(ctx context.Context, view *placestream.MediaGetVideo_VideoView) {
	if s.statefulDB == nil || view == nil || view.Record == nil {
		return
	}
	video, err := glex.RecordAs[placestream.Video](view.Record.Val)
	if err != nil {
		return
	}
	uris := livestreamConnections(video, view.Author.Did)
	if len(uris) == 0 {
		return
	}
	totals, err := s.statefulDB.LivestreamViewTotals(ctx, uris)
	if err != nil {
		log.Warn(ctx, "could not add livestream views to a video", "video", view.Uri, "error", err)
		return
	}
	for _, n := range totals {
		view.ViewCounts.Count += n
	}
}
