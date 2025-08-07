package api

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/julienschmidt/httprouter"
	"github.com/patrickmn/go-cache"
	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers"
	"stream.place/streamplace/pkg/fonts"
	"stream.place/streamplace/pkg/log"
)

const (
	// Canvas dimensions
	ogWidth  = 400.0
	ogHeight = 200.0

	// Card dimensions and positioning
	cardPadding = 10.0
	cardWidth   = 380.0
	cardHeight  = 180.0
	cardRadius  = 12.0

	// Image dimensions and positioning
	imageX      = 25.0
	imageY      = 45.0
	imageWidth  = 400
	imageHeight = 480
	imageRadius = 180.0
	imageDPMM   = 3.9

	// Text positioning
	textStartX = 135.0
	joinY      = 142.0
	subtitleY  = 115.0
	descY      = 94.0

	// Font sizes
	joinFontSize        = 56.0
	minJoinFontSize     = 40.0
	subtitleFontSize    = 48.0
	descFontSize        = 28.0
	placeholderFontSize = 18.0

	// Available text width
	textAvailableWidth = 255.0

	// Canvas DPI
	canvasDPMM = 2.0
)

var (
	// Colors
	bgColor              = color.RGBA{R: 0, G: 0, B: 0, A: 255}
	cardColor            = color.RGBA{R: 38, G: 38, B: 38, A: 255}
	cardBorderColor      = color.RGBA{R: 64, G: 64, B: 64, A: 255}
	placeholderColor     = color.RGBA{R: 240, G: 240, B: 240, A: 255}
	placeholderTextColor = color.RGBA{R: 100, G: 100, B: 100, A: 255}
	joinTextColor        = color.RGBA{R: 255, G: 200, B: 50, A: 255}
	subtitleColor        = color.RGBA{R: 200, G: 200, B: 200, A: 255}
	descColor            = color.RGBA{R: 180, G: 180, B: 180, A: 255}
	imageBorderColor     = color.RGBA{R: 200, G: 200, B: 200, A: 255}
)

const (
	// Description settings
	maxDescriptionLength = 120
	descriptionTruncate  = 117
)

// createResponsiveJoinText creates a text box for "Join [username]" that fits within the available width
// by reducing font size and truncating with ellipsis if necessary
func createResponsiveJoinText(fontFamily *canvas.FontFamily, text string, availableWidth float64) (*canvas.Text, float64) {
	fontSize := joinFontSize
	minFontSize := minJoinFontSize

	for fontSize >= minFontSize {
		face := fontFamily.Face(fontSize, joinTextColor, canvas.FontBold, canvas.FontNormal)
		textBox := canvas.NewTextBox(face, text, availableWidth, 40, canvas.Left, canvas.Center, &canvas.TextOptions{})

		// Check if text fits
		if textBox.Bounds().W() <= availableWidth {
			return textBox, fontSize
		}

		fontSize -= 2.0 // Reduce font size by 2px each iteration
	}

	// If we get here, even minimum size doesn't fit, so we need to truncate
	face := fontFamily.Face(minFontSize, joinTextColor, canvas.FontBold, canvas.FontNormal)

	// Try progressively shorter versions with ellipsis
	for i := len(text) - 1; i > 0; i-- {
		truncatedText := text[:i] + "..."
		textBox := canvas.NewTextBox(face, truncatedText, availableWidth, 40, canvas.Left, canvas.Center, &canvas.TextOptions{})
		if textBox.Bounds().W() <= availableWidth {
			return textBox, minFontSize
		}
	}

	// Fallback - just ellipsis
	return canvas.NewTextBox(face, "...", availableWidth, 40, canvas.Left, canvas.Center, &canvas.TextOptions{}), minFontSize
}

func (a *StreamplaceAPI) HandleOGImage(ctx context.Context) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		username := ps.ByName("username")
		if username == "" {
			http.Error(w, "Username required", http.StatusBadRequest)
			return
		}

		// Check cache first
		cacheKey := fmt.Sprintf("og_image_%s", username)
		if cached, found := a.OGImageCache.Get(cacheKey); found {
			imgData := cached.([]byte)
			log.Debug(ctx, "OG image cache hit", "username", username, "size_bytes", len(imgData))

			// Set headers
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(imgData)))
			w.Header().Set("X-Cache", "HIT")

			// Write the cached image data
			code, werr := w.Write(imgData)
			if werr != nil {
				log.Error(ctx, "failed to write cached OG image", "username", username, "error", werr)
				http.Error(w, "Failed to write cached image: "+werr.Error(), http.StatusInternalServerError)
				return
			}
			log.Debug(ctx, "OG image served from cache", "username", username, "code", code)
			return
		}

		// Generate the OG image
		log.Debug(ctx, "OG image cache miss, generating new image", "username", username)
		imgData, err := a.generateOGImage(ctx, username)
		if err != nil {
			http.Error(w, "Failed to generate image: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Store in cache
		a.OGImageCache.Set(cacheKey, imgData, cache.DefaultExpiration)
		log.Debug(ctx, "OG image generated and cached", "username", username, "size_bytes", len(imgData))

		// Set headers
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(imgData)))
		w.Header().Set("X-Cache", "MISS")

		// Write the image data
		code, werr := w.Write(imgData)

		if werr != nil {
			log.Error(ctx, "failed to write generated OG image", "username", username, "error", werr)
			http.Error(w, "Failed to write image: "+werr.Error(), http.StatusInternalServerError)
			return
		}
		log.Debug(ctx, "OG image generated and served", "username", username, "code", code)
	}
}

func downloadImage(url string) ([]byte, error) {
	if url == "" {
		return nil, fmt.Errorf("empty URL provided")
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image data: %w", err)
	}

	return imageData, nil
}

func (a *StreamplaceAPI) generateOGImage(ctx context.Context, username string) ([]byte, error) {
	// Fetch user profile and avatar from Bluesky
	var imageURL string
	var handle, description string

	// Set default fallbacks
	handle = username
	description = "Live streaming platform for creators and their communities."

	profileData, err := a.fetchUserProfile(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch profile, because %w", err)
	} else if profileData != nil {
		// Safely extract profile data with nil checks
		if profileData.Avatar != nil && *profileData.Avatar != "" {
			imageURL = *profileData.Avatar
		}

		if profileData.Handle != "" {
			handle = profileData.Handle
		}

		if profileData.Description != nil && *profileData.Description != "" {
			// Truncate description if too long for the image
			desc := *profileData.Description
			if len(desc) > maxDescriptionLength {
				desc = desc[:descriptionTruncate] + "..."
			}
			description = desc
		}
	} else {
		log.Warn(ctx, "received nil profile data, using fallbacks", "username", username)
	}

	// Create new canvas of dimension ogWidth x ogHeight mm for profile card
	c := canvas.New(ogWidth, ogHeight)

	// Create a canvas context used to keep drawing state
	canvasCtx := canvas.NewContext(c)

	fontAHN := canvas.NewFontFamily("Atkinson Hyperlegible Next")

	// Create temporary font files from embedded data
	regularFontFile := filepath.Join(os.TempDir(), "atkinson-regular.ttf")
	boldFontFile := filepath.Join(os.TempDir(), "atkinson-bold.ttf")

	// Write embedded font data to temporary files
	if err := os.WriteFile(regularFontFile, fonts.GetAtkinsonRegular(), 0644); err != nil {
		log.Warn(ctx, "failed to write regular font to temp file", "error", err)
	}
	defer os.Remove(regularFontFile)

	if err := os.WriteFile(boldFontFile, fonts.GetAtkinsonBold(), 0644); err != nil {
		log.Warn(ctx, "failed to write bold font to temp file", "error", err)
	}
	defer os.Remove(boldFontFile)

	// Load fonts from temporary files
	regularErr := fontAHN.LoadFont(fonts.GetAtkinsonRegular(), 0, canvas.FontRegular)
	boldErr := fontAHN.LoadFont(fonts.GetAtkinsonBold(), 0, canvas.FontBold)

	// If font loading fails, the canvas library will fall back to default fonts
	if regularErr != nil {
		log.Warn(ctx, "failed to load regular Atkinson font, using fallback", "error", regularErr)
	}
	if boldErr != nil {
		log.Warn(ctx, "failed to load bold Atkinson font, using fallback", "error", boldErr)
	}

	// Set black background
	canvasCtx.SetFillColor(bgColor)
	canvasCtx.DrawPath(0, 0, canvas.Rectangle(ogWidth, ogHeight))
	canvasCtx.Fill()

	// Create neutral-800 rounded card
	canvasCtx.SetFillColor(cardColor)
	canvasCtx.DrawPath(cardPadding, cardPadding, canvas.RoundedRectangle(cardWidth, cardHeight, cardRadius))
	canvasCtx.Fill()

	// Add subtle border to card
	canvasCtx.SetStrokeColor(cardBorderColor)
	canvasCtx.SetStrokeWidth(1)
	canvasCtx.DrawPath(cardPadding, cardPadding, canvas.RoundedRectangle(cardWidth, cardHeight, cardRadius))
	canvasCtx.Stroke()

	// Try to download and decode the image in memory
	var img image.Image
	if imageURL != "" {
		imageData, downloadErr := downloadImage(imageURL)
		if downloadErr != nil {
			log.Warn(ctx, "failed to download profile image", "username", username, "image_url", imageURL, "error", downloadErr)
		} else {
			// Decode image directly from memory
			reader := bytes.NewReader(imageData)
			var err error
			img, _, err = image.Decode(reader)
			if err != nil {
				log.Warn(ctx, "failed to decode image", "username", username, "error", err)
				img = nil
			}
		}
	}

	if img == nil {
		// Fallback to placeholder if download or loading fails - positioned within card
		canvasCtx.SetFillColor(placeholderColor)
		canvasCtx.DrawPath(imageX, 50, canvas.RoundedRectangle(100, 120, 8))
		canvasCtx.Fill()

		imageFace := fontAHN.Face(placeholderFontSize, placeholderTextColor, canvas.FontBold, canvas.FontNormal)
		imageText := canvas.NewTextBox(imageFace, "Stream.place", 100, 30, canvas.Center, canvas.Center, &canvas.TextOptions{})
		canvasCtx.DrawText(imageX, 100, imageText)
	} else {
		// Create circular mask using image/draw.DrawMask approach with high resolution
		maskWidth, maskHeight := imageWidth, imageHeight

		// Create circular mask image
		mask := image.NewRGBA(image.Rect(0, 0, maskWidth, maskHeight))
		centerX, centerY := maskWidth/2, maskHeight/2
		radius := int(imageRadius)

		// anti-alias
		for y := range maskHeight {
			for x := range maskWidth {
				dx := x - centerX
				dy := y - centerY
				distance := math.Sqrt(float64(dx*dx + dy*dy))

				if distance <= float64(radius) {
					alpha := uint8(255)
					// Anti-aliasing: smooth edges over 2-pixel border
					if distance > float64(radius-2) {
						alpha = uint8(255 * (float64(radius) - distance) / 2.0)
					}
					mask.Set(x, y, color.RGBA{255, 255, 255, alpha}) // White with smooth alpha
				} else {
					mask.Set(x, y, color.RGBA{0, 0, 0, 0}) // Transparent = hidden
				}
			}
		}

		// Create destination image for the masked result
		dst := image.NewRGBA(image.Rect(0, 0, maskWidth, maskHeight))

		// Apply circular mask to the source image using DrawMask
		bounds := img.Bounds()
		scaledImg := image.NewRGBA(image.Rect(0, 0, maskWidth, maskHeight))

		// Scale image while preserving aspect ratio
		imgWidth := float64(bounds.Dx())
		imgHeight := float64(bounds.Dy())
		maskWidthF := float64(maskWidth)
		maskHeightF := float64(maskHeight)

		// Calculate scale to fit image within mask while preserving aspect ratio
		scaleX := maskWidthF / imgWidth
		scaleY := maskHeightF / imgHeight
		scale := min(scaleY, scaleX)

		// Calculate scaled dimensions and centering offsets
		scaledWidth := int(imgWidth * scale)
		scaledHeight := int(imgHeight * scale)
		offsetX := (maskWidth - scaledWidth) / 2
		offsetY := (maskHeight - scaledHeight) / 2

		// Draw scaled image centered in mask
		for y := range scaledHeight {
			for x := range scaledWidth {
				srcX := bounds.Min.X + int((float64(x)*imgWidth)/float64(scaledWidth))
				srcY := bounds.Min.Y + int((float64(y)*imgHeight)/float64(scaledHeight))
				scaledImg.Set(x+offsetX, y+offsetY, img.At(srcX, srcY))
			}
		}

		// Apply the circular mask
		draw.DrawMask(dst, dst.Bounds(), scaledImg, image.Point{}, mask, image.Point{}, draw.Over)

		// Add border around the circular image
		// Calculate exact center and radius based on image positioning and scaling
		borderImgWidth := float64(imageWidth) / imageDPMM
		borderImgHeight := float64(imageHeight) / imageDPMM
		borderCenterX := imageX + borderImgWidth/2
		borderCenterY := imageY + borderImgHeight/2
		borderRadius := imageRadius / imageDPMM

		canvasCtx.SetStrokeColor(imageBorderColor)
		canvasCtx.SetStrokeWidth(3)
		canvasCtx.DrawPath(borderCenterX, borderCenterY, canvas.Circle(borderRadius))
		canvasCtx.Stroke()

		canvasCtx.DrawImage(imageX, imageY, dst, canvas.DPMM(imageDPMM))
	}

	// Create unified responsive "Join @handle" text
	joinUserContent := fmt.Sprintf("Join @%s", handle)

	availableWidth := textAvailableWidth // Full available width for the text
	joinText, _ := createResponsiveJoinText(fontAHN, joinUserContent, availableWidth)
	canvasCtx.DrawText(textStartX, joinY, joinText)

	// Add "streaming on Stream.place" subtitle
	onFace := fontAHN.Face(subtitleFontSize, subtitleColor, canvas.FontRegular, canvas.FontNormal)
	onText := canvas.NewTextBox(onFace, "streaming on Stream.place", 250, 30, canvas.Left, canvas.Center, &canvas.TextOptions{})
	canvasCtx.DrawText(textStartX, subtitleY, onText)

	// Add user description or promotional text
	descFace := fontAHN.Face(descFontSize, descColor, canvas.FontRegular, canvas.FontNormal)
	descText := canvas.NewTextBox(descFace, description, 230, 30, canvas.Left, canvas.Center, &canvas.TextOptions{})
	canvasCtx.DrawText(textStartX, descY, descText)

	b := &bytes.Buffer{}
	if err := c.Write(b, renderers.JPEG(canvas.DPMM(canvasDPMM))); err != nil {
		return nil, fmt.Errorf("failed to render canvas to buffer: %w", err)
	}

	return b.Bytes(), nil
}

func (a *StreamplaceAPI) fetchUserProfile(ctx context.Context, username string) (*bsky.ActorDefs_ProfileViewDetailed, error) {
	// Use ATSync to resolve username to DID, then fetch full profile from Bluesky
	var actor string

	// First try to resolve via internal DB
	repo, err := a.ATSync.Model.GetRepoByHandleOrDID(username)
	if err != nil {
		// log.Warn(ctx, "failed to resolve via ATSync, trying username directly", "username", username, "error", err)
		// // Fall back to using the username directly
		// actor = username
		// if !strings.HasPrefix(username, "did:") && !strings.Contains(username, ".") {
		// 	actor = username + ".bsky.social"
		// }

		return nil, fmt.Errorf("username '%s' not on this node: %w", username, err)
	} else if repo != nil {
		// Use the DID as it's the most reliable identifier
		actor = repo.DID
	} else {
		return nil, fmt.Errorf("no repo found for username %s", username)
	}

	// TODO: check if actor is restricted

	// Fetch full profile from Bluesky public API
	client := &xrpc.Client{
		Host: "https://public.api.bsky.app",
	}

	profile, err := bsky.ActorGetProfile(ctx, client, actor)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch profile from Bluesky for '%s': %w", actor, err)
	}

	if profile == nil {
		return nil, fmt.Errorf("received nil profile from Bluesky API for '%s'", actor)
	}

	return profile, nil
}

// GetOGImageCacheStats returns statistics about the OG image cache
func (a *StreamplaceAPI) GetOGImageCacheStats() map[string]any {
	return map[string]any{
		"item_count": a.OGImageCache.ItemCount(),
	}
}

// InvalidateOGImageCache removes a specific user's OG image from cache
func (a *StreamplaceAPI) InvalidateOGImageCache(username string) {
	cacheKey := fmt.Sprintf("og_image_%s", username)
	a.OGImageCache.Delete(cacheKey)
}

// ClearOGImageCache removes all cached OG images
func (a *StreamplaceAPI) ClearOGImageCache() {
	a.OGImageCache.Flush()
}
