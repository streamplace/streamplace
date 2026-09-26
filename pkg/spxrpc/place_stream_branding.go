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

// getBroadcasterID resolves a request's `broadcaster` parameter. Without
// one, the brand is the one for the hostname the request arrived on: a
// custom domain's own, else the node's.
func (s *Server) getBroadcasterID(ctx context.Context, broadcasterDID string) string {
	if strings.TrimSpace(broadcasterDID) == "" {
		id, _ := branding.ResolveHost(s.statefulDB, s.cli.BroadcasterHost, branding.RequestHost(ctx))
		return id
	}
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

	// build key set including defaults; build-time keys (native icons, the
	// app's identity) are for app builds, not the running app
	allKeys := make(map[string]bool)
	for _, key := range dbKeys {
		if branding.IsRuntime(key) {
			allKeys[key] = true
		}
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
	var broadcasterDID string
	if input.Broadcaster != nil {
		broadcasterDID = *input.Broadcaster
	}
	target, err := s.brandWriteTarget(ctx, broadcasterDID)
	if err != nil {
		return nil, err
	}

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

	if target.domain != nil {
		// A custom domain's brand is a record, which has room for the
		// vocabulary's keys and the image types it accepts, nothing else.
		mimeType := input.MimeType
		switch {
		case !branding.Known(input.Key):
			return nil, echo.NewHTTPError(http.StatusBadRequest, "InvalidValue: unknown branding key "+input.Key)
		case branding.IsText(input.Key):
			mimeType = branding.TextMime
		case !branding.AcceptsImage(mimeType):
			return nil, echo.NewHTTPError(http.StatusBadRequest, "InvalidValue: "+mimeType+" is not an image type a brand may use")
		}
		err := s.editDomainBrand(ctx, target, func(values branding.Values) (branding.Values, error) {
			if len(data) == 0 {
				delete(values, input.Key)
			} else {
				values[input.Key] = branding.Value{MimeType: mimeType, Data: data}
			}
			return values, nil
		})
		if err != nil {
			return nil, err
		}
		return &placestream.BrandingUpdateBlob_Output{Success: true}, nil
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

	err = s.statefulDB.PutBrandingBlob(target.brandID, input.Key, input.MimeType, data, width, height)
	if err != nil {
		log.Error(ctx, "failed to store branding blob", "err", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "unable to store branding blob")
	}

	return &placestream.BrandingUpdateBlob_Output{
		Success: true,
	}, nil
}

func (s *Server) handlePlaceStreamBrandingDeleteBlob(ctx context.Context, input *placestream.BrandingDeleteBlob_Input) (*placestream.BrandingDeleteBlob_Output, error) {
	var broadcasterDID string
	if input.Broadcaster != nil {
		broadcasterDID = *input.Broadcaster
	}
	target, err := s.brandWriteTarget(ctx, broadcasterDID)
	if err != nil {
		return nil, err
	}

	if target.domain != nil {
		err := s.editDomainBrand(ctx, target, func(values branding.Values) (branding.Values, error) {
			if _, ok := values[input.Key]; !ok {
				return nil, echo.NewHTTPError(http.StatusNotFound, "branding asset not found")
			}
			delete(values, input.Key)
			return values, nil
		})
		if err != nil {
			return nil, err
		}
		return &placestream.BrandingDeleteBlob_Output{Success: true}, nil
	}

	err = s.statefulDB.DeleteBrandingBlob(target.brandID, input.Key)
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
	brandID, _ := branding.ResolveHost(s.statefulDB, s.cli.BroadcasterHost, c.Request().Host)
	data, mimeType, _, _, err := s.GetBrandingBlob(ctx, brandID, "favicon")
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
	brandID, _ := branding.ResolveHost(s.statefulDB, s.cli.BroadcasterHost, c.Request().Host)
	data, mimeType, _, _, err := s.GetBrandingBlob(ctx, brandID, "linkBanner")
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
