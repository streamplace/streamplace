package spxrpc

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"hash/fnv"
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

	"github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/labstack/echo/v4"
	"github.com/patrickmn/go-cache"
	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers"
	"stream.place/streamplace/js/app"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
)

const (
	// Canvas dimensions
	ogWidth  = 400.0
	ogHeight = 210.0

	// Card dimensions and positioning
	cardPadding = 10.0
	cardWidth   = 380.0
	cardHeight  = 190.0
	cardRadius  = 12.0

	// Image dimensions and positioning
	imageX      = 27.5
	imageY      = 60.0
	imageWidth  = 400
	imageHeight = 480
	imageRadius = 180.0
	imageDPMM   = 3.9

	// Text positioning
	textStartX = 135.0
	joinY      = 147.0
	subtitleY  = 120.0
	descY      = 95.0

	// Font sizes
	joinFontSize        = 56.0
	minJoinFontSize     = 48.0
	subtitleFontSize    = 48.0
	descFontSize        = 28.0
	placeholderFontSize = 18.0

	// Available text width
	textAvailableWidth = 245.0

	// Canvas DPI
	canvasDPMM = 3.0
)

var (
	// Colors
	bgColor              = color.RGBA{R: 0, G: 0, B: 0, A: 255}
	cardColor            = color.RGBA{R: 38, G: 38, B: 38, A: 255}
	cardBorderColor      = color.RGBA{R: 64, G: 64, B: 64, A: 255}
	placeholderColor     = color.RGBA{R: 240, G: 240, B: 240, A: 255}
	placeholderTextColor = color.RGBA{R: 100, G: 100, B: 100, A: 255}
	joinTextColor        = color.RGBA{R: 248, G: 186, B: 202, A: 255}
	subtitleColor        = color.RGBA{R: 200, G: 200, B: 200, A: 255}
	descColor            = color.RGBA{R: 180, G: 180, B: 180, A: 255}
	imageBorderColor     = color.RGBA{R: 248, G: 186, B: 202, A: 255}
)

const (
	// Description settings
	maxDescriptionLength = 120
	descriptionTruncate  = 117
)

var ErrUserNotFound = errors.New("user not found")

// blendWithBackground creates a pseudo-transparent color by blending the given color with the background
// alpha should be between 0.0 (fully background) and 1.0 (fully foreground color)
func blendWithBackground(fg color.RGBA, bg color.RGBA, alpha float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(bg.R)*(1-alpha) + float64(fg.R)*alpha),
		G: uint8(float64(bg.G)*(1-alpha) + float64(fg.G)*alpha),
		B: uint8(float64(bg.B)*(1-alpha) + float64(fg.B)*alpha),
		A: 255,
	}
}

// createResponsiveJoinText creates a text line for "Join [username]" that fits within the available width
// by reducing font size and truncating with ellipsis if necessary
// Returns the text object and the font size used
func createResponsiveJoinText(fontFamily *canvas.FontFamily, text string, availableWidth float64, textColor color.RGBA) *canvas.Text {
	fontSize := joinFontSize
	minFontSize := minJoinFontSize

	for fontSize >= minFontSize {
		// Try bold first, fall back to regular if bold fails
		face := fontFamily.Face(fontSize, textColor, canvas.FontBold, canvas.FontNormal)
		if face == nil {
			face = fontFamily.Face(fontSize, textColor, canvas.FontRegular, canvas.FontNormal)
		}

		if face != nil {
			// Measure actual text width
			textWidth := face.TextWidth(text)

			// Check if text fits with some margin
			if textWidth <= availableWidth {
				textObj := canvas.NewTextLine(face, text, canvas.Bottom)
				return textObj
			}
		}

		fontSize -= 2.0 // Reduce font size by 2px each iteration
	}

	// If we get here, even minimum size doesn't fit, so we need to truncate
	face := fontFamily.Face(minFontSize, textColor, canvas.FontBold, canvas.FontNormal)
	if face == nil {
		face = fontFamily.Face(minFontSize, textColor, canvas.FontRegular, canvas.FontNormal)
	}

	// Ensure we have a valid face before truncating
	if face == nil {
		// Absolute fallback - just return "Join"
		fallbackFace := fontFamily.Face(minFontSize, textColor, canvas.FontRegular, canvas.FontNormal)
		if fallbackFace == nil {
			return canvas.NewTextLine(nil, "Join", canvas.Bottom)
		}
		face = fallbackFace
	}

	// Try progressively shorter versions with ellipsis
	runes := []rune(text)
	for i := len(runes) - 1; i >= 7; i-- { // Keep at least "Join @" + one char
		truncatedText := string(runes[:i]) + "..."
		textWidth := face.TextWidth(truncatedText)
		if textWidth <= availableWidth {
			return canvas.NewTextLine(face, truncatedText, canvas.Bottom)
		}
	}

	// Final fallback - just "Join ..."
	return canvas.NewTextLine(face, "Join ...", canvas.Bottom)
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

	resp, err := aqhttp.Do(ctx, req)
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
	var userDID string

	// Set default fallbacks
	handle = username
	description = "Live streaming platform for creators and their communities."

	profileData, err := s.fetchUserProfile(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch profile, because %w", err)
	} else if profileData != nil {
		// Safely extract profile data with nil checks
		userDID = profileData.Did

		if profileData.Avatar != nil && *profileData.Avatar != "" {
			imageURL = *profileData.Avatar
		}

		if profileData.Handle != "" {
			handle = profileData.Handle
		}

		if profileData.Description != nil && *profileData.Description != "" {
			desc := *profileData.Description
			// runes are used to properly handle multi-byte characters
			runes := []rune(desc)
			if len(runes) > maxDescriptionLength {
				desc = string(runes[:descriptionTruncate]) + "..."
			}
			description = desc
		}
	} else {
		log.Warn(ctx, "received nil profile data, using fallbacks", "username", username)
	}

	// Fetch user's chat profile color
	var userColor = joinTextColor      // default
	var borderColor = imageBorderColor // default
	if userDID != "" {
		chatProfile, err := s.ATSync.Model.GetChatProfile(ctx, userDID)
		if err != nil {
			log.Warn(ctx, "failed to fetch chat profile", "did", userDID, "error", err)
			clr := model.DefaultColors[hashString(userDID)%len(model.DefaultColors)]
			userColor = color.RGBA{
				R: uint8(clr.Red),
				G: uint8(clr.Green),
				B: uint8(clr.Blue),
				A: 255,
			}
		} else if chatProfile != nil {
			streamplaceChatProfile, err := chatProfile.ToStreamplaceChatProfile()
			if err != nil {
				log.Warn(ctx, "failed to decode chat profile", "did", userDID, "error", err)
			} else if streamplaceChatProfile != nil && streamplaceChatProfile.Color != nil {
				userColor = color.RGBA{
					R: uint8(streamplaceChatProfile.Color.Red),
					G: uint8(streamplaceChatProfile.Color.Green),
					B: uint8(streamplaceChatProfile.Color.Blue),
					A: 255,
				}
				borderColor = userColor
				log.Debug(ctx, "using user's custom color", "did", userDID, "color", userColor)
			}
		}
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
	canvasCtx.SetFillColor(blendWithBackground(borderColor, cardColor, 0.04))
	canvasCtx.DrawPath(cardPadding, cardPadding, canvas.RoundedRectangle(cardWidth, cardHeight, cardRadius))
	canvasCtx.Fill()

	// border
	cardBorderTransparent := blendWithBackground(blendWithBackground(borderColor, color.RGBA{R: 180, G: 180, B: 180}, 0.3), bgColor, 0.3)
	canvasCtx.SetStrokeColor(cardBorderTransparent)
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
		canvasCtx.DrawPath(imageX, 55, canvas.RoundedRectangle(100, 120, 8))
		canvasCtx.Fill()

		imageFace := fontAHN.Face(placeholderFontSize, placeholderTextColor, canvas.FontBold, canvas.FontNormal)
		imageText := canvas.NewTextBox(imageFace, "Streamplace", 100, 30, canvas.Center, canvas.Center, &canvas.TextOptions{})
		canvasCtx.DrawText(imageX, 105, imageText)
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

		// Add circular border with user's color (50% opacity for subtle effect)
		avatarCenterX := imageX + avatarDisplaySize/2
		avatarCenterY := imageY + avatarDisplaySize/2
		avatarBorderTransparent := blendWithBackground(blendWithBackground(borderColor, color.RGBA{R: 180, G: 180, B: 180}, 0.5), bgColor, 0.5)
		canvasCtx.SetStrokeColor(avatarBorderTransparent)
		canvasCtx.SetStrokeWidth(1)
		canvasCtx.DrawPath(avatarCenterX, avatarCenterY, canvas.Circle(avatarDisplaySize/2))
		canvasCtx.Stroke()

		canvasCtx.DrawImage(imageX, imageY, maskedAvatar, canvas.DPMM(canvasDPMM))
	}

	joinUserContent := fmt.Sprintf("Join @%s", handle)

	availableWidth := textAvailableWidth
	joinText := createResponsiveJoinText(fontAHN, joinUserContent, availableWidth, userColor)
	canvasCtx.DrawText(textStartX, joinY-(joinFontSize*0.5), joinText)

	// Add "streaming on Stream.place" subtitle
	onFace := fontAHN.Face(subtitleFontSize, blendWithBackground(borderColor, subtitleColor, 0.2), canvas.FontRegular, canvas.FontNormal)
	onText := canvas.NewTextBox(onFace, "streaming on Stream.place", 250, 30, canvas.Left, canvas.Center, &canvas.TextOptions{})
	canvasCtx.DrawText(textStartX, subtitleY, onText)

	// Add user description or promotional text
	descFace := fontAHN.Face(descFontSize, blendWithBackground(borderColor, descColor, 0.2), canvas.FontRegular, canvas.FontNormal)
	descText := canvas.NewTextBox(descFace, description, 230, 30, canvas.Left, canvas.Center, &canvas.TextOptions{})
	canvasCtx.DrawText(textStartX, descY, descText)

	b := &bytes.Buffer{}
	if err := c.Write(b, renderers.JPEG(canvas.DPMM(canvasDPMM))); err != nil {
		return nil, fmt.Errorf("failed to render canvas to buffer: %w", err)
	}

	return b.Bytes(), nil
}

func hashString(s string) int {
	h := fnv.New32a()
	h.Write([]byte(s))
	return int(h.Sum32())
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
