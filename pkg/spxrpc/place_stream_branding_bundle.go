package spxrpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"

	"stream.place/streamplace/pkg/branding"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
)

// requireAdmin returns the caller's DID when there is an OAuth session for a
// node admin, or the HTTP error to answer with.
func (s *Server) requireAdmin(ctx context.Context, what string) (string, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return "", echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}
	if !s.isAdminDID(session.DID) {
		log.Warn(ctx, "unauthorized admin attempt", "did", session.DID, "what", what)
		return "", echo.NewHTTPError(http.StatusUnauthorized, "Unauthorized: not authorized to "+what)
	}
	return session.DID, nil
}

func (s *Server) handlePlaceStreamBrandingExportBundle(ctx context.Context, broadcaster string) (io.Reader, error) {
	// Whoever may change a brand may take it away with them: an admin for
	// the node's, a custom domain's owner for theirs.
	target, err := s.brandWriteTarget(ctx, broadcaster)
	if err != nil {
		return nil, err
	}
	bs, err := branding.Export(ctx, s.statefulDB, target.brandID)
	if err != nil {
		log.Error(ctx, "failed to export branding bundle", "err", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "unable to export branding")
	}
	if ec, ok := ctx.Value(echoContextKey).(echo.Context); ok {
		ec.Response().Header().Set("Content-Disposition", `attachment; filename="branding.zip"`)
	}
	return bytes.NewReader(bs), nil
}

func (s *Server) handlePlaceStreamBrandingImportBundle(ctx context.Context, broadcaster string, dryRun bool, merge bool, r io.Reader, contentType string) (*placestream.BrandingImportBundle_Output, error) {
	target, err := s.brandWriteTarget(ctx, broadcaster)
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
	var report *branding.Report
	if target.domain == nil {
		report, err = branding.Import(ctx, s.statefulDB, target.brandID, zipBytes, merge, dryRun)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "InvalidBundle: "+err.Error())
		}
	} else {
		report, err = s.importDomainBundle(ctx, target, zipBytes, merge, dryRun)
		if err != nil {
			return nil, err
		}
	}
	if report.Applied {
		log.Log(ctx, "branding bundle imported", "by", target.caller, "brand", target.brandID, "changes", len(report.Changes), "merge", merge)
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

// importDomainBundle applies a bundle to a custom domain: the result is
// published as the owner's brand record, then cached.
func (s *Server) importDomainBundle(ctx context.Context, target *brandTarget, zipBytes []byte, merge, dryRun bool) (*branding.Report, error) {
	p, err := branding.Parse(zipBytes)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "InvalidBundle: "+err.Error())
	}
	existing, err := s.domainBrandValues(ctx, target)
	if err != nil {
		return nil, err
	}
	report, _, _ := branding.Plan(existing, p.Values, merge)
	report.Warnings = p.Warnings
	if dryRun {
		return report, nil
	}
	next := branding.Values{}
	if merge {
		for k, v := range existing {
			next[k] = v
		}
	}
	for k, v := range p.Values {
		next[k] = v
	}
	if err := s.writeDomainBrand(ctx, target, next); err != nil {
		log.Error(ctx, "failed to publish custom domain brand", "err", err)
		return nil, echo.NewHTTPError(http.StatusBadGateway, "unable to publish brand record: "+err.Error())
	}
	report.Applied = true
	return report, nil
}
