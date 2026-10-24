package spxrpc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	_ "image/gif"
	"image/png"
	"math"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers/rasterizer"
	"golang.org/x/image/draw"
)

// Keep only the current conversion; the source digest makes uploads, bundle
// imports and deletions take effect without separate cache invalidation paths.
type encodedFavicon struct {
	source [32]byte
	mime   string
	data   []byte
}

func encodeFaviconICO(data []byte, mime string) ([]byte, error) {
	if len(data) >= 6 && bytes.Equal(data[:4], []byte{0, 0, 1, 0}) {
		count := int(binary.LittleEndian.Uint16(data[4:6]))
		if count == 0 || len(data) < 6+16*count {
			return nil, fmt.Errorf("invalid ICO directory")
		}
		for i := range count {
			entry := data[6+16*i : 6+16*(i+1)]
			size := uint64(binary.LittleEndian.Uint32(entry[8:12]))
			offset := uint64(binary.LittleEndian.Uint32(entry[12:16]))
			if size == 0 || offset < uint64(6+16*count) || offset+size > uint64(len(data)) {
				return nil, fmt.Errorf("invalid ICO image bounds")
			}
		}
		return data, nil
	}

	const size = 64
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	if mime == "image/svg+xml" {
		art, err := canvas.ParseSVG(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("parse favicon SVG: %w", err)
		}
		w, h := art.Size()
		if !(w > 0 && h > 0) || math.IsInf(w, 0) || math.IsInf(h, 0) {
			return nil, fmt.Errorf("invalid favicon SVG dimensions")
		}
		scale := size / math.Max(w, h)
		renderer := rasterizer.FromImage(img, canvas.DPMM(1), canvas.DefaultColorSpace)
		art.RenderViewTo(renderer, canvas.Identity.Translate((size-w*scale)/2, (size-h*scale)/2).Scale(scale, scale))
		renderer.Close()
	} else {
		art, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("decode favicon: %w", err)
		}
		bounds := art.Bounds()
		scale := float64(size) / float64(max(bounds.Dx(), bounds.Dy()))
		w := max(1, int(math.Round(float64(bounds.Dx())*scale)))
		h := max(1, int(math.Round(float64(bounds.Dy())*scale)))
		x, y := (size-w)/2, (size-h)/2
		draw.CatmullRom.Scale(img, image.Rect(x, y, x+w, y+h), art, bounds, draw.Src, nil)
	}

	// Like the bundled favicon, an ICO directory containing a PNG image.
	buf := bytes.NewBuffer(make([]byte, 22, 4096))
	if err := png.Encode(buf, img); err != nil {
		return nil, fmt.Errorf("encode favicon PNG: %w", err)
	}
	ico := buf.Bytes()
	binary.LittleEndian.PutUint16(ico[2:4], 1) // Icon, not cursor.
	binary.LittleEndian.PutUint16(ico[4:6], 1) // One directory entry.
	ico[6], ico[7] = size, size
	binary.LittleEndian.PutUint16(ico[10:12], 1)
	binary.LittleEndian.PutUint16(ico[12:14], 32)
	binary.LittleEndian.PutUint32(ico[14:18], uint32(len(ico)-22))
	binary.LittleEndian.PutUint32(ico[18:22], 22)
	return ico, nil
}
