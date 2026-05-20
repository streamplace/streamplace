package model

import (
	"context"
	"fmt"

	"github.com/bluesky-social/indigo/api/bsky"
	lexutil "github.com/bluesky-social/indigo/lex/util"
	"gorm.io/gorm"
	"stream.place/streamplace/pkg/streamplace"
)

// GetVideoList returns a page of hydrated video views for the given
// repo DID, newest first. Pagination is cursor-based: each page
// returns a cursor for the next page if more videos exist.
func (m *DBModel) GetVideoList(ctx context.Context, repoDID string, limit int, cursor string) (*streamplace.MediaGetVideoList_Output, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}

	query := m.DB.WithContext(ctx).
		Model(&Video{}).
		Where("repo_did = ?", repoDID).
		Order("indexed_at DESC, uri DESC")

	if cursor != "" {
		// Cursor is the uri of the last video from the previous page.
		// Find its indexed_at to use as the pagination anchor.
		var last Video
		if err := m.DB.WithContext(ctx).
			Where("uri = ?", cursor).
			First(&last).Error; err != nil {
			return nil, fmt.Errorf("resolve cursor: %w", err)
		}
		query = query.Where(
			"indexed_at < ? OR (indexed_at = ? AND uri < ?)",
			last.IndexedAt, last.IndexedAt, cursor,
		)
	}

	var rows []*Video
	if err := query.Limit(limit + 1).Find(&rows).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return &streamplace.MediaGetVideoList_Output{Videos: []*streamplace.MediaGetVideo_VideoView{}}, nil
		}
		return nil, fmt.Errorf("list videos: %w", err)
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	// Hydrate each row into a VideoView.
	videos := make([]*streamplace.MediaGetVideo_VideoView, 0, len(rows))
	for _, row := range rows {
		view, err := m.hydrateVideoView(ctx, row)
		if err != nil {
			return nil, fmt.Errorf("hydrate video %s: %w", row.URI, err)
		}
		videos = append(videos, view)
	}

	out := &streamplace.MediaGetVideoList_Output{Videos: videos}
	if hasMore && len(rows) > 0 {
		lastURI := rows[len(rows)-1].URI
		out.Cursor = &lastURI
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

	return &streamplace.MediaGetVideo_VideoView{
		Uri:        row.URI,
		Cid:        row.CID,
		Author:     author,
		Record:     &lexutil.LexiconTypeDecoder{Val: rec},
		ViewCounts: summary,
	}, nil
}
