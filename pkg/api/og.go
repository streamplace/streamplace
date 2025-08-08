package api

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"strings"

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
	imageY      = 55.0
	imageWidth  = 400
	imageHeight = 480
	imageRadius = 180.0
	imageDPMM   = 3.9

	// Text positioning
	textStartX = 135.0
	joinY      = 142.0
	subtitleY  = 115.0
	descY      = 90.0

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

var ErrUserNotFound = errors.New("user not found")

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
		imgData, err := a.generateOGImage(ctx, username)
		if err != nil {
			if errors.Is(err, ErrUserNotFound) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			log.Error(ctx, "failed to generate OG image", "username", username, "error", err)
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
			// if includes "no repo found" in error, return 404
			if strings.Contains(werr.Error(), "no repo found") {
				http.Error(w, "User not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to write image: "+werr.Error(), http.StatusInternalServerError)
			return
		}
		log.Debug(ctx, "OG image generated and served", "username", username, "code", code)
	}
}

func downloadImage(url string) ([]byte, error) {
	if url == "" {
		return nil, errors.New("empty URL provided")
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
		// Create efficient circular masked avatar
		avatarSize := int(imageRadius * 2)
		center := avatarSize / 2

		// Create circular mask using efficient alpha channel
		mask := image.NewAlpha(image.Rect(0, 0, avatarSize, avatarSize))

		// Draw anti-aliased circle using efficient algorithm
		for y := 0; y < avatarSize; y++ {
			for x := 0; x < avatarSize; x++ {
				dx := float64(x - center)
				dy := float64(y - center)
				distance := math.Sqrt(dx*dx + dy*dy)

				if distance <= float64(center) {
					// Anti-aliasing on edge
					alpha := 255.0
					if distance > float64(center-1) {
						alpha = 255.0 * (float64(center) - distance)
					}
					mask.SetAlpha(x, y, color.Alpha{uint8(math.Max(0, alpha))})
				}
			}
		}

		// Create destination for final masked image
		dst := image.NewRGBA(image.Rect(0, 0, avatarSize, avatarSize))

		// Scale source image to fill circle (crop to fit)
		bounds := img.Bounds()
		srcWidth, srcHeight := bounds.Dx(), bounds.Dy()

		// Calculate scale to fill circle completely
		scale := math.Max(float64(avatarSize)/float64(srcWidth), float64(avatarSize)/float64(srcHeight))
		scaledWidth := int(float64(srcWidth) * scale)
		scaledHeight := int(float64(srcHeight) * scale)

		// Create properly scaled image
		scaledImg := image.NewRGBA(image.Rect(0, 0, avatarSize, avatarSize))
		offsetX := (avatarSize - scaledWidth) / 2
		offsetY := (avatarSize - scaledHeight) / 2

		// Efficient nearest-neighbor scaling
		for y := 0; y < scaledHeight; y++ {
			for x := 0; x < scaledWidth; x++ {
				srcX := bounds.Min.X + (x * srcWidth / scaledWidth)
				srcY := bounds.Min.Y + (y * srcHeight / scaledHeight)
				scaledImg.Set(x+offsetX, y+offsetY, img.At(srcX, srcY))
			}
		}

		// Apply circular mask using efficient DrawMask
		draw.DrawMask(dst, dst.Bounds(), scaledImg, image.Point{}, mask, image.Point{}, draw.Over)

		// Add circular border
		avatarDisplaySize := imageRadius * 2 / imageDPMM
		borderCenterX := imageX + avatarDisplaySize/2
		borderCenterY := imageY + avatarDisplaySize/2
		borderRadius := avatarDisplaySize / 2

		canvasCtx.SetStrokeColor(imageBorderColor)
		canvasCtx.SetStrokeWidth(3)
		canvasCtx.DrawPath(borderCenterX, borderCenterY, canvas.Circle(borderRadius))
		canvasCtx.Stroke()

		// Draw the masked image to canvas
		canvasCtx.DrawImage(imageX, imageY, dst, canvas.DPMM(imageDPMM))
	}

	// Create unified responsive "Join @handle" text
	joinUserContent := fmt.Sprintf("Catch @%s", handle)

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

		return nil, fmt.Errorf("%w: %w", ErrUserNotFound, err)
	} else if repo != nil {
		// Use the DID as it's the most reliable identifier
		actor = repo.DID
	} else {
		return nil, fmt.Errorf("no repo found for username: %s (%w)", username, ErrUserNotFound)
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
