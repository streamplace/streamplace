package media

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"strings"

	"aquareum.tv/aquareum/js/app"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

type TextRenderer struct {
	font *truetype.Font
}

func NewTextRenderer() (*TextRenderer, error) {
	font, err := loadFont()
	if err != nil {
		return nil, err
	}
	return &TextRenderer{font: font}, nil
}

func loadFont() (*truetype.Font, error) {
	entries, err := app.AllFiles.ReadDir("dist/assets/assets/fonts")
	if err != nil {
		return nil, err
	}
	var fontFile string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "FiraCode-Medium") {
			fontFile = fmt.Sprintf("dist/assets/assets/fonts/%s", entry.Name())
			break
		}
	}
	if fontFile == "" {
		return nil, fmt.Errorf("could not find FiraCode-Medium font")
	}
	fd, err := app.AllFiles.Open(fontFile)
	if err != nil {
		return nil, err
	}
	fontBytes, err := io.ReadAll(fd)
	if err != nil {
		return nil, err
	}
	f, err := freetype.ParseFont(fontBytes)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (tr *TextRenderer) GenerateImage(textContent string, fgColorHex string, bgColorHex string, fontSize float64) ([]byte, error) {
	fgColor := color.RGBA{0xff, 0xff, 0xff, 0xff}
	if len(fgColorHex) == 7 {
		_, err := fmt.Sscanf(fgColorHex, "#%02x%02x%02x", &fgColor.R, &fgColor.G, &fgColor.B)
		if err != nil {
			return nil, err
		}
	}

	bgColor := color.RGBA{0x30, 0x0a, 0x24, 0xcc}
	if len(bgColorHex) == 7 {
		_, err := fmt.Sscanf(bgColorHex, "#%02x%02x%02x", &bgColor.R, &bgColor.G, &bgColor.B)
		if err != nil {
			return nil, err
		}
	}

	code := strings.Replace(textContent, "\t", "    ", -1) // convert tabs into spaces
	text := strings.Split(code, "\n")                      // split newlines into arrays

	fg := image.NewUniform(fgColor)
	bg := image.NewUniform(bgColor)
	rgba := image.NewRGBA(image.Rect(0, 0, 540, 55))
	draw.Draw(rgba, rgba.Bounds(), bg, image.Pt(0, 0), draw.Src)
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(tr.font)
	c.SetFontSize(fontSize)
	c.SetClip(rgba.Bounds())
	c.SetDst(rgba)
	c.SetSrc(fg)
	c.SetHinting(font.HintingNone)

	textXOffset := 10
	textYOffset := 5 + int(c.PointToFixed(fontSize)>>6) // Note shift/truncate 6 bits first

	pt := freetype.Pt(textXOffset, textYOffset)
	for _, s := range text {
		_, err := c.DrawString(strings.Replace(s, "\r", "", -1), pt)
		if err != nil {
			return nil, err
		}
		pt.Y += c.PointToFixed(fontSize * 1.5)
	}

	b := new(bytes.Buffer)
	if err := png.Encode(b, rgba); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
