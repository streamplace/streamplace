package spxrpc

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/streamplace/oatproxy/pkg/oatproxy"
	"gorm.io/gorm"
	"stream.place/streamplace/js/app"
	"stream.place/streamplace/pkg/branding"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
)

// defaultBrandingAssets are the text keys with app defaults, served by
// getBranding when the node has not set them (see pkg/branding for the
// full vocabulary).
var defaultBrandingAssets = func() map[string]struct {
	data []byte
	mime string
} {
	m := map[string]struct {
		data []byte
		mime string
	}{}
	for key, def := range branding.Defaults() {
		m[key] = struct {
			data []byte
			mime string
		}{data: []byte(def), mime: branding.TextMime}
	}
	return m
}()

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
		return nil, "", nil, nil, fmt.Errorf("%w: %s", ErrBrandingKeyUnset, key)
	}
	if err != nil {
		return nil, "", nil, nil, fmt.Errorf("error fetching branding blob: %w", err)
	}
	return blob.Data, blob.MimeType, blob.Width, blob.Height, nil
}

// ErrBrandingKeyUnset is returned for a branding key with neither a stored
// value nor a built-in default.
var ErrBrandingKeyUnset = errors.New("branding key is not set")

func (s *Server) handlePlaceStreamBrandingGetBlob(ctx context.Context, broadcasterDID string, key string) (io.Reader, error) {
	r, err := s.HandlePlaceStreamBrandingGetBlobDirect(ctx, broadcasterDID, key)
	if errors.Is(err, ErrBrandingKeyUnset) {
		// 404, not 500: a client polling a key (the front door's
		// defaultVideo) must tell "cleared" from "the node is down".
		return nil, echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	return r, err
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
	for _, adminDID := range s.cli.AdminDIDs {
		if adminDID == did {
			return true
		}
	}
	return false
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

	// validate size and value against the branding vocabulary
	if len(data) > branding.MaxSize(input.Key) {
		return nil, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("blob too large (max %d bytes)", branding.MaxSize(input.Key)))
	}
	if branding.IsText(input.Key) {
		canon, err := branding.Normalize(input.Key, data)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "InvalidValue: "+err.Error())
		}
		data = canon
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

// HandleFaviconICO serves /favicon.ico: the node's branded favicon, else
// the bundled one.
func (s *Server) HandleFaviconICO(c echo.Context) error {
	return s.handleFavicon(c, "favicon.ico", "image/x-icon")
}

// HandleFaviconPNG serves /favicon.png, the icon the app's HTML template
// links first (Expo's web export writes it). It carries the same branded
// favicon as /favicon.ico: link unfurlers (Slack, Discord) and browsers
// take the first icon link they see, and this one used to be the bundled
// brand mark on every node no matter its branding.
func (s *Server) HandleFaviconPNG(c echo.Context) error {
	return s.handleFavicon(c, "favicon.png", "image/png")
}

// handleFavicon serves the branded favicon blob with its own MIME type
// (browsers sniff icon bytes, so an ICO at /favicon.png is fine), falling
// back to the bundled file fallback / fallbackMime.
func (s *Server) handleFavicon(c echo.Context, fallback, fallbackMime string) error {
	ctx := c.Request().Context()
	data, mimeType, _, _, err := s.GetBrandingBlob(ctx, s.cli.BroadcasterDID(), "favicon")
	if err != nil || len(data) == 0 || !strings.HasPrefix(mimeType, "image/") {
		distFiles, fsErr := app.Files()
		if fsErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch favicon")
		}
		f, fsErr := distFiles.Open(fallback)
		if fsErr != nil {
			return echo.NewHTTPError(http.StatusNotFound, "favicon not found")
		}
		defer f.Close()
		data, fsErr = io.ReadAll(f)
		if fsErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to read favicon")
		}
		mimeType = fallbackMime
	}
	// Short-lived: a rebrand shows up within minutes instead of whenever a
	// cache's heuristic for an uncached-header image runs out.
	c.Response().Header().Set("Cache-Control", "public, max-age=300")
	return c.Blob(http.StatusOK, mimeType, data)
}

// HandleLinkBanner serves /linkbanner.png, the image behind the front
// page's OpenGraph card: the node's uploaded linkBanner branding asset with
// its real content type (link crawlers refuse application/octet-stream),
// else the bundled brand banner. Branding is public even on a private node.
func (s *Server) HandleLinkBanner(c echo.Context) error {
	ctx := c.Request().Context()
	data, mimeType, _, _, err := s.GetBrandingBlob(ctx, s.cli.BroadcasterDID(), "linkBanner")
	if err == nil && len(data) > 0 && strings.HasPrefix(mimeType, "image/") {
		c.Response().Header().Set("Cache-Control", "public, max-age=300")
		return c.Blob(http.StatusOK, mimeType, data)
	}
	distFiles, fsErr := app.Files()
	if fsErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load link banner")
	}
	f, fsErr := distFiles.Open("linkbanner.png")
	if fsErr != nil {
		return echo.NewHTTPError(http.StatusNotFound, "link banner not found")
	}
	defer f.Close()
	bs, fsErr := io.ReadAll(f)
	if fsErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to read link banner")
	}
	c.Response().Header().Set("Cache-Control", "public, max-age=300")
	return c.Blob(http.StatusOK, "image/png", bs)
}
