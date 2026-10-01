package statedb

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/model"
	placestream "stream.place/streamplace/pkg/placestream"
)

// LivestreamItem is one livestream record: the indexed row and its decoded
// record.
type LivestreamItem struct {
	Livestream *model.Livestream
	Record     *placestream.Livestream
}

// VideoDraftForLivestreams describes the place.stream.video record for the
// VOD of one or more livestream records: the first one's title (or the given
// one), description, tags and activity, connected to every livestream record
// the way the app's draft is connected to its one. Duration, source tracks
// and thumbnail are filled in at publish time from the finalized upload.
func VideoDraftForLivestreams(items []LivestreamItem, title, description string) *VideoDraft {
	first := items[0].Record
	title = strings.TrimSpace(title)
	if title == "" {
		title = strings.TrimSpace(first.Title)
	}
	if title == "" {
		title = "Livestream"
	}
	v := &VideoDraft{
		Title: title,
		Tags:  first.Tags,
	}
	for _, it := range items {
		v.Connections = append(v.Connections, placestream.Video_Connections_Elem{
			Video_Connection: &placestream.Video_Connection{
				LexiconTypeID: "place.stream.video#connection",
				Ref:           &comatproto.RepoStrongRef{Uri: it.Livestream.URI, Cid: it.Livestream.CID},
			},
		})
	}
	if d := strings.TrimSpace(description); d != "" {
		v.Description = &d
	}
	if first.Activity != nil {
		switch {
		case first.Activity.Defs_ActivityGame != nil:
			v.Activity = &placestream.Video_Activity{Defs_ActivityGame: first.Activity.Defs_ActivityGame}
		case first.Activity.Defs_ActivityLabel != nil:
			v.Activity = &placestream.Video_Activity{Defs_ActivityLabel: first.Activity.Defs_ActivityLabel}
		}
	}
	return v
}

// CreateLivestreamUpload creates the synthetic Upload row a livestream VOD is
// finalized into, so clients follow it with the getUploadStatus / publishVideo
// flow they already have for resumable uploads. firstURI is the first
// livestream record of the recording.
func (state *StatefulDB) CreateLivestreamUpload(ctx context.Context, repoDID, firstURI string) (string, error) {
	uu, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	uploadID := uu.String()
	if err := state.CreateUpload(ctx, &Upload{
		ID:       uploadID,
		RepoDID:  repoDID,
		MimeType: "video/mp4",
		Backend:  "live",
		Location: firstURI,
	}); err != nil {
		return "", err
	}
	return uploadID, nil
}
