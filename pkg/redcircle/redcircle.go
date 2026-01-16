package redcircle

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers"
)

const WIDTH = 1000.0
const HEIGHT = 1000.0

const BORDER_SIZE = 62.0

//go:embed redcircle.png
var RedCirclePNG []byte

func GenerateRedCircle(ctx context.Context, profileJPEG []byte) ([]byte, error) {
	c := canvas.New(WIDTH, HEIGHT)
	reader := bytes.NewReader(RedCirclePNG)
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	profileReader := bytes.NewReader(profileJPEG)
	profileImg, _, err := image.Decode(profileReader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	scaleFactor := WIDTH / (WIDTH - BORDER_SIZE*2)

	// Draw the decoded image onto the canvas
	canvasCtx := canvas.NewContext(c)
	canvasCtx.DrawImage(BORDER_SIZE, BORDER_SIZE, profileImg, canvas.DPMM(scaleFactor))
	canvasCtx.DrawImage(0, 0, img, canvas.DPMM(1))

	var buf bytes.Buffer

	if err := c.Write(&buf, renderers.JPEG()); err != nil {
		return nil, fmt.Errorf("failed to render to JPEG: %w", err)
	}
	return buf.Bytes(), nil
}
