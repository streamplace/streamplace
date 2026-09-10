// Package branding is the vocabulary of a node's branding: every key an
// operator can set, its kind, its default and its limits. The XRPC handlers,
// the link-card injector, the bundle export/import and the CLI all read
// from here so a key added once is understood everywhere.
package branding

import (
	"encoding/json"
	"fmt"
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

// Spec describes one branding key.
type Spec struct {
	Key     string
	Kind    Kind
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
	{Key: "loginMode", Kind: KindEnum, Enum: []string{"oauth", "pds"}, MaxSize: textMax, Doc: "How viewers sign in: the node's OAuth flow (oauth), or directly against a PDS with email / app password and its QR login (pds)."},
	{Key: "loginPdsUrl", Kind: KindText, MaxSize: textMax, Doc: "The PDS the pds login mode signs in against, e.g. https://pds.example.com."},
	{Key: "quickLogin", Kind: KindEnum, Enum: []string{"on", "off"}, MaxSize: textMax, Doc: "Offer the PDS's QR / app login (eu.wsocial.quicklogin) beside email and password (pds login mode)."},
	{Key: "loginForgotUrl", Kind: KindText, MaxSize: textMax, Doc: "Where \"Forgot password?\" on the sign-in screen goes (pds login mode)."},
	{Key: "loginWaitlistUrl", Kind: KindText, MaxSize: textMax, Doc: "Where the sign-in screen sends people without an account (pds login mode)."},
	{Key: "loginSupportEmail", Kind: KindText, MaxSize: textMax, Doc: "Support address linked from the sign-in screen (pds login mode)."},
	{Key: "sessionBrokerOrigin", Kind: KindText, MaxSize: textMax, Doc: "Origin of a sibling app whose /session-broker hands this front end its login (web only), e.g. https://example.com."},
	{Key: "networkProfileUrl", Kind: KindText, MaxSize: textMax, Doc: "Where a chat user's profile link goes, with {handle} and {did} placeholders; default https://bsky.app/profile/{handle}."},
	{Key: "loginPlaceholder", Kind: KindText, MaxSize: textMax, Doc: "Example handle shown in the login form's empty handle field."},
	{Key: "defaultStreamer", Kind: KindText, MaxSize: textMax, Doc: "Handle or DID whose stream page is the node's front door (single-user nodes)."},
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
	{Key: "appLayout", Kind: KindEnum, Enum: []string{"classic", "social"}, MaxSize: textMax, Doc: "App shell: classic Streamplace chrome, or social (no top bar, avatar atop the nav rail, fixed-width feed column, timeline-style nav)."},
	{Key: "streamLayout", Kind: KindEnum, Enum: []string{"classic", "card"}, MaxSize: textMax, Doc: "Stream page shape: classic full-width player, or card (post in a feed column with live chat beside it)."},
	{Key: "typeface", Kind: KindEnum, Enum: []string{"geist", "inter"}, MaxSize: textMax, Doc: "Sans-serif typeface."},
	{Key: "verifierDids", Kind: KindJSON, MaxSize: jsonMax, Doc: "DIDs whose app.bsky.graph.verification records mark a chat user as verified: [\"did:plc:...\"]."},
	{Key: "labelerDid", Kind: KindText, MaxSize: textMax, Doc: "A labeler whose account labels mark users as verified here (see verifiedLabels): did:plc:..."},
	{Key: "verifiedLabels", Kind: KindText, MaxSize: textMax, Doc: "Label values from labelerDid that count as verified, comma-separated; a trailing * matches a prefix: \"wsocial-*\" or \"verified,org\"."},
	{Key: "verifyUrl", Kind: KindText, MaxSize: textMax, Doc: "Where a signed-in but unverified user goes to get verified (the composer's \"Verify now\" link); unset shows no link."},
	{Key: "chatVerifiedOnly", Kind: KindEnum, Enum: []string{"on", "off"}, MaxSize: textMax, Doc: "Only users verified by one of verifierDids may chat (their messages are the only ones shown)."},
	{Key: "chatBadges", Kind: KindEnum, Enum: []string{"all", "custom", "none"}, MaxSize: textMax, Doc: "Badges beside chat names: all, only badges issued by badge definitions (no built-in streamer / moderator / bot marks), or none."},
	{Key: "chatNameColors", Kind: KindEnum, Enum: []string{"on", "off"}, MaxSize: textMax, Doc: "Whether chat shows display names in each user's chosen color (on) or the default text color (off)."},
	{Key: "chatLayout", Kind: KindEnum, Enum: []string{"compact", "avatar"}, MaxSize: textMax, Doc: "Chat message shape: compact one-line rows, or avatar rows with name, handle and time above the text."},
	{Key: "navLinks", Kind: KindJSON, MaxSize: jsonMax, Doc: "Navigation links replacing the sidebar sections: [{label, url, icon}]."},
	{Key: "navCta", Kind: KindJSON, MaxSize: jsonMax, Doc: "Highlighted button under the navigation links: {label, url}."},
	{Key: "legalLinks", Kind: KindJSON, MaxSize: jsonMax, Doc: "Footer legal links: [{text, url}]."},
	{Key: "externalLinkTarget", Kind: KindEnum, Enum: []string{"newTab", "sameWindow"}, MaxSize: textMax, Doc: "Where the sidebar's external links (navLinks, navCta, bottomLinks, socialLinks) open on the web: a new tab (default) or the same window."},
	{Key: "showDownloadLink", Kind: KindEnum, Enum: []string{"on", "off"}, MaxSize: textMax, Doc: "Whether the sidebar shows the app Download link (default on for classic, off for social)."},
	{Key: "bottomLinks", Kind: KindJSON, MaxSize: jsonMax, Doc: "Links pinned above the social row at the bottom of the sidebar: [{label, url, icon}]; unset shows Documentation (classic only), [] hides them."},
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

// IsImage reports whether key holds an image.
func IsImage(key string) bool {
	s, ok := specByKey[key]
	return ok && s.Kind == KindImage
}

// Defaults returns every key with a non-empty default, for the app's
// getBranding fallbacks.
func Defaults() map[string]string {
	m := map[string]string{}
	for _, s := range Specs {
		if s.Kind != KindImage {
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
		return canon, nil
	}
	return []byte(v), nil
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
