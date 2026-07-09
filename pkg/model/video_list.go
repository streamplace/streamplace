package model

import (
	"context"
	"fmt"
	"time"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/api/bsky"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"stream.place/streamplace/pkg/placestream"
)

// videoListMaxScan bounds how many indexed rows GetVideoList will examine
// to fill one page when a hosting filter is applied. Without a cap, a node
// that hosts a small subset of a large index would scan the whole table for
// a single page. When the cap is hit we return what we found plus a cursor
// so the client can resume — bounded work per request, full coverage across
// pages.
const videoListMaxScan = 2000

// GetVideoList returns a page of hydrated video views, newest first. When
// repoDID is non-empty the list is scoped to that repo; when it's empty the
// list spans every indexed repo. Pagination is cursor-based: each page
// returns a cursor for the next page if more videos may exist.
//
// When hostedByServerDID is non-empty, only videos whose content blob this
// server actually hosts (i.e. has published a place.stream.media.origin for,
// under that DID) are returned — a node shouldn't advertise that it can play
// back videos it only knows about from the firehose but can't serve. The
// scan walks indexed rows in page order, skipping the ones we don't host,
// until the page is full or the table (or scan budget) is exhausted.
func (m *DBModel) GetVideoList(ctx context.Context, repoDID string, limit int, cursor string, hostedByServerDID string) (*streamplace.MediaGetVideoList_Output, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}

	// Resolve the cursor anchor once. The cursor is the uri of the last row
	// examined on the previous page; we page by (indexed_at, uri) descending.
	var anchorIndexedAt time.Time
	anchorURI := cursor
	haveAnchor := false
	if cursor != "" {
		var last Video
		if err := m.DB.WithContext(ctx).Where("uri = ?", cursor).First(&last).Error; err != nil {
			return nil, fmt.Errorf("resolve cursor: %w", err)
		}
		anchorIndexedAt = last.IndexedAt
		haveAnchor = true
	}

	videos := make([]*streamplace.MediaGetVideo_VideoView, 0, limit)
	scanned := 0
	lastExaminedURI := ""
	moreRows := true

	for len(videos) < limit && scanned < videoListMaxScan {
		// When filtering we expect to drop rows, so pull a wider batch to
		// keep the number of round trips down. Unfiltered, limit+1 reproduces
		// the original single-query behavior exactly.
		batchSize := limit + 1
		if hostedByServerDID != "" {
			batchSize = 100
		}

		query := m.DB.WithContext(ctx).
			Model(&Video{}).
			Order("indexed_at DESC, uri DESC")
		if repoDID != "" {
			query = query.Where("repo_did = ?", repoDID)
		}
		if haveAnchor {
			query = query.Where(
				"indexed_at < ? OR (indexed_at = ? AND uri < ?)",
				anchorIndexedAt, anchorIndexedAt, anchorURI,
			)
		}

		var rows []*Video
		if err := query.Limit(batchSize).Find(&rows).Error; err != nil {
			return nil, fmt.Errorf("list videos: %w", err)
		}
		if len(rows) == 0 {
			moreRows = false
			break
		}
		// A full-length batch means there may be rows beyond it; a short
		// batch means we've reached the end of the table.
		fullBatch := len(rows) == batchSize

		// One batched origin lookup per page-fill iteration: which of these
		// rows' content blobs do we host?
		var hosted map[string]bool
		if hostedByServerDID != "" {
			var err error
			hosted, err = m.hostedVideoRows(ctx, rows, hostedByServerDID)
			if err != nil {
				return nil, err
			}
		}

		brokeEarly := false
		for i, row := range rows {
			scanned++
			lastExaminedURI = row.URI
			anchorIndexedAt = row.IndexedAt
			anchorURI = row.URI
			haveAnchor = true

			if hostedByServerDID != "" && !hosted[row.URI] {
				continue
			}

			view, err := m.hydrateVideoView(ctx, row)
			if err != nil {
				return nil, fmt.Errorf("hydrate video %s: %w", row.URI, err)
			}
			videos = append(videos, view)
			if len(videos) >= limit {
				// Did we stop before consuming this batch's tail?
				brokeEarly = i < len(rows)-1
				break
			}
		}

		if len(videos) >= limit {
			// Page is full. More rows remain iff this batch had an
			// unexamined tail, or there were further batches behind it.
			moreRows = brokeEarly || fullBatch
			break
		}
		// Page not full: we examined the whole batch. If it was short,
		// that's the end of the table; otherwise fetch another batch.
		if !fullBatch {
			moreRows = false
			break
		}
	}

	out := &streamplace.MediaGetVideoList_Output{Videos: videos}
	// Hand back a cursor whenever more rows might remain — the page filled,
	// or we stopped on the scan budget. The client pages until the cursor
	// comes back nil.
	if moreRows && lastExaminedURI != "" {
		cur := lastExaminedURI
		out.Cursor = &cur
	}
	return out, nil
}

// hostedVideoRows resolves each row to its content blob CID and returns the
// set of row URIs whose blob this server hosts (has a place.stream.media.origin
// for under serverDID). A row we can't resolve to a blob is treated as not
// hosted — if we can't find the blob we certainly can't serve it.
func (m *DBModel) hostedVideoRows(ctx context.Context, rows []*Video, serverDID string) (map[string]bool, error) {
	uriToBlob := make(map[string]string, len(rows))
	blobSet := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		blob, err := m.videoContentBlob(ctx, row)
		if err != nil {
			return nil, fmt.Errorf("resolve content blob for %s: %w", row.URI, err)
		}
		if blob == "" {
			continue
		}
		uriToBlob[row.URI] = blob
		blobSet[blob] = struct{}{}
	}

	blobs := make([]string, 0, len(blobSet))
	for b := range blobSet {
		blobs = append(blobs, b)
	}
	hostedBlobs, err := m.hostedBlobs(ctx, serverDID, blobs)
	if err != nil {
		return nil, err
	}

	out := make(map[string]bool, len(uriToBlob))
	for uri, blob := range uriToBlob {
		if hostedBlobs[blob] {
			out[uri] = true
		}
	}
	return out, nil
}

// videoContentBlob resolves a video row to the BDASL CID of the primary
// content blob a player would read segments from: a sourceTracks record's
// first track's muxlTrack.blob, or — for a sourceClip — the same on its
// parent video (one level only, mirroring the playback resolver). Returns
// "" (not an error) when the record has no resolvable blob.
func (m *DBModel) videoContentBlob(ctx context.Context, row *Video) (string, error) {
	rec, err := row.ToRecord()
	if err != nil {
		return "", err
	}
	if rec.Source == nil {
		return "", nil
	}
	switch {
	case rec.Source.MediaDefs_SourceTracks != nil:
		return m.firstTrackBlobCID(ctx, rec.Source.MediaDefs_SourceTracks.Tracks)
	case rec.Source.MediaDefs_SourceClip != nil:
		clip := rec.Source.MediaDefs_SourceClip
		if clip.Video == "" {
			return "", nil
		}
		parent, err := m.GetVideoByURI(ctx, clip.Video)
		if err != nil {
			return "", err
		}
		if parent == nil || parent.Source == nil || parent.Source.MediaDefs_SourceTracks == nil {
			return "", nil
		}
		return m.firstTrackBlobCID(ctx, parent.Source.MediaDefs_SourceTracks.Tracks)
	default:
		return "", nil
	}
}

// firstTrackBlobCID returns the muxlTrack.blob CID of the first track ref in
// a sourceTracks bundle, via the local track index. Returns "" when the ref
// is empty or the track isn't indexed / isn't a muxlTrack.
func (m *DBModel) firstTrackBlobCID(ctx context.Context, tracks []*comatproto.RepoStrongRef) (string, error) {
	if len(tracks) == 0 || tracks[0] == nil || tracks[0].Uri == "" {
		return "", nil
	}
	track, err := m.GetMediaTrackByURI(ctx, tracks[0].Uri)
	if err != nil {
		return "", err
	}
	if track == nil || track.Track == nil || track.Track.MediaDefs_MuxlTrack == nil {
		return "", nil
	}
	return track.Track.MediaDefs_MuxlTrack.Blob, nil
}

// hostedBlobs returns the subset of the given blob CIDs for which serverDID
// has published a place.stream.media.origin (i.e. the node hosts the blob).
func (m *DBModel) hostedBlobs(ctx context.Context, serverDID string, blobs []string) (map[string]bool, error) {
	out := map[string]bool{}
	if serverDID == "" || len(blobs) == 0 {
		return out, nil
	}
	var found []string
	err := m.DB.WithContext(ctx).
		Model(&MediaOrigin{}).
		Where("server_did = ? AND blob IN ?", serverDID, blobs).
		Distinct().
		Pluck("blob", &found).Error
	if err != nil {
		return nil, fmt.Errorf("query hosted origins: %w", err)
	}
	for _, b := range found {
		out[b] = true
	}
	return out, nil
}

// hydrateVideoView builds a VideoView from a model row: decodes the
// record, fetches the author handle, and sums view counts.
func (m *DBModel) hydrateVideoView(ctx context.Context, row *Video) (*streamplace.MediaGetVideo_VideoView, error) {
	rec, err := row.ToRecord()
	if err != nil {
		return nil, err
	}

	author := &bsky.ActorDefs_ProfileViewBasic{Did: row.RepoDID}
	repo, err := m.GetRepo(row.RepoDID)
	if err != nil {
		return nil, fmt.Errorf("hydrate author repo: %w", err)
	}
	if repo != nil {
		author.Handle = repo.Handle
	}

	summary, err := m.viewCountSummary(ctx, row.URI)
	if err != nil {
		return nil, err
	}

	likeCount, err := m.GetLikeCount(ctx, row.URI)
	if err != nil {
		return nil, fmt.Errorf("get like count: %w", err)
	}

	return &streamplace.MediaGetVideo_VideoView{
		Uri:        row.URI,
		Cid:        row.CID,
		Author:     author,
		Record:     &lexutil.LexiconTypeDecoder{Val: rec},
		ViewCounts: summary,
		LikeCount:  likeCount,
	}, nil
}
