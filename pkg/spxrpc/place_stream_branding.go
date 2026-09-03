package spxrpc

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"gorm.io/gorm"
	"stream.place/streamplace/js/app"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
)

var defaultBrandingAssets = map[string]struct {
	data []byte
	mime string
}{
	// "mainLogo":         {data: defaultLogoSVG, mime: "image/svg+xml"},
	// "favicon":          {data: defaultFaviconSVG, mime: "image/svg+xml"},
	"siteTitle":       {data: []byte(""), mime: "text/plain"},
	"siteDescription": {data: []byte(""), mime: "text/plain"},
	"primaryColor":    {data: []byte("#6366f1"), mime: "text/plain"},
	"accentColor":     {data: []byte("#8b5cf6"), mime: "text/plain"},
	"defaultStreamer": {data: []byte(""), mime: "text/plain"},
	// Chrome colors: the app derives its surface, text and border ramps from
	// one background + one foreground per color scheme (see
	// js/components/src/lib/theme/chrome.ts). Empty means the app's defaults.
	"backgroundColor":      {data: []byte(""), mime: "text/plain"},
	"foregroundColor":      {data: []byte(""), mime: "text/plain"},
	"backgroundColorLight": {data: []byte(""), mime: "text/plain"},
	"foregroundColorLight": {data: []byte(""), mime: "text/plain"},
}

// brandingTextKeys are the small text-valued assets (1KB cap).
var brandingTextKeys = map[string]bool{
	"siteTitle": true, "siteDescription": true, "primaryColor": true, "accentColor": true,
	"defaultStreamer": true, "backgroundColor": true, "foregroundColor": true,
	"backgroundColorLight": true, "foregroundColorLight": true,
}

// NormalizeBroadcasterID turns the optional `broadcaster` parameter into the
// key branding is stored under. Assets are written under the broadcaster's
// DID (did:web:<host>), which is what clients send; an empty parameter means
// this node's own broadcaster, and a bare host is upgraded to its did:web.
func NormalizeBroadcasterID(param, defaultHost string) string {
	param = strings.TrimSpace(param)
	if param == "" {
		return "did:web:" + defaultHost
	}
	if strings.HasPrefix(param, "did:") {
		return param
	}
	return "did:web:" + param
}

func (s *Server) getBroadcasterID(ctx context.Context, broadcasterDID string) string {
	return NormalizeBroadcasterID(broadcasterDID, s.cli.BroadcasterHost)
}

func (s *Server) GetBrandingBlob(ctx context.Context, broadcasterID, key string) ([]byte, string, *int, *int, error) {
	// cache miss - fetch from db
	blob, err := s.statefulDB.GetBrandingBlob(broadcasterID, key)
	if err == gorm.ErrRecordNotFound {
		// Older nodes stored unparameterised writes under the bare host.
		if host, ok := strings.CutPrefix(broadcasterID, "did:web:"); ok {
			if legacy, lerr := s.statefulDB.GetBrandingBlob(host, key); lerr == nil {
				return legacy.Data, legacy.MimeType, legacy.Width, legacy.Height, nil
			}
		}
		// not in db, use default
		if def, ok := defaultBrandingAssets[key]; ok {
			return def.data, def.mime, nil, nil, nil
		}
		return nil, "", nil, nil, fmt.Errorf("unknown branding key: %s", key)
	}
	if err != nil {
		return nil, "", nil, nil, fmt.Errorf("error fetching branding blob: %w", err)
	}
	return blob.Data, blob.MimeType, blob.Width, blob.Height, nil
}

func (s *Server) handlePlaceStreamBrandingGetBlob(ctx context.Context, broadcasterDID string, key string) (io.Reader, error) {
	return s.HandlePlaceStreamBrandingGetBlobDirect(ctx, broadcasterDID, key)
}

// HandlePlaceStreamBrandingGetBlobDirect is the exported version for direct calls
func (s *Server) HandlePlaceStreamBrandingGetBlobDirect(ctx context.Context, broadcasterDID string, key string) (io.Reader, error) {
	broadcasterID := s.getBroadcasterID(ctx, broadcasterDID)
	data, _, _, _, err := s.GetBrandingBlob(ctx, broadcasterID, key)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (s *Server) handlePlaceStreamBrandingGetBranding(ctx context.Context, broadcasterDID string) (*placestream.BrandingGetBranding_Output, error) {
	return s.HandlePlaceStreamBrandingGetBrandingDirect(ctx, broadcasterDID)
}

// HandlePlaceStreamBrandingGetBrandingDirect is the exported version for direct calls
func (s *Server) HandlePlaceStreamBrandingGetBrandingDirect(ctx context.Context, broadcasterDID string) (*placestream.BrandingGetBranding_Output, error) {
	broadcasterID := s.getBroadcasterID(ctx, broadcasterDID)

	// get all keys from database
	dbKeys, err := s.statefulDB.ListBrandingKeys(broadcasterID)
	if err != nil {
		return nil, fmt.Errorf("error listing branding keys: %w", err)
	}
	if host, ok := strings.CutPrefix(broadcasterID, "did:web:"); ok {
		legacyKeys, err := s.statefulDB.ListBrandingKeys(host)
		if err != nil {
			return nil, fmt.Errorf("error listing legacy branding keys: %w", err)
		}
		dbKeys = append(dbKeys, legacyKeys...)
	}

	// build key set including defaults
	allKeys := make(map[string]bool)
	for _, key := range dbKeys {
		allKeys[key] = true
	}
	for key := range defaultBrandingAssets {
		allKeys[key] = true
	}

	// build output
	assets := make([]placestream.BrandingGetBranding_BrandingAsset, 0, len(allKeys))
	for key := range allKeys {
		data, mimeType, width, height, err := s.GetBrandingBlob(ctx, broadcasterID, key)
		if err != nil {
			continue // skip if error
		}

		asset := &placestream.BrandingGetBranding_BrandingAsset{
			Key:      key,
			MimeType: mimeType,
		}

		// add dimensions if available
		if width != nil {
			w := int64(*width)
			asset.Width = &w
		}
		if height != nil {
			h := int64(*height)
			asset.Height = &h
		}

		// for text assets, include data inline; for images, provide URL
		if mimeType == "text/plain" {
			str := string(data)
			asset.Data = &str
		} else {
			url := fmt.Sprintf("/xrpc/place.stream.branding.getBlob?key=%s&broadcaster=%s", key, broadcasterID)
			asset.Url = &url
		}

		assets = append(assets, *asset)
	}

	return &placestream.BrandingGetBranding_Output{
		Assets: assets,
	}, nil
}

func (s *Server) isAdminDID(did string) bool {
	return s.cli.IsAdmin(did)
}

func (s *Server) handlePlaceStreamBrandingUpdateBlob(ctx context.Context, input *placestream.BrandingUpdateBlob_Input) (*placestream.BrandingUpdateBlob_Output, error) {
	// check authentication
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	// check admin authorization
	if !s.isAdminDID(session.DID) {
		log.Warn(ctx, "unauthorized branding update attempt", "did", session.DID)
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "not authorized to modify branding")
	}

	var broadcasterDID string
	if input.Broadcaster != nil {
		broadcasterDID = *input.Broadcaster
	}
	broadcasterID := s.getBroadcasterID(ctx, broadcasterDID)

	// decode base64 data
	data, err := base64.StdEncoding.DecodeString(input.Data)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid base64 data")
	}

	// validate size based on key type
	maxSize := 500 * 1024 // 500KB default for logos
	if input.Key == "favicon" {
		maxSize = 100 * 1024 // 100KB for favicons
	} else if brandingTextKeys[input.Key] {
		maxSize = 1024 // 1KB for text values
	}
	// sidebarBackgroundImage uses default 500KB limit
	if len(data) > maxSize {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("blob too large (max %d bytes)", maxSize))
	}

	// store in database
	var width, height *int
	if input.Width != nil {
		w := int(*input.Width)
		width = &w
	}
	if input.Height != nil {
		h := int(*input.Height)
		height = &h
	}

	err = s.statefulDB.PutBrandingBlob(broadcasterID, input.Key, input.MimeType, data, width, height)
	if err != nil {
		log.Error(ctx, "failed to store branding blob", "err", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "unable to store branding blob")
	}

	return &placestream.BrandingUpdateBlob_Output{
		Success: true,
	}, nil
}

func (s *Server) handlePlaceStreamBrandingDeleteBlob(ctx context.Context, input *placestream.BrandingDeleteBlob_Input) (*placestream.BrandingDeleteBlob_Output, error) {
	// check authentication
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}

	// check admin authorization
	if !s.isAdminDID(session.DID) {
		log.Warn(ctx, "unauthorized branding delete attempt", "did", session.DID)
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "not authorized to modify branding")
	}

	var broadcasterDID string
	if input.Broadcaster != nil {
		broadcasterDID = *input.Broadcaster
	}
	broadcasterID := s.getBroadcasterID(ctx, broadcasterDID)

	err := s.statefulDB.DeleteBrandingBlob(broadcasterID, input.Key)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "branding asset not found")
		}
		log.Error(ctx, "failed to delete branding blob", "err", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "unable to delete branding blob")
	}

	return &placestream.BrandingDeleteBlob_Output{
		Success: true,
	}, nil
}

// HandleFaviconICO serves the favicon at /favicon.ico
func (s *Server) HandleFaviconICO(c echo.Context) error {
	ctx := c.Request().Context()

	broadcasterID := s.cli.BroadcasterHost
	log.Log(ctx, "fetching favicon", "broadcasterID", broadcasterID)
	data, mimeType, _, _, err := s.GetBrandingBlob(ctx, "did:web:"+broadcasterID, "favicon")

	if err != nil || data == nil {
		log.Log(ctx, "using fallback favicon", "err", err, "data_nil", data == nil)
		distFiles, fsErr := app.Files()
		if fsErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch favicon")
		}

		faviconFile, fsErr := distFiles.Open("favicon.ico")
		if fsErr != nil {
			return echo.NewHTTPError(http.StatusNotFound, "favicon not found")
		}
		defer faviconFile.Close()

		data, fsErr = io.ReadAll(faviconFile)
		if fsErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to read favicon")
		}

		// detect mime type based on file extension (ico)
		mimeType = "image/x-icon"
	}

	return c.Blob(http.StatusOK, mimeType, data)
}
