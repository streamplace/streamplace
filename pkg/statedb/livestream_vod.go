package statedb

import (
	"context"
	"strings"
	"time"

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

// livestreamUploadBackend marks the synthetic Upload rows of livestream VODs;
// their Location is the first livestream record of the recording.
const livestreamUploadBackend = "live"

// CreateLivestreamUpload creates the synthetic Upload row a livestream VOD is
// finalized into, so clients follow it with the getUploadStatus / publishVideo
// flow they already have for resumable uploads. firstURI is the first
// livestream record of the recording.
func (state *StatefulDB) CreateLivestreamUpload(ctx context.Context, uploadID, repoDID, firstURI string) error {
	return state.CreateUpload(ctx, &Upload{
		ID:       uploadID,
		RepoDID:  repoDID,
		MimeType: "video/mp4",
		Backend:  livestreamUploadBackend,
		Location: firstURI,
	})
}

// ListLivestreamUploads lists the VOD uploads whose recording starts with the
// given livestream record.
func (state *StatefulDB) ListLivestreamUploads(ctx context.Context, firstURI string) ([]Upload, error) {
	var out []Upload
	err := state.DB.WithContext(ctx).
		Where("backend = ? AND location = ?", livestreamUploadBackend, firstURI).
		Find(&out).Error
	return out, err
}

// CreateLivestreamDraft creates the draft VOD for a livestream upload, in the
// 'processing' state, inheriting the video record's title, description,
// activity, tags and connections back to the livestream records. The draft
// turns 'ready' with the upload; the streamer publishes it from the Drafts
// tab.
func (state *StatefulDB) CreateLivestreamDraft(ctx context.Context, did, uploadID string, v *VideoDraft) (*DraftVideo, error) {
	draftRec := placestream.VodDraftVideo{
		LexiconTypeID: "place.stream.vod.draftVideo",
		Title:         v.Title,
		Description:   v.Description,
		Status:        "processing",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	if v.Activity != nil {
		draftRec.Activity = &placestream.VodDraftVideo_Activity{
			Defs_ActivityGame:  v.Activity.Defs_ActivityGame,
			Defs_ActivityLabel: v.Activity.Defs_ActivityLabel,
		}
	}
	if len(v.Tags) > 0 {
		draftRec.Tags = v.Tags
	}
	// Link back to every source livestream so a published VOD carries the
	// connections (the existing UI uses this to flip a finalized row to
	// "View VOD", and the replay inherits the records' view totals).
	for _, c := range v.Connections {
		if c.Video_Connection != nil {
			draftRec.Connections = append(draftRec.Connections, placestream.VodDraftVideo_Connections_Elem{Video_Connection: c.Video_Connection})
		}
	}
	return state.CreateDraft(ctx, did, uploadID, &draftRec)
}
