package spxrpc

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"strings"

	imagedraw "image/draw"

	"golang.org/x/image/draw"
	"golang.org/x/net/context/ctxhttp"

	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/labstack/echo/v4"
	"github.com/patrickmn/go-cache"
	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers"
	"stream.place/streamplace/js/app"
	"stream.place/streamplace/pkg/aqhttp"
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
		// Try bold first, fall back to regular if bold fails
		face := fontFamily.Face(fontSize, joinTextColor, canvas.FontBold, canvas.FontNormal)
		if face == nil {
			face = fontFamily.Face(fontSize, joinTextColor, canvas.FontRegular, canvas.FontNormal)
		}

		if face != nil {
			textBox := canvas.NewTextBox(face, text, availableWidth, 40, canvas.Left, canvas.Center, &canvas.TextOptions{})

			// Check if text fits
			if textBox.Bounds().W() <= availableWidth {
				return textBox, fontSize
			}
		}

		fontSize -= 2.0 // Reduce font size by 2px each iteration
	}

	// If we get here, even minimum size doesn't fit, so we need to truncate
	face := fontFamily.Face(minFontSize, joinTextColor, canvas.FontBold, canvas.FontNormal)
	if face == nil {
		face = fontFamily.Face(minFontSize, joinTextColor, canvas.FontRegular, canvas.FontNormal)
	}

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

func (s *Server) handlePlaceStreamLiveGetProfileCard(ctx context.Context, id string) (io.Reader, error) {
	if id == "" {
		return nil, errors.New("id required")
	}

	// Get Echo context to set response headers
	c, ok := ctx.Value(echoContextKey).(echo.Context)
	if ok {
		// Set appropriate headers for image response
		c.Response().Header().Set("Content-Type", "image/jpeg")
		c.Response().Header().Set("Cache-Control", "public, max-age=300") // 5 minutes
		c.Response().Header().Set("X-Content-Type-Options", "nosniff")
	}

	// trim ending slash if any
	username := strings.TrimRight(id, "/")

	cacheKey := fmt.Sprintf("og_image_%s", username)
	if cached, found := s.OGImageCache.Get(cacheKey); found {
		imgData := cached.([]byte)
		log.Debug(ctx, "OG image cache hit", "username", username, "size_bytes", len(imgData))
		return bytes.NewReader(imgData), nil
	}

	imgData, err := s.generateOGImage(ctx, username)
	if err != nil {
		log.Error(ctx, "failed to generate OG image", "username", username, "error", err)
		return nil, err
	}

	s.OGImageCache.Set(cacheKey, imgData, cache.DefaultExpiration)
	log.Debug(ctx, "OG image generated and cached", "username", username, "size_bytes", len(imgData))

	return bytes.NewReader(imgData), nil
}

func downloadImage(ctx context.Context, url string) ([]byte, error) {
	if url == "" {
		return nil, errors.New("empty URL provided")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := ctxhttp.Do(ctx, &aqhttp.Client, req)
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

func (s *Server) generateOGImage(ctx context.Context, username string) ([]byte, error) {
	// Fetch user profile and avatar from Bluesky
	var imageURL string
	var handle, description string

	// Set default fallbacks
	handle = username
	description = "Live streaming platform for creators and their communities."

	profileData, err := s.fetchUserProfile(ctx, username)
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

	regularData, regularDataErr := getAtkinsonRegular()
	if regularDataErr != nil {
		log.Warn(ctx, "failed to load regular Atkinson font data", "error", regularDataErr)
	}

	boldData, boldDataErr := getAtkinsonBold()
	if boldDataErr != nil {
		log.Warn(ctx, "failed to load bold Atkinson font data", "error", boldDataErr)
	}

	var regularErr, boldErr error
	if regularDataErr == nil {
		regularErr = fontAHN.LoadFont(regularData, 0, canvas.FontRegular)
	}
	if boldDataErr == nil {
		boldErr = fontAHN.LoadFont(boldData, 0, canvas.FontBold)
	}

	// If font loading fails, the canvas library will fall back to default fonts
	if regularErr != nil {
		log.Warn(ctx, "failed to load regular Atkinson font, using fallback", "error", regularErr)
	}
	if boldErr != nil {
		log.Warn(ctx, "failed to load bold Atkinson font, using fallback", "error", boldErr)
	}

	// If both custom fonts failed to load, ensure we have a working font family
	if (regularDataErr != nil || regularErr != nil) && (boldDataErr != nil || boldErr != nil) {
		log.Warn(ctx, "all custom fonts failed to load, using system default")
		fontAHN = canvas.NewFontFamily("sans-serif")
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
		imageData, downloadErr := downloadImage(ctx, imageURL)
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
		imageText := canvas.NewTextBox(imageFace, "Streamplace", 100, 30, canvas.Center, canvas.Center, &canvas.TextOptions{})
		canvasCtx.DrawText(imageX, 100, imageText)
	} else {
		// High-quality avatar processing with circular masking
		avatarDisplaySize := imageRadius * 2 / imageDPMM
		avatarSize := int(avatarDisplaySize * canvasDPMM)

		// High-quality scaling with center cropping
		bounds := img.Bounds()
		srcWidth, srcHeight := bounds.Dx(), bounds.Dy()

		// Calculate square crop (center crop for circular fit)
		cropSize := srcWidth
		if srcHeight < cropSize {
			cropSize = srcHeight
		}
		cropOffsetX := (srcWidth - cropSize) / 2
		cropOffsetY := (srcHeight - cropSize) / 2
		cropRect := image.Rect(
			bounds.Min.X+cropOffsetX,
			bounds.Min.Y+cropOffsetY,
			bounds.Min.X+cropOffsetX+cropSize,
			bounds.Min.Y+cropOffsetY+cropSize,
		)

		scaledAvatar := image.NewRGBA(image.Rect(0, 0, avatarSize, avatarSize))
		draw.CatmullRom.Scale(scaledAvatar, scaledAvatar.Bounds(), img, cropRect, draw.Over, nil)

		// Create circular alpha mask
		mask := image.NewAlpha(image.Rect(0, 0, avatarSize, avatarSize))
		center := avatarSize / 2
		radius := float64(center)

		// Generate anti-aliased circular mask
		for y := 0; y < avatarSize; y++ {
			for x := 0; x < avatarSize; x++ {
				dx := float64(x - center)
				dy := float64(y - center)
				distance := math.Sqrt(dx*dx + dy*dy)

				if distance <= radius {
					alpha := 255.0
					if distance > radius-1 {
						alpha = 255.0 * (radius - distance)
					}
					mask.SetAlpha(x, y, color.Alpha{uint8(alpha)})
				}
			}
		}

		// Apply circular mask
		maskedAvatar := image.NewRGBA(image.Rect(0, 0, avatarSize, avatarSize))
		imagedraw.DrawMask(maskedAvatar, maskedAvatar.Bounds(), scaledAvatar, image.Point{}, mask, image.Point{}, imagedraw.Over)

		// Add circular border
		avatarCenterX := imageX + avatarDisplaySize/2
		avatarCenterY := imageY + avatarDisplaySize/2
		canvasCtx.SetStrokeColor(imageBorderColor)
		canvasCtx.SetStrokeWidth(3)
		canvasCtx.DrawPath(avatarCenterX, avatarCenterY, canvas.Circle(avatarDisplaySize/2))
		canvasCtx.Stroke()

		// Draw the final circular avatar
		canvasCtx.DrawImage(imageX, imageY, maskedAvatar, canvas.DPMM(canvasDPMM))
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

// getAtkinsonRegular returns the regular Atkinson Hyperlegible Next font data from app filesystem
func getAtkinsonRegular() ([]byte, error) {
	files, err := app.Assets()
	if err != nil {
		return nil, fmt.Errorf("failed to get app assets: %w", err)
	}

	file, err := files.Open("fonts/AtkinsonHyperlegibleNext-Regular.ttf")
	if err != nil {
		return nil, fmt.Errorf("failed to open regular font: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read regular font: %w", err)
	}

	return data, nil
}

// getAtkinsonBold returns the bold Atkinson Hyperlegible Next font data from app filesystem
func getAtkinsonBold() ([]byte, error) {
	files, err := app.Assets()
	if err != nil {
		return nil, fmt.Errorf("failed to get app assets: %w", err)
	}

	file, err := files.Open("fonts/AtkinsonHyperlegibleNext-Bold.ttf")
	if err != nil {
		return nil, fmt.Errorf("failed to open bold font: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read bold font: %w", err)
	}

	return data, nil
}

func (s *Server) fetchUserProfile(ctx context.Context, username string) (*bsky.ActorDefs_ProfileViewDetailed, error) {
	// Use ATSync to resolve username to DID, then fetch full profile from Bluesky
	var actor string

	// First try to resolve via internal DB
	repo, err := s.ATSync.Model.GetRepoByHandleOrDID(username)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUserNotFound, err)
	} else if repo != nil {
		// Use the DID as it's the most reliable identifier
		actor = repo.DID
	} else {
		return nil, fmt.Errorf("no repo found for username: %s (%w)", username, ErrUserNotFound)
	}

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
