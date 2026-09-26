// Package branding is the vocabulary of a node's branding: every key an
// operator can set, its kind, its default and its limits. The XRPC handlers,
// the link-card injector, the bundle export/import and the CLI all read
// from here so a key added once is understood everywhere.
package branding

import (
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"slices"
	"strings"
)

// Kind is how a key's value is interpreted.
type Kind int

const (
	// KindText: a short free-text value (title, description, handle).
	KindText Kind = iota
	// KindColor: a hex color, #rgb or #rrggbb, stored lowercase.
	KindColor
	// KindEnum: one of a fixed set of strings.
	KindEnum
	// KindJSON: a JSON document (links, navigation).
	KindJSON
	// KindImage: a binary image asset.
	KindImage
)

// Scope is who reads a key.
type Scope int

const (
	// ScopeRuntime: served by the node and applied by the running app; an
	// operator changes it after the fact from the branding admin.
	ScopeRuntime Scope = iota
	// ScopeApp: read when an app is built (js/brand/generate.mjs): native
	// icons, splash, the app's name and identity. The node stores and
	// round-trips these so one brand directory, bundle or record carries a
	// whole brand, but never serves them to the running app.
	ScopeApp
)

// Spec describes one branding key.
type Spec struct {
	Key     string
	Kind    Kind
	Scope   Scope
	Default string   // for text-valued kinds; "" when unset means the app default
	Enum    []string // for KindEnum
	MaxSize int      // bytes
	// Doc is the one-line description shown in exported bundles.
	Doc string
}

// TextMime is the MIME type every text-valued key is stored with.
const TextMime = "text/plain"

const (
	textMax  = 1024
	jsonMax  = 16 * 1024
	imageMax = 500 * 1024
	iconMax  = 64 * 1024
)

// Specs lists every known key in export order.
var Specs = []Spec{
	{Key: "siteTitle", Kind: KindText, MaxSize: textMax, Doc: "Name of the node, shown in the nav lockup, the browser tab and link cards."},
	{Key: "siteDescription", Kind: KindText, MaxSize: textMax, Doc: "One-line description for link cards and the page meta description."},
	{Key: "networkName", Kind: KindText, Default: "Bluesky", MaxSize: textMax, Doc: "What the app calls the social network viewers sign in with and share to."},
	{Key: "networkProfileUrl", Kind: KindText, MaxSize: textMax, Doc: "Where a chat user's profile link goes, with {handle} and {did} placeholders; default https://bsky.app/profile/{handle}."},
	{Key: "cardSiteName", Kind: KindText, MaxSize: textMax, Doc: "How link cards name the site in their titles: the {site} in the card templates below. Default siteTitle."},
	{Key: "cardLiveTitle", Kind: KindText, Default: "{name} is live on {site}", MaxSize: textMax, Doc: "Link-card title of a live stream page; {name} is the streamer's display name (their handle when unset), {handle} their handle, {site} cardSiteName. The description is the stream's post text."},
	{Key: "cardHomeTitle", Kind: KindText, MaxSize: textMax, Doc: "Link-card title of the front page (the live tab), e.g. \"Live streams on {site}\". Default siteTitle."},
	{Key: "cardHomeDescription", Kind: KindText, MaxSize: textMax, Doc: "Link-card description of the front page, e.g. \"Watch live streams from verified accounts on {site}.\" Default siteDescription."},
	{Key: "cardVideosTitle", Kind: KindText, Default: "Videos on {site}", MaxSize: textMax, Doc: "Link-card title of the on-demand videos tab (/video)."},
	{Key: "cardVideosDescription", Kind: KindText, Default: "Watch replays of live streams on {site}.", MaxSize: textMax, Doc: "Link-card description of the on-demand videos tab."},
	{Key: "cardVideoTitle", Kind: KindText, Default: "{name}'s video on {site}", MaxSize: textMax, Doc: "Link-card title of a video page; {name}, {handle} and {site} as in cardLiveTitle. The description is the video's title."},
	{Key: "cardGoLiveTitle", Kind: KindText, Default: "Go live on {site}", MaxSize: textMax, Doc: "Link-card title of the go-live tab (/live)."},
	{Key: "cardGoLiveDescription", Kind: KindText, Default: "Set up and start your live stream on {site}.", MaxSize: textMax, Doc: "Link-card description of the go-live tab."},
	{Key: "loginPlaceholder", Kind: KindText, MaxSize: textMax, Doc: "Example handle shown in the login form's empty handle field."},
	{Key: "defaultStreamer", Kind: KindText, MaxSize: textMax, Doc: "Handle or DID whose stream page is the node's front door (single-user nodes)."},
	{Key: "defaultVideo", Kind: KindText, MaxSize: textMax, Doc: "A video whose page is the node's front door while set, over defaultStreamer: its at:// URI (at://did/place.stream.video/rkey) or its path (<handle-or-did>/video/<rkey>). Clear it to return the front door to the streamer."},
	{Key: "primaryColor", Kind: KindColor, Default: "#6366f1", MaxSize: textMax, Doc: "Buttons, links, focus rings."},
	{Key: "accentColor", Kind: KindColor, Default: "#8b5cf6", MaxSize: textMax, Doc: "Secondary surfaces and highlights."},
	{Key: "accentColorLight", Kind: KindColor, MaxSize: textMax, Doc: "accentColor override for the light scheme."},
	{Key: "backgroundColor", Kind: KindColor, MaxSize: textMax, Doc: "Page background (dark mode); raised surfaces and borders derive from it."},
	{Key: "foregroundColor", Kind: KindColor, MaxSize: textMax, Doc: "Text color (dark mode); unset picks white or near-black to read on the background."},
	{Key: "backgroundColorLight", Kind: KindColor, MaxSize: textMax, Doc: "Page background (light mode)."},
	{Key: "foregroundColorLight", Kind: KindColor, MaxSize: textMax, Doc: "Text color (light mode)."},
	{Key: "dangerColor", Kind: KindColor, MaxSize: textMax, Doc: "Errors and destructive actions."},
	{Key: "dangerColorLight", Kind: KindColor, MaxSize: textMax, Doc: "dangerColor override for the light scheme."},
	{Key: "successColor", Kind: KindColor, MaxSize: textMax, Doc: "Success states."},
	{Key: "successColorLight", Kind: KindColor, MaxSize: textMax, Doc: "successColor override for the light scheme."},
	{Key: "warningColor", Kind: KindColor, MaxSize: textMax, Doc: "Warnings."},
	{Key: "warningColorLight", Kind: KindColor, MaxSize: textMax, Doc: "warningColor override for the light scheme."},
	{Key: "infoColor", Kind: KindColor, MaxSize: textMax, Doc: "Informational notes."},
	{Key: "infoColorLight", Kind: KindColor, MaxSize: textMax, Doc: "infoColor override for the light scheme."},
	{Key: "liveColor", Kind: KindColor, MaxSize: textMax, Doc: "The live badge."},
	{Key: "chatBadges", Kind: KindEnum, Enum: []string{"all", "custom", "none"}, MaxSize: textMax, Doc: "Badges beside chat names: all, only badges issued by badge definitions (no built-in streamer / moderator / bot marks), or none."},
	{Key: "chatNameColors", Kind: KindEnum, Enum: []string{"on", "off"}, MaxSize: textMax, Doc: "Whether chat shows display names in each user's chosen color (on) or the default text color (off)."},
	{Key: "chatLayout", Kind: KindEnum, Enum: []string{"compact", "avatar"}, MaxSize: textMax, Doc: "Chat message shape: compact one-line rows, or avatar rows with name, handle and time above the text."},
	{Key: "navLinks", Kind: KindJSON, MaxSize: jsonMax, Doc: "Navigation links replacing the sidebar sections: [{label, url, icon}]."},
	{Key: "navCta", Kind: KindJSON, MaxSize: jsonMax, Doc: "Highlighted button under the navigation links: {label, url}."},
	{Key: "legalLinks", Kind: KindJSON, MaxSize: jsonMax, Doc: "Footer legal links: [{text, url}]."},
	{Key: "mobileAppBanner", Kind: KindEnum, Enum: []string{"on", "off"}, MaxSize: textMax, Doc: "Whether the web app suggests the Streamplace mobile app on phones: the in-page banner and the iOS Smart App Banner meta tag. Default on; off for a node that is its own product."},
	{Key: "socialHeading", Kind: KindText, Default: "Say Hello?", MaxSize: textMax, Doc: "Heading above the social links at the bottom of the sidebar."},
	{Key: "socialLinks", Kind: KindJSON, MaxSize: jsonMax, Doc: "Social links at the bottom of the sidebar: [{label, url, icon}], icon a built-in name (bluesky, discord, ...) or socialIcon1..4; [] hides them."},
	{Key: "mainLogo", Kind: KindImage, MaxSize: imageMax, Doc: "The logo mark (SVG preferred)."},
	{Key: "favicon", Kind: KindImage, MaxSize: 100 * 1024, Doc: "Browser tab icon."},
	{Key: "sidebarBackgroundImage", Kind: KindImage, MaxSize: imageMax, Doc: "Decorative image at the bottom of the sidebar."},
	{Key: "linkBanner", Kind: KindImage, MaxSize: 2 * 1024 * 1024, Doc: "OpenGraph image for the front page's link card (1200x630)."},
	{Key: "verifiedIcon", Kind: KindImage, MaxSize: iconMax, Doc: "Badge shown beside verified chat users (SVG preferred); default a check in the primary color."},
	{Key: "networkIcon", Kind: KindImage, MaxSize: iconMax, Doc: "Icon on chat profile links to the network (SVG preferred); default the Bluesky butterfly."},
	{Key: "socialIcon1", Kind: KindImage, MaxSize: iconMax, Doc: "Custom icon for socialLinks (SVG preferred; use currentColor to take the sidebar's icon color)."},
	{Key: "socialIcon2", Kind: KindImage, MaxSize: iconMax, Doc: "Custom icon for socialLinks (SVG preferred; use currentColor to take the sidebar's icon color)."},
	{Key: "socialIcon3", Kind: KindImage, MaxSize: iconMax, Doc: "Custom icon for socialLinks (SVG preferred; use currentColor to take the sidebar's icon color)."},
	{Key: "socialIcon4", Kind: KindImage, MaxSize: iconMax, Doc: "Custom icon for socialLinks (SVG preferred; use currentColor to take the sidebar's icon color)."},

	// Build-time keys. A build also reads mainLogo (the mark every icon is
	// drawn from; it must be an SVG with a viewBox), linkBanner and siteTitle.
	{Key: "appName", Kind: KindText, Scope: ScopeApp, MaxSize: textMax, Doc: "Product name of the built apps: the iOS/Android home-screen name, the desktop app, the docs. Default siteTitle."},
	{Key: "appWordmark", Kind: KindText, Scope: ScopeApp, MaxSize: textMax, Doc: "Text beside the mark in the app's lockup; a . in it gets the accent treatment. Default appName."},
	{Key: "appMonochrome", Kind: KindEnum, Scope: ScopeApp, Enum: []string{"on", "off"}, MaxSize: textMax, Doc: "on when mainLogo is single-color art drawn with fill=\"currentColor\", so the app can tint it. Default off."},
	{Key: "appColors", Kind: KindJSON, Scope: ScopeApp, MaxSize: jsonMax, Doc: "Icon and splash colors: {ink, paper, iconBackground, iconForeground, adaptiveIconBackground, adaptiveIconForeground, splashBackground, splashForeground, tileBackground, tileForeground, tileHairline, bannerBackground, bannerForeground}, all optional."},
	{Key: "appBundleId", Kind: KindText, Scope: ScopeApp, MaxSize: textMax, Doc: "iOS bundle identifier and Android package of the built app, reverse-DNS (com.example.live). SP_BUNDLE_OVERRIDE wins over it."},
	{Key: "appHost", Kind: KindText, Scope: ScopeApp, MaxSize: textMax, Doc: "Hostname of the node the built app talks to by default, and the domain its universal/app links open (live.example.com). Default stream.place."},
	{Key: "appStory", Kind: KindJSON, Scope: ScopeApp, MaxSize: jsonMax, Doc: "The mark's design story for the /brand guidelines page: {tagline, readingsIntro, readings, geometry, constructionNotes, specs, usage}."},
	{Key: "appIcon", Kind: KindImage, Scope: ScopeApp, MaxSize: imageMax, Doc: "Full-bleed square app icon (SVG or 1024px PNG). Default: mainLogo at 62% on appColors.iconBackground."},
	{Key: "appIconForeground", Kind: KindImage, Scope: ScopeApp, MaxSize: imageMax, Doc: "Android adaptive icon foreground; keep the art in the inner ~66%. Default: mainLogo at 45%."},
	{Key: "appSplash", Kind: KindImage, Scope: ScopeApp, MaxSize: imageMax, Doc: "Splash screen logo, shown on appColors.splashBackground. Default: mainLogo at 50%."},
	{Key: "appWordmarkImage", Kind: KindImage, Scope: ScopeApp, MaxSize: imageMax, Doc: "Wordmark lettering (SVG) for the downloadable lockup. Default: appWordmark set in type."},
}

var specByKey = func() map[string]Spec {
	m := map[string]Spec{}
	for _, s := range Specs {
		m[s.Key] = s
	}
	return m
}()

// Lookup returns the spec for key.
func Lookup(key string) (Spec, bool) {
	s, ok := specByKey[key]
	return s, ok
}

// IsText reports whether key holds a text-valued kind.
func IsText(key string) bool {
	s, ok := specByKey[key]
	return ok && s.Kind != KindImage
}

// Known reports whether key is in the vocabulary.
func Known(key string) bool {
	_, ok := specByKey[key]
	return ok
}

// IsRuntime reports whether the running app reads key; unknown keys count
// as runtime so a newer admin's keys still reach the app.
func IsRuntime(key string) bool {
	s, ok := specByKey[key]
	return !ok || s.Scope == ScopeRuntime
}

// IsImage reports whether key holds an image.
func IsImage(key string) bool {
	s, ok := specByKey[key]
	return ok && s.Kind == KindImage
}

// Defaults returns every text-valued runtime key and its default ("" when
// it has none), for the app's getBranding fallbacks.
func Defaults() map[string]string {
	m := map[string]string{}
	for _, s := range Specs {
		if s.Kind != KindImage && s.Scope == ScopeRuntime {
			m[s.Key] = s.Default
		}
	}
	return m
}

// hexColor accepts #rgb / #rrggbb, the only forms the app's theme accepts
// (it does string math on the value).
var hexColor = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// MaxSize returns the byte cap for key; unknown keys get the image cap so
// forward-compatible clients are not rejected outright.
func MaxSize(key string) int {
	if s, ok := specByKey[key]; ok {
		return s.MaxSize
	}
	return imageMax
}

// Normalize validates a text-valued key's value and returns the canonical
// form to store. An empty value is always valid (it means "unset").
func Normalize(key string, value []byte) ([]byte, error) {
	s, ok := specByKey[key]
	if !ok || s.Kind == KindImage {
		return value, nil
	}
	v := strings.TrimSpace(string(value))
	if v == "" {
		return []byte(""), nil
	}
	switch s.Kind {
	case KindColor:
		if !hexColor.MatchString(v) {
			return nil, fmt.Errorf("%s must be a hex color like #1a2b3c", key)
		}
		return []byte(strings.ToLower(v)), nil
	case KindEnum:
		if !slices.Contains(s.Enum, v) {
			return nil, fmt.Errorf("%s must be one of %s", key, strings.Join(s.Enum, ", "))
		}
		return []byte(v), nil
	case KindJSON:
		// Stored in canonical form (compact, keys sorted) so a value that
		// went through YAML and back compares equal to the original.
		canon, err := CanonicalJSON([]byte(v))
		if err != nil {
			return nil, fmt.Errorf("%s must be JSON", key)
		}
		if check, ok := jsonShapes[key]; ok {
			if err := check(canon); err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
		}
		return canon, nil
	}
	if check, ok := textShapes[key]; ok && !check.MatchString(v) {
		return nil, fmt.Errorf("%s must be %s", key, textShapeDocs[key])
	}
	return []byte(v), nil
}

// textShapes are the text keys a native build feeds to tooling that rejects
// anything else, so a bad value fails on import, not in Xcode.
var textShapes = map[string]*regexp.Regexp{
	"appBundleId": regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*(\.[A-Za-z][A-Za-z0-9_-]*)+$`),
	"appHost":     hostname,
}

var textShapeDocs = map[string]string{
	"appBundleId": "a reverse-DNS identifier like com.example.live",
	"appHost":     "a bare hostname like live.example.com",
}

// hostname is a DNS name (or IP literal) without scheme, port or path.
var hostname = regexp.MustCompile(`^(?:[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)*$`)

// NormalizeHostname lowercases a hostname and strips a port, returning ""
// when it is not a plausible DNS name.
func NormalizeHostname(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	if host, _, err := net.SplitHostPort(h); err == nil {
		h = host
	}
	h = strings.TrimSuffix(h, ".")
	if len(h) == 0 || len(h) > 253 || !hostname.MatchString(h) {
		return ""
	}
	return h
}

// jsonShapes is what each JSON key must look like; the app reads these
// without further checks, so a value of the wrong shape (an object where a
// list is documented) would crash it rather than be ignored.
var jsonShapes = map[string]func([]byte) error{
	"navLinks":    linkList("label", "url"),
	"socialLinks": linkList("label", "url"),
	"legalLinks":  linkList("text", "url"),
	"navCta":      linkObject("label", "url"),
	"appColors":   stringObject,
	"appStory":    anyObject,
}

// linkList accepts a JSON array of objects whose named fields are
// non-empty strings.
func linkList(fields ...string) func([]byte) error {
	return func(b []byte) error {
		var items []map[string]any
		if err := json.Unmarshal(b, &items); err != nil {
			return fmt.Errorf("must be a JSON list of objects")
		}
		for i, item := range items {
			if err := hasStringFields(item, fields); err != nil {
				return fmt.Errorf("item %d %w", i+1, err)
			}
		}
		return nil
	}
}

// linkObject accepts one JSON object whose named fields are non-empty
// strings.
func linkObject(fields ...string) func([]byte) error {
	return func(b []byte) error {
		var item map[string]any
		if err := json.Unmarshal(b, &item); err != nil {
			return fmt.Errorf("must be a JSON object")
		}
		return hasStringFields(item, fields)
	}
}

// stringObject accepts a JSON object whose values are all strings (or
// null, which means "use the default").
func stringObject(b []byte) error {
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err != nil {
		return fmt.Errorf("must be a JSON object")
	}
	for k, v := range obj {
		if _, ok := v.(string); !ok && v != nil {
			return fmt.Errorf("%s must be a string", k)
		}
	}
	return nil
}

func anyObject(b []byte) error {
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err != nil {
		return fmt.Errorf("must be a JSON object")
	}
	return nil
}

func hasStringFields(item map[string]any, fields []string) error {
	for _, f := range fields {
		if v, _ := item[f].(string); strings.TrimSpace(v) == "" {
			return fmt.Errorf("needs a %s", f)
		}
	}
	return nil
}

// CanonicalJSON re-serializes a JSON document compactly with object keys
// sorted (encoding/json sorts map keys), so equal documents are byte-equal.
func CanonicalJSON(b []byte) ([]byte, error) {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}
