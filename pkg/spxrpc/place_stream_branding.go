package spxrpc

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"

	"github.com/patrickmn/go-cache"
	"gorm.io/gorm"
	placestreamtypes "stream.place/streamplace/pkg/streamplace"
)

//go:embed assets/logo.svg
var defaultLogoSVG []byte

//go:embed assets/favicon.svg
var defaultFaviconSVG []byte

var defaultBrandingAssets = map[string]struct {
	data []byte
	mime string
}{
	"mainLogo":         {data: defaultLogoSVG, mime: "image/svg+xml"},
	"favicon":          {data: defaultFaviconSVG, mime: "image/svg+xml"},
	"siteTitle":        {data: []byte("Streamplace"), mime: "text/plain"},
	"siteDescription":  {data: []byte("Live streaming platform"), mime: "text/plain"},
	"primaryColor":     {data: []byte("#6366f1"), mime: "text/plain"},
	"accentColor":      {data: []byte("#8b5cf6"), mime: "text/plain"},
	"defaultStreamKey": {data: []byte(""), mime: "text/plain"},
}

func (s *Server) getBroadcasterID(ctx context.Context) string {
	// for now, use BroadcasterHost as the ID
	// in the future, this could come from session/auth context
	return s.cli.BroadcasterHost
}

func (s *Server) getBrandingBlobCached(ctx context.Context, broadcasterID, key string) ([]byte, string, error) {
	cacheKey := fmt.Sprintf("%s:%s", broadcasterID, key)

	// check cache first
	if cached, found := s.BrandingCache.Get(cacheKey); found {
		blob := cached.(struct {
			data []byte
			mime string
		})
		return blob.data, blob.mime, nil
	}

	// cache miss - fetch from db
	blob, err := s.statefulDB.GetBrandingBlob(broadcasterID, key)
	if err == gorm.ErrRecordNotFound {
		// not in db, use default
		if def, ok := defaultBrandingAssets[key]; ok {
			// cache the default too
			s.BrandingCache.Set(cacheKey, def, cache.DefaultExpiration)
			return def.data, def.mime, nil
		}
		return nil, "", fmt.Errorf("unknown branding key: %s", key)
	}
	if err != nil {
		return nil, "", fmt.Errorf("error fetching branding blob: %w", err)
	}

	// store in cache
	cacheData := struct {
		data []byte
		mime string
	}{data: blob.Data, mime: blob.MimeType}
	s.BrandingCache.Set(cacheKey, cacheData, cache.DefaultExpiration)

	return blob.Data, blob.MimeType, nil
}

func (s *Server) handlePlaceStreamBrandingGetBlob(ctx context.Context, key string) (io.Reader, error) {
	broadcasterID := s.getBroadcasterID(ctx)
	data, _, err := s.getBrandingBlobCached(ctx, broadcasterID, key)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (s *Server) handlePlaceStreamBrandingGetBranding(ctx context.Context) (*placestreamtypes.BrandingGetBranding_Output, error) {
	broadcasterID := s.getBroadcasterID(ctx)

	// get all keys from database
	dbKeys, err := s.statefulDB.ListBrandingKeys(broadcasterID)
	if err != nil {
		return nil, fmt.Errorf("error listing branding keys: %w", err)
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
	assets := make([]*placestreamtypes.BrandingGetBranding_BrandingAsset, 0, len(allKeys))
	for key := range allKeys {
		_, mimeType, err := s.getBrandingBlobCached(ctx, broadcasterID, key)
		if err != nil {
			continue // skip if error
		}

		// construct URL - need to get base URL from echo context
		url := fmt.Sprintf("/xrpc/place.stream.branding.getBlob?key=%s", key)

		assets = append(assets, &placestreamtypes.BrandingGetBranding_BrandingAsset{
			Key:      key,
			MimeType: mimeType,
			Url:      url,
		})
	}

	return &placestreamtypes.BrandingGetBranding_Output{
		Assets: assets,
	}, nil
}
