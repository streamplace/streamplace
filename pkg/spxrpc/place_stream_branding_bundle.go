package spxrpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"

	"stream.place/streamplace/pkg/branding"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
)

func (s *Server) handlePlaceStreamBrandingExportBundle(ctx context.Context, broadcaster string) (io.Reader, error) {
	if _, err := s.requireAdmin(ctx, "export branding"); err != nil {
		return nil, err
	}
	bs, err := branding.Export(ctx, s.statefulDB, s.getBroadcasterID(ctx, broadcaster))
	if err != nil {
		log.Error(ctx, "failed to export branding bundle", "err", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "unable to export branding")
	}
	if ec, ok := ctx.Value(echoContextKey).(echo.Context); ok {
		// The generated stub streams every binary output as octet-stream, but
		// the lexicon declares application/zip and the TS client rejects a
		// mismatch. echo's Stream only fills in Content-Type when unset, so
		// declaring it here wins.
		ec.Response().Header().Set(echo.HeaderContentType, "application/zip")
		ec.Response().Header().Set("Content-Disposition", `attachment; filename="branding.zip"`)
	}
	return bytes.NewReader(bs), nil
}

func (s *Server) handlePlaceStreamBrandingImportBundle(ctx context.Context, broadcaster string, dryRun bool, merge bool, r io.Reader, contentType string) (*placestream.BrandingImportBundle_Output, error) {
	author, err := s.requireAdmin(ctx, "import branding")
	if err != nil {
		return nil, err
	}
	zipBytes, err := io.ReadAll(io.LimitReader(r, branding.MaxBundleSize+1))
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "InvalidBundle: "+err.Error())
	}
	if len(zipBytes) > branding.MaxBundleSize {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("InvalidBundle: larger than %d bytes", branding.MaxBundleSize))
	}
	report, err := branding.Import(ctx, s.statefulDB, s.getBroadcasterID(ctx, broadcaster), zipBytes, merge, dryRun)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "InvalidBundle: "+err.Error())
	}
	if report.Applied {
		log.Log(ctx, "branding bundle imported", "by", author, "changes", len(report.Changes), "merge", merge)
	}
	out := &placestream.BrandingImportBundle_Output{
		Applied:  report.Applied,
		Changes:  make([]placestream.BrandingImportBundle_Change, 0, len(report.Changes)),
		Warnings: report.Warnings,
	}
	for _, c := range report.Changes {
		ch := placestream.BrandingImportBundle_Change{Key: c.Key, Action: c.Action}
		if c.Detail != "" {
			d := c.Detail
			ch.Detail = &d
		}
		out.Changes = append(out.Changes, ch)
	}
	return out, nil
}
