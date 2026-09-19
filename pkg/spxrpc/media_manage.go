package spxrpc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
	"stream.place/streamplace/pkg/vod"
)

// A streamer's recordings and video records, through XRPC, for the streamer
// and for the moderators they have granted livestream.manage: finalize a
// recording into a VOD (place.stream.media.finalizeLivestream), publish a
// record for a finished upload (publishVideo), list the records and this
// node's uploads (listVideos), retitle or repair a record (updateVideo),
// delete one (deleteVideo). Every write goes through the streamer's stored
// session, the way delegated moderation does, so nothing here can act for a
// streamer who never signed in to this node.

// requireLivestreamManage is the authorization every handler here shares:
// the caller must be streamer, or hold livestream.manage from them; the
// streamer must have a stored session for the writes.
func (s *Server) requireLivestreamManage(ctx context.Context, streamer, action string) (*DelegatedModerationContext, error) {
	modCtx, err := s.GetDelegatedModerationContext(ctx, streamer, action)
	if err != nil {
		var he *echo.HTTPError
		if errors.As(err, &he) && he.Code == http.StatusForbidden {
			return nil, echo.NewHTTPError(http.StatusForbidden, "NotPermitted: the caller is neither the streamer nor a moderator with livestream.manage from them")
		}
		return nil, err
	}
	return modCtx, nil
}

// livestreamItem is one livestream record: the indexed row and its decoded
// record.
type livestreamItem struct {
	ls  *model.Livestream
	rec *placestream.Livestream
}

// livestreamItems resolves livestream URIs to their indexed rows and decoded
// records, checks they belong to one streamer, and orders them by creation:
// recording order is record order, whatever order the caller listed them in.
func (s *Server) livestreamItems(uris []string) ([]livestreamItem, error) {
	items := make([]livestreamItem, 0, len(uris))
	for _, u := range uris {
		ls, err := s.model.GetLivestream(u)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "get livestream "+u+": "+err.Error())
		}
		if ls == nil {
			return nil, echo.NewHTTPError(http.StatusNotFound, "LivestreamNotFound: not indexed on this node: "+u)
		}
		view, err := ls.ToLivestreamView()
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "decode livestream "+u+": "+err.Error())
		}
		rec, ok := view.Record.Val.(*placestream.Livestream)
		if !ok {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "record is not a place.stream.livestream: "+u)
		}
		items = append(items, livestreamItem{ls: ls, rec: rec})
	}
	for _, it := range items[1:] {
		if it.ls.RepoDID != items[0].ls.RepoDID {
			return nil, echo.NewHTTPError(http.StatusNotFound, "LivestreamNotFound: the livestreams belong to different streamers")
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].rec.CreatedAt < items[j].rec.CreatedAt })
	return items, nil
}

// uniqueURIs merges the single and plural forms of a livestream argument.
func uniqueURIs(one *string, many []string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(u string) {
		u = strings.TrimSpace(u)
		if u != "" && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	if one != nil {
		add(*one)
	}
	for _, u := range many {
		add(u)
	}
	return out
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// videoDraftForLivestreams describes the place.stream.video record for the
// VOD of one or more livestream records: the first one's title (or the given
// one), description, tags and activity, connected to every livestream record
// the way the app's draft is connected to its one. Duration, source tracks
// and thumbnail are filled in at publish time from the finalized upload.
func videoDraftForLivestreams(items []livestreamItem, title, description string) *statedb.VideoDraft {
	first := items[0].rec
	title = strings.TrimSpace(title)
	if title == "" {
		title = strings.TrimSpace(first.Title)
	}
	if title == "" {
		title = "Livestream"
	}
	v := &statedb.VideoDraft{
		Title: title,
		Tags:  first.Tags,
	}
	for _, it := range items {
		v.Connections = append(v.Connections, placestream.Video_Connections_Elem{
			Video_Connection: &placestream.Video_Connection{
				LexiconTypeID: "place.stream.video#connection",
				Ref:           &comatproto.RepoStrongRef{Uri: it.ls.URI, Cid: it.ls.CID},
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

// videoOwner is the repo a place.stream.video URI lives in.
func videoOwner(uri string) (string, error) {
	aturi, err := syntax.ParseATURI(uri)
	if err != nil || aturi.Collection().String() != "place.stream.video" {
		return "", echo.NewHTTPError(http.StatusBadRequest, "uri must be a place.stream.video record")
	}
	return aturi.Authority().String(), nil
}

// videoError maps the vod package's failures onto XRPC errors.
func videoError(what string, err error) error {
	switch {
	case errors.Is(err, vod.ErrNotAVideo):
		return echo.NewHTTPError(http.StatusBadRequest, what+": "+err.Error())
	case errors.Is(err, vod.ErrUploadNotFound):
		return echo.NewHTTPError(http.StatusNotFound, what+": no such upload for this streamer")
	case errors.Is(err, vod.ErrUploadNotReady):
		return echo.NewHTTPError(http.StatusConflict, what+": upload is not finished processing")
	case strings.Contains(strings.ToLower(err.Error()), "could not locate record") || strings.Contains(strings.ToLower(err.Error()), "record not found"):
		return echo.NewHTTPError(http.StatusNotFound, "VideoNotFound: "+err.Error())
	}
	return echo.NewHTTPError(http.StatusInternalServerError, what+": "+err.Error())
}

// handlePlaceStreamMediaListVideos lists a streamer's video records and this
// node's uploads for them.
func (s *Server) handlePlaceStreamMediaListVideos(ctx context.Context, repo string) (*placestream.MediaListVideos_Output, error) {
	did := strings.TrimSpace(repo)
	if _, err := syntax.ParseDID(did); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "repo must be a DID")
	}
	if _, err := s.requireLivestreamManage(ctx, did, "listVideos"); err != nil {
		return nil, err
	}
	recs, err := vod.ListVideoRecords(ctx, s.statefulDB, did)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "list video records: "+err.Error())
	}
	uploads, err := s.statefulDB.ListUploadsForRepo(ctx, did)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "list uploads: "+err.Error())
	}
	out := &placestream.MediaListVideos_Output{RepoDID: did, Videos: []placestream.MediaListVideos_Video{}, Uploads: []placestream.MediaListVideos_Upload{}}
	for _, rec := range recs {
		item := placestream.MediaListVideos_Video{Uri: rec.URI, Cid: rec.CID, Tracks: rec.TrackURIs()}
		if item.Tracks == nil {
			item.Tracks = []string{}
		}
		if rec.Video != nil {
			item.Title = rec.Video.Title
			d := rec.Video.DurationMs
			item.DurationMs = &d
			created := rec.Video.CreatedAt
			item.CreatedAt = &created
			thumb := rec.Video.Thumb != nil
			item.Thumb = &thumb
			for _, c := range rec.Video.Connections {
				if c.Video_Connection != nil && c.Video_Connection.Ref != nil {
					item.Connections = append(item.Connections, c.Video_Connection.Ref.Uri)
				}
			}
		}
		out.Videos = append(out.Videos, item)
	}
	for _, u := range uploads {
		item := placestream.MediaListVideos_Upload{UploadId: u.ID, Status: u.ProcessingStatus, Backend: u.Backend}
		if u.ProcessingError != "" {
			e := u.ProcessingError
			item.Error = &e
		}
		if u.Location != "" {
			l := u.Location
			item.Location = &l
		}
		if u.ContentCID != "" {
			c := u.ContentCID
			item.ContentCid = &c
		}
		dur, size := u.DurationMS, u.BlobSize
		item.DurationMs, item.BlobSize = &dur, &size
		created := u.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z")
		item.CreatedAt = &created
		if u.TrackURIs != "" {
			var refs []struct{ URI string }
			if json.Unmarshal([]byte(u.TrackURIs), &refs) == nil {
				n := int64(len(refs))
				item.Tracks = &n
			}
		}
		out.Uploads = append(out.Uploads, item)
	}
	return out, nil
}

// handlePlaceStreamMediaUpdateVideo rewrites a video record in place.
func (s *Server) handlePlaceStreamMediaUpdateVideo(ctx context.Context, body *placestream.MediaUpdateVideo_Input) (*placestream.MediaUpdateVideo_Output, error) {
	owner, err := videoOwner(body.Uri)
	if err != nil {
		return nil, err
	}
	if _, err := s.requireLivestreamManage(ctx, owner, "updateVideo"); err != nil {
		return nil, err
	}
	if s.playbackStore == nil {
		return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "playback store not configured")
	}
	cid, err := vod.UpdateVideo(ctx, s.statefulDB, s.playbackStore, body.Uri, vod.VideoUpdate{
		Title: body.Title, Description: body.Description, Tags: body.Tags, UploadID: deref(body.UploadId),
	})
	if err != nil {
		return nil, videoError("update video", err)
	}
	log.Log(ctx, "video record updated", "uri", body.Uri, "cid", cid)
	return &placestream.MediaUpdateVideo_Output{Uri: body.Uri, Cid: cid}, nil
}

// handlePlaceStreamMediaDeleteVideo deletes a video record and, on request,
// its track records.
func (s *Server) handlePlaceStreamMediaDeleteVideo(ctx context.Context, body *placestream.MediaDeleteVideo_Input) (*placestream.MediaDeleteVideo_Output, error) {
	owner, err := videoOwner(body.Uri)
	if err != nil {
		return nil, err
	}
	if _, err := s.requireLivestreamManage(ctx, owner, "deleteVideo"); err != nil {
		return nil, err
	}
	deleted, err := vod.DeleteVideo(ctx, s.statefulDB, body.Uri, body.Tracks != nil && *body.Tracks)
	if err != nil {
		return nil, videoError("delete video", err)
	}
	log.Log(ctx, "video record deleted", "uri", body.Uri, "deleted", deleted)
	if deleted == nil {
		deleted = []string{}
	}
	return &placestream.MediaDeleteVideo_Output{Deleted: deleted}, nil
}
