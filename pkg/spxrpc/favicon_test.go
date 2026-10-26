package spxrpc

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"stream.place/streamplace/js/app"
	"stream.place/streamplace/pkg/config"
	"stream.place/streamplace/pkg/statedb"
)

func bundledFile(t *testing.T, name string) []byte {
	t.Helper()
	files, err := app.Files()
	require.NoError(t, err)
	f, err := files.Open(name)
	require.NoError(t, err)
	defer f.Close()
	data, err := io.ReadAll(f)
	require.NoError(t, err)
	return data
}

// The PNG route preserves uploaded artwork; the legacy ICO route must
// encode it as a real icon, including existing SVG branding in the database.
func TestFaviconRoutesServeBranding(t *testing.T) {
	ctx := context.Background()
	sdb, err := statedb.MakeDB(ctx, &config.CLI{DBURL: ":memory:"}, nil, nil)
	require.NoError(t, err)
	s := &Server{cli: &config.CLI{BroadcasterHost: "example.com"}, statefulDB: sdb}
	serve := func(h echo.HandlerFunc, path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, path, nil), rec)
		require.NoError(t, h(c))
		return rec
	}

	pngResponse := serve(s.HandleFaviconPNG, "/favicon.png")
	require.Equal(t, http.StatusOK, pngResponse.Code)
	require.Equal(t, "image/png", pngResponse.Header().Get("Content-Type"))
	require.Equal(t, bundledFile(t, "favicon.png"), pngResponse.Body.Bytes())
	ico := serve(s.HandleFaviconICO, "/favicon.ico")
	require.Equal(t, "image/x-icon", ico.Header().Get("Content-Type"))
	require.Equal(t, bundledFile(t, "favicon.ico"), ico.Body.Bytes())

	var brandedPNG bytes.Buffer
	pixels := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	pixels.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	pixels.SetNRGBA(1, 0, color.NRGBA{R: 255, A: 255})
	require.NoError(t, png.Encode(&brandedPNG, pixels))
	for _, tc := range []struct {
		name string
		mime string
		data []byte
		want color.NRGBA
	}{
		{"svg", "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 18 18"><rect width="18" height="18" fill="#808080"/></svg>`), color.NRGBA{R: 128, G: 128, B: 128, A: 255}},
		{"rebrand", "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18"><rect width="18" height="18" fill="#ff0000"/></svg>`), color.NRGBA{R: 255, A: 255}},
		{"png", "image/png", brandedPNG.Bytes(), color.NRGBA{R: 255, A: 255}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, sdb.PutBrandingBlob("did:web:example.com", "favicon", tc.mime, tc.data, nil, nil))
			rec := serve(s.HandleFaviconPNG, "/favicon.png")
			require.Equal(t, tc.data, rec.Body.Bytes())
			require.Equal(t, tc.mime, rec.Header().Get("Content-Type"))
			rec = serve(s.HandleFaviconICO, "/favicon.ico")
			require.Equal(t, "image/x-icon", rec.Header().Get("Content-Type"))
			require.Equal(t, "public, max-age=300", rec.Header().Get("Cache-Control"))
			data := rec.Body.Bytes()
			require.GreaterOrEqual(t, len(data), 22)
			require.Equal(t, []byte{0, 0, 1, 0}, data[:4])
			require.Positive(t, binary.LittleEndian.Uint16(data[4:6]))
			size := int(binary.LittleEndian.Uint32(data[14:18]))
			offset := int(binary.LittleEndian.Uint32(data[18:22]))
			require.GreaterOrEqual(t, offset, 22)
			require.LessOrEqual(t, offset+size, len(data))
			img, err := png.Decode(bytes.NewReader(data[offset : offset+size]))
			require.NoError(t, err)
			bounds := img.Bounds()
			require.Equal(t, tc.want, color.NRGBAModel.Convert(img.At(bounds.Dx()/2, bounds.Dy()/2)))
			if tc.name == "png" {
				// A wide image is contained, not stretched into a square.
				require.Equal(t, color.NRGBA{}, color.NRGBAModel.Convert(img.At(0, 0)))
			}
		})
	}

	// Byte limits alone do not bound decoded memory. This highly compressed
	// image exceeds the raster conversion limit despite its small file size.
	var oversized bytes.Buffer
	require.NoError(t, png.Encode(&oversized, image.NewGray(image.Rect(0, 0, 2049, 2048))))
	for _, tc := range []struct {
		mime string
		data []byte
	}{
		{"image/png", oversized.Bytes()},
		{"image/png", []byte("broken PNG")},
		{"image/svg+xml", []byte("broken SVG")},
		{"image/x-icon", []byte{0, 0, 1, 0, 1, 0}},
	} {
		require.NoError(t, sdb.PutBrandingBlob("did:web:example.com", "favicon", tc.mime, tc.data, nil, nil))
		rec := serve(s.HandleFaviconICO, "/favicon.ico")
		require.Equal(t, "image/x-icon", rec.Header().Get("Content-Type"))
		require.Equal(t, bundledFile(t, "favicon.ico"), rec.Body.Bytes())
	}

	// Already encoded ICOs are not decoded/re-encoded, and deletion drops
	// any cached conversion immediately.
	bundledICO := bundledFile(t, "favicon.ico")
	require.NoError(t, sdb.PutBrandingBlob("did:web:example.com", "favicon", "image/x-icon", bundledICO, nil, nil))
	require.Equal(t, bundledICO, serve(s.HandleFaviconICO, "/favicon.ico").Body.Bytes())
	require.NoError(t, sdb.DeleteBrandingBlob("did:web:example.com", "favicon"))
	require.Equal(t, bundledICO, serve(s.HandleFaviconICO, "/favicon.ico").Body.Bytes())
}

// /favicon.png?scheme= serves that color scheme's variant with its own MIME
// type, and falls back to the generic favicon, then the bundled one, when
// the variant is missing, deleted, not an image, or the scheme is unknown.
func TestFaviconSchemeVariants(t *testing.T) {
	ctx := context.Background()
	sdb, err := statedb.MakeDB(ctx, &config.CLI{DBURL: ":memory:"}, nil, nil)
	require.NoError(t, err)
	s := &Server{cli: &config.CLI{BroadcasterHost: "example.com"}, statefulDB: sdb}
	const broadcaster = "did:web:example.com"
	type icon struct {
		data []byte
		mime string
	}
	bundled := icon{bundledFile(t, "favicon.png"), "image/png"}
	generic := icon{[]byte{0x89, 'P', 'N', 'G', 'g'}, "image/png"}
	light := icon{[]byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect id="light"/></svg>`), "image/svg+xml"}
	dark := icon{[]byte("RIFF....WEBPdark"), "image/webp"}

	serve := func(query string) icon {
		t.Helper()
		rec := httptest.NewRecorder()
		c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/favicon.png"+query, nil), rec)
		require.NoError(t, s.HandleFaviconPNG(c), query)
		require.Equal(t, http.StatusOK, rec.Code, query)
		require.Equal(t, "public, max-age=300", rec.Header().Get("Cache-Control"), query)
		return icon{rec.Body.Bytes(), rec.Header().Get("Content-Type")}
	}
	put := func(key string, i icon) {
		t.Helper()
		require.NoError(t, sdb.PutBrandingBlob(broadcaster, key, i.mime, i.data, nil, nil))
	}
	expect := func(generic, light, dark icon) {
		t.Helper()
		require.Equal(t, generic, serve(""))
		require.Equal(t, generic, serve("?scheme=bogus"))
		require.Equal(t, generic, serve("?scheme="))
		require.Equal(t, light, serve("?scheme=light"))
		require.Equal(t, dark, serve("?scheme=dark"))
	}

	expect(bundled, bundled, bundled)

	put("favicon", generic)
	expect(generic, generic, generic)

	put("faviconLight", light)
	expect(generic, light, generic)

	put("faviconDark", dark)
	expect(generic, light, dark)

	// Variants win over the generic icon but never replace it for clients
	// that don't ask for a scheme.
	require.NoError(t, sdb.DeleteBrandingBlob(broadcaster, "favicon"))
	expect(bundled, light, dark)

	// A variant that isn't an image is skipped, not served.
	put("faviconLight", icon{[]byte("not an image"), "text/plain"})
	put("favicon", generic)
	expect(generic, generic, dark)

	require.NoError(t, sdb.DeleteBrandingBlob(broadcaster, "faviconDark"))
	expect(generic, generic, generic)
}
