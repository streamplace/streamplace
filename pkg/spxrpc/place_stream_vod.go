package spxrpc

import (
	"context"
	"net/http"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/labstack/echo/v4"

	placestream "stream.place/streamplace/pkg/placestream"
)

func (s *Server) handlePlaceStreamVodGetComments(ctx context.Context, cursor string, limit int, video string) (*placestream.VodGetComments_Output, error) {
	if video == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "video is required")
	}
	if _, err := syntax.ParseATURI(video); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "video must be a valid AT-URI: "+err.Error())
	}
	l := 50
	if limit > 0 && limit <= 100 {
		l = limit
	}
	var c *time.Time
	if cursor != "" {
		t, err := time.Parse(time.RFC3339Nano, cursor)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid cursor: "+err.Error())
		}
		c = &t
	}
	comments, nextCursor, err := s.model.GetCommentsForVideo(ctx, video, l, c)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	out := &placestream.VodGetComments_Output{
		Comments: comments,
	}
	if nextCursor != nil {
		cs := nextCursor.Format(time.RFC3339Nano)
		out.Cursor = &cs
	}
	return out, nil
}

func (s *Server) handlePlaceStreamGetLikes(ctx context.Context, cursor string, limit int, subject string) (*placestream.GetLikes_Output, error) {
	if subject == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "subject is required")
	}
	if _, err := syntax.ParseATURI(subject); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "subject must be a valid AT-URI: "+err.Error())
	}
	l := 50
	if limit > 0 && limit <= 100 {
		l = limit
	}
	var c *time.Time
	if cursor != "" {
		t, err := time.Parse(time.RFC3339Nano, cursor)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid cursor: "+err.Error())
		}
		c = &t
	}
	likes, count, nextCursor, err := s.model.GetLikesForSubject(ctx, subject, l, c)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	out := &placestream.GetLikes_Output{
		Subject: subject,
		Count:   count,
		Likes:   likes,
	}
	if nextCursor != nil {
		cs := nextCursor.Format(time.RFC3339Nano)
		out.Cursor = &cs
	}
	return out, nil
}
