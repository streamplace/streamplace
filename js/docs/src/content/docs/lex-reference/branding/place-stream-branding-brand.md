---
title: place.stream.branding.brand
description: Reference for the place.stream.branding.brand lexicon
---

**Lexicon Version:** 1

## Definitions

<a name="main"></a>

### `main`

**Type:** `record`

A brand: how a Streamplace node looks on one hostname, and how apps built for it look. Keyed by the hostname. Generated from pkg/branding/vocab.go; do not edit by hand.

**Record Key:** `any`

**Record Properties:**

| Name                     | Type                                | Req'd | Description                                                                                                                                                                                                                  | Constraints                                                                                                                                           |
| ------------------------ | ----------------------------------- | ----- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| `siteTitle`              | `string`                            | ❌    | Name of the node, shown in the nav lockup, the browser tab and link cards.                                                                                                                                                   | Max Length: 1024                                                                                                                                      |
| `siteDescription`        | `string`                            | ❌    | One-line description for link cards and the page meta description.                                                                                                                                                           | Max Length: 1024                                                                                                                                      |
| `networkName`            | `string`                            | ❌    | What the app calls the social network viewers sign in with and share to.                                                                                                                                                     | Max Length: 1024                                                                                                                                      |
| `networkProfileUrl`      | `string`                            | ❌    | Where a chat user's profile link goes, with {handle} and {did} placeholders; default https://bsky.app/profile/{handle}.                                                                                                      | Max Length: 1024                                                                                                                                      |
| `cardSiteName`           | `string`                            | ❌    | How link cards name the site in their titles: the {site} in the card templates below. Default siteTitle.                                                                                                                     | Max Length: 1024                                                                                                                                      |
| `cardLiveTitle`          | `string`                            | ❌    | Link-card title of a live stream page; {name} is the streamer's display name (their handle when unset), {handle} their handle, {site} cardSiteName. The description is the stream's post text.                               | Max Length: 1024                                                                                                                                      |
| `cardHomeTitle`          | `string`                            | ❌    | Link-card title of the front page (the live tab), e.g. "Live streams on {site}". Default siteTitle.                                                                                                                          | Max Length: 1024                                                                                                                                      |
| `cardHomeDescription`    | `string`                            | ❌    | Link-card description of the front page, e.g. "Watch live streams from verified accounts on {site}." Default siteDescription.                                                                                                | Max Length: 1024                                                                                                                                      |
| `cardVideosTitle`        | `string`                            | ❌    | Link-card title of the on-demand videos tab (/video).                                                                                                                                                                        | Max Length: 1024                                                                                                                                      |
| `cardVideosDescription`  | `string`                            | ❌    | Link-card description of the on-demand videos tab.                                                                                                                                                                           | Max Length: 1024                                                                                                                                      |
| `cardVideoTitle`         | `string`                            | ❌    | Link-card title of a video page; {name}, {handle} and {site} as in cardLiveTitle. The description is the video's title.                                                                                                      | Max Length: 1024                                                                                                                                      |
| `cardGoLiveTitle`        | `string`                            | ❌    | Link-card title of the go-live tab (/live).                                                                                                                                                                                  | Max Length: 1024                                                                                                                                      |
| `cardGoLiveDescription`  | `string`                            | ❌    | Link-card description of the go-live tab.                                                                                                                                                                                    | Max Length: 1024                                                                                                                                      |
| `loginPlaceholder`       | `string`                            | ❌    | Example handle shown in the login form's empty handle field.                                                                                                                                                                 | Max Length: 1024                                                                                                                                      |
| `defaultStreamer`        | `string`                            | ❌    | Handle or DID whose stream page is the node's front door (single-user nodes).                                                                                                                                                | Max Length: 1024                                                                                                                                      |
| `defaultVideo`           | `string`                            | ❌    | A video whose page is the node's front door while set, over defaultStreamer: its at:// URI (at://did/place.stream.video/rkey) or its path (<handle-or-did>/video/<rkey>). Clear it to return the front door to the streamer. | Max Length: 1024                                                                                                                                      |
| `primaryColor`           | `string`                            | ❌    | Buttons, links, focus rings.                                                                                                                                                                                                 | Max Length: 7                                                                                                                                         |
| `accentColor`            | `string`                            | ❌    | Secondary surfaces and highlights.                                                                                                                                                                                           | Max Length: 7                                                                                                                                         |
| `accentColorLight`       | `string`                            | ❌    | accentColor override for the light scheme.                                                                                                                                                                                   | Max Length: 7                                                                                                                                         |
| `backgroundColor`        | `string`                            | ❌    | Page background (dark mode); raised surfaces and borders derive from it.                                                                                                                                                     | Max Length: 7                                                                                                                                         |
| `foregroundColor`        | `string`                            | ❌    | Text color (dark mode); unset picks white or near-black to read on the background.                                                                                                                                           | Max Length: 7                                                                                                                                         |
| `backgroundColorLight`   | `string`                            | ❌    | Page background (light mode).                                                                                                                                                                                                | Max Length: 7                                                                                                                                         |
| `foregroundColorLight`   | `string`                            | ❌    | Text color (light mode).                                                                                                                                                                                                     | Max Length: 7                                                                                                                                         |
| `dangerColor`            | `string`                            | ❌    | Errors and destructive actions.                                                                                                                                                                                              | Max Length: 7                                                                                                                                         |
| `dangerColorLight`       | `string`                            | ❌    | dangerColor override for the light scheme.                                                                                                                                                                                   | Max Length: 7                                                                                                                                         |
| `successColor`           | `string`                            | ❌    | Success states.                                                                                                                                                                                                              | Max Length: 7                                                                                                                                         |
| `successColorLight`      | `string`                            | ❌    | successColor override for the light scheme.                                                                                                                                                                                  | Max Length: 7                                                                                                                                         |
| `warningColor`           | `string`                            | ❌    | Warnings.                                                                                                                                                                                                                    | Max Length: 7                                                                                                                                         |
| `warningColorLight`      | `string`                            | ❌    | warningColor override for the light scheme.                                                                                                                                                                                  | Max Length: 7                                                                                                                                         |
| `infoColor`              | `string`                            | ❌    | Informational notes.                                                                                                                                                                                                         | Max Length: 7                                                                                                                                         |
| `infoColorLight`         | `string`                            | ❌    | infoColor override for the light scheme.                                                                                                                                                                                     | Max Length: 7                                                                                                                                         |
| `liveColor`              | `string`                            | ❌    | The live badge.                                                                                                                                                                                                              | Max Length: 7                                                                                                                                         |
| `chatBadges`             | `string`                            | ❌    | Badges beside chat names: all, only badges issued by badge definitions (no built-in streamer / moderator / bot marks), or none.                                                                                              | Known Values: `all`, `custom`, `none`                                                                                                                 |
| `chatNameColors`         | `string`                            | ❌    | Whether chat shows display names in each user's chosen color (on) or the default text color (off).                                                                                                                           | Known Values: `on`, `off`                                                                                                                             |
| `chatLayout`             | `string`                            | ❌    | Chat message shape: compact one-line rows, or avatar rows with name, handle and time above the text.                                                                                                                         | Known Values: `compact`, `avatar`                                                                                                                     |
| `navLinks`               | Array of [`#link`](#link)           | ❌    |                                                                                                                                                                                                                              |                                                                                                                                                       |
| `navCta`                 | [`#link`](#link)                    | ❌    |                                                                                                                                                                                                                              |                                                                                                                                                       |
| `legalLinks`             | Array of [`#legalLink`](#legallink) | ❌    |                                                                                                                                                                                                                              |                                                                                                                                                       |
| `mobileAppBanner`        | `string`                            | ❌    | Whether the web app suggests the Streamplace mobile app on phones: the in-page banner and the iOS Smart App Banner meta tag. Default on; off for a node that is its own product.                                             | Known Values: `on`, `off`                                                                                                                             |
| `socialHeading`          | `string`                            | ❌    | Heading above the social links at the bottom of the sidebar.                                                                                                                                                                 | Max Length: 1024                                                                                                                                      |
| `socialLinks`            | Array of [`#link`](#link)           | ❌    |                                                                                                                                                                                                                              |                                                                                                                                                       |
| `mainLogo`               | `blob`                              | ❌    | The logo mark (SVG preferred).                                                                                                                                                                                               | Accept: `image/svg+xml`, `image/png`, `image/jpeg`, `image/webp`, `image/gif`, `image/x-icon`, `image/vnd.microsoft.icon`<br/>Max Size: 512000 bytes  |
| `favicon`                | `blob`                              | ❌    | Browser tab icon.                                                                                                                                                                                                            | Accept: `image/svg+xml`, `image/png`, `image/jpeg`, `image/webp`, `image/gif`, `image/x-icon`, `image/vnd.microsoft.icon`<br/>Max Size: 102400 bytes  |
| `sidebarBackgroundImage` | `blob`                              | ❌    | Decorative image at the bottom of the sidebar.                                                                                                                                                                               | Accept: `image/svg+xml`, `image/png`, `image/jpeg`, `image/webp`, `image/gif`, `image/x-icon`, `image/vnd.microsoft.icon`<br/>Max Size: 512000 bytes  |
| `linkBanner`             | `blob`                              | ❌    | OpenGraph image for the front page's link card (1200x630).                                                                                                                                                                   | Accept: `image/svg+xml`, `image/png`, `image/jpeg`, `image/webp`, `image/gif`, `image/x-icon`, `image/vnd.microsoft.icon`<br/>Max Size: 2097152 bytes |
| `verifiedIcon`           | `blob`                              | ❌    | Badge shown beside verified chat users (SVG preferred); default a check in the primary color.                                                                                                                                | Accept: `image/svg+xml`, `image/png`, `image/jpeg`, `image/webp`, `image/gif`, `image/x-icon`, `image/vnd.microsoft.icon`<br/>Max Size: 65536 bytes   |
| `networkIcon`            | `blob`                              | ❌    | Icon on chat profile links to the network (SVG preferred); default the Bluesky butterfly.                                                                                                                                    | Accept: `image/svg+xml`, `image/png`, `image/jpeg`, `image/webp`, `image/gif`, `image/x-icon`, `image/vnd.microsoft.icon`<br/>Max Size: 65536 bytes   |
| `socialIcon1`            | `blob`                              | ❌    | Custom icon for socialLinks (SVG preferred; use currentColor to take the sidebar's icon color).                                                                                                                              | Accept: `image/svg+xml`, `image/png`, `image/jpeg`, `image/webp`, `image/gif`, `image/x-icon`, `image/vnd.microsoft.icon`<br/>Max Size: 65536 bytes   |
| `socialIcon2`            | `blob`                              | ❌    | Custom icon for socialLinks (SVG preferred; use currentColor to take the sidebar's icon color).                                                                                                                              | Accept: `image/svg+xml`, `image/png`, `image/jpeg`, `image/webp`, `image/gif`, `image/x-icon`, `image/vnd.microsoft.icon`<br/>Max Size: 65536 bytes   |
| `socialIcon3`            | `blob`                              | ❌    | Custom icon for socialLinks (SVG preferred; use currentColor to take the sidebar's icon color).                                                                                                                              | Accept: `image/svg+xml`, `image/png`, `image/jpeg`, `image/webp`, `image/gif`, `image/x-icon`, `image/vnd.microsoft.icon`<br/>Max Size: 65536 bytes   |
| `socialIcon4`            | `blob`                              | ❌    | Custom icon for socialLinks (SVG preferred; use currentColor to take the sidebar's icon color).                                                                                                                              | Accept: `image/svg+xml`, `image/png`, `image/jpeg`, `image/webp`, `image/gif`, `image/x-icon`, `image/vnd.microsoft.icon`<br/>Max Size: 65536 bytes   |
| `appName`                | `string`                            | ❌    | Product name of the built apps: the iOS/Android home-screen name, the desktop app, the docs. Default siteTitle.                                                                                                              | Max Length: 1024                                                                                                                                      |
| `appWordmark`            | `string`                            | ❌    | Text beside the mark in the app's lockup; a . in it gets the accent treatment. Default appName.                                                                                                                              | Max Length: 1024                                                                                                                                      |
| `appMonochrome`          | `string`                            | ❌    | on when mainLogo is single-color art drawn with fill="currentColor", so the app can tint it. Default off.                                                                                                                    | Known Values: `on`, `off`                                                                                                                             |
| `appColors`              | [`#appColors`](#appcolors)          | ❌    |                                                                                                                                                                                                                              |                                                                                                                                                       |
| `appBundleId`            | `string`                            | ❌    | iOS bundle identifier and Android package of the built app, reverse-DNS (com.example.live). SP_BUNDLE_OVERRIDE wins over it.                                                                                                 | Max Length: 1024                                                                                                                                      |
| `appHost`                | `string`                            | ❌    | Hostname of the node the built app talks to by default, and the domain its universal/app links open (live.example.com). Default stream.place.                                                                                | Max Length: 1024                                                                                                                                      |
| `appStory`               | `string`                            | ❌    | The mark's design story for the /brand guidelines page: {tagline, readingsIntro, readings, geometry, constructionNotes, specs, usage}.                                                                                       | Max Length: 16384                                                                                                                                     |
| `appIcon`                | `blob`                              | ❌    | Full-bleed square app icon (SVG or 1024px PNG). Default: mainLogo at 62% on appColors.iconBackground.                                                                                                                        | Accept: `image/svg+xml`, `image/png`, `image/jpeg`, `image/webp`, `image/gif`, `image/x-icon`, `image/vnd.microsoft.icon`<br/>Max Size: 512000 bytes  |
| `appIconForeground`      | `blob`                              | ❌    | Android adaptive icon foreground; keep the art in the inner ~66%. Default: mainLogo at 45%.                                                                                                                                  | Accept: `image/svg+xml`, `image/png`, `image/jpeg`, `image/webp`, `image/gif`, `image/x-icon`, `image/vnd.microsoft.icon`<br/>Max Size: 512000 bytes  |
| `appSplash`              | `blob`                              | ❌    | Splash screen logo, shown on appColors.splashBackground. Default: mainLogo at 50%.                                                                                                                                           | Accept: `image/svg+xml`, `image/png`, `image/jpeg`, `image/webp`, `image/gif`, `image/x-icon`, `image/vnd.microsoft.icon`<br/>Max Size: 512000 bytes  |
| `appWordmarkImage`       | `blob`                              | ❌    | Wordmark lettering (SVG) for the downloadable lockup. Default: appWordmark set in type.                                                                                                                                      | Accept: `image/svg+xml`, `image/png`, `image/jpeg`, `image/webp`, `image/gif`, `image/x-icon`, `image/vnd.microsoft.icon`<br/>Max Size: 512000 bytes  |

---

<a name="link"></a>

### `link`

**Type:** `object`

**Properties:**

| Name    | Type     | Req'd | Description                                              | Constraints      |
| ------- | -------- | ----- | -------------------------------------------------------- | ---------------- |
| `label` | `string` | ✅    |                                                          | Max Length: 256  |
| `url`   | `string` | ✅    |                                                          | Max Length: 2048 |
| `icon`  | `string` | ❌    | A built-in icon name or, in socialLinks, socialIcon1..4. | Max Length: 256  |

---

<a name="legallink"></a>

### `legalLink`

**Type:** `object`

**Properties:**

| Name   | Type     | Req'd | Description | Constraints      |
| ------ | -------- | ----- | ----------- | ---------------- |
| `text` | `string` | ✅    |             | Max Length: 256  |
| `url`  | `string` | ✅    |             | Max Length: 2048 |

---

<a name="appcolors"></a>

### `appColors`

**Type:** `object`

Icon and splash colors of a built app; any CSS color.

**Properties:**

| Name                     | Type     | Req'd | Description | Constraints    |
| ------------------------ | -------- | ----- | ----------- | -------------- |
| `ink`                    | `string` | ❌    |             | Max Length: 64 |
| `paper`                  | `string` | ❌    |             | Max Length: 64 |
| `iconBackground`         | `string` | ❌    |             | Max Length: 64 |
| `iconForeground`         | `string` | ❌    |             | Max Length: 64 |
| `adaptiveIconBackground` | `string` | ❌    |             | Max Length: 64 |
| `adaptiveIconForeground` | `string` | ❌    |             | Max Length: 64 |
| `splashBackground`       | `string` | ❌    |             | Max Length: 64 |
| `splashForeground`       | `string` | ❌    |             | Max Length: 64 |
| `tileBackground`         | `string` | ❌    |             | Max Length: 64 |
| `tileForeground`         | `string` | ❌    |             | Max Length: 64 |
| `tileHairline`           | `string` | ❌    |             | Max Length: 64 |
| `bannerBackground`       | `string` | ❌    |             | Max Length: 64 |
| `bannerForeground`       | `string` | ❌    |             | Max Length: 64 |

---

## Lexicon Source

```json
{
  "lexicon": 1,
  "id": "place.stream.branding.brand",
  "defs": {
    "main": {
      "type": "record",
      "description": "A brand: how a Streamplace node looks on one hostname, and how apps built for it look. Keyed by the hostname. Generated from pkg/branding/vocab.go; do not edit by hand.",
      "key": "any",
      "record": {
        "type": "object",
        "required": [],
        "properties": {
          "siteTitle": {
            "type": "string",
            "maxLength": 1024,
            "description": "Name of the node, shown in the nav lockup, the browser tab and link cards."
          },
          "siteDescription": {
            "type": "string",
            "maxLength": 1024,
            "description": "One-line description for link cards and the page meta description."
          },
          "networkName": {
            "type": "string",
            "maxLength": 1024,
            "description": "What the app calls the social network viewers sign in with and share to."
          },
          "networkProfileUrl": {
            "type": "string",
            "maxLength": 1024,
            "description": "Where a chat user's profile link goes, with {handle} and {did} placeholders; default https://bsky.app/profile/{handle}."
          },
          "cardSiteName": {
            "type": "string",
            "maxLength": 1024,
            "description": "How link cards name the site in their titles: the {site} in the card templates below. Default siteTitle."
          },
          "cardLiveTitle": {
            "type": "string",
            "maxLength": 1024,
            "description": "Link-card title of a live stream page; {name} is the streamer's display name (their handle when unset), {handle} their handle, {site} cardSiteName. The description is the stream's post text."
          },
          "cardHomeTitle": {
            "type": "string",
            "maxLength": 1024,
            "description": "Link-card title of the front page (the live tab), e.g. \"Live streams on {site}\". Default siteTitle."
          },
          "cardHomeDescription": {
            "type": "string",
            "maxLength": 1024,
            "description": "Link-card description of the front page, e.g. \"Watch live streams from verified accounts on {site}.\" Default siteDescription."
          },
          "cardVideosTitle": {
            "type": "string",
            "maxLength": 1024,
            "description": "Link-card title of the on-demand videos tab (/video)."
          },
          "cardVideosDescription": {
            "type": "string",
            "maxLength": 1024,
            "description": "Link-card description of the on-demand videos tab."
          },
          "cardVideoTitle": {
            "type": "string",
            "maxLength": 1024,
            "description": "Link-card title of a video page; {name}, {handle} and {site} as in cardLiveTitle. The description is the video's title."
          },
          "cardGoLiveTitle": {
            "type": "string",
            "maxLength": 1024,
            "description": "Link-card title of the go-live tab (/live)."
          },
          "cardGoLiveDescription": {
            "type": "string",
            "maxLength": 1024,
            "description": "Link-card description of the go-live tab."
          },
          "loginPlaceholder": {
            "type": "string",
            "maxLength": 1024,
            "description": "Example handle shown in the login form's empty handle field."
          },
          "defaultStreamer": {
            "type": "string",
            "maxLength": 1024,
            "description": "Handle or DID whose stream page is the node's front door (single-user nodes)."
          },
          "defaultVideo": {
            "type": "string",
            "maxLength": 1024,
            "description": "A video whose page is the node's front door while set, over defaultStreamer: its at:// URI (at://did/place.stream.video/rkey) or its path (<handle-or-did>/video/<rkey>). Clear it to return the front door to the streamer."
          },
          "primaryColor": {
            "type": "string",
            "maxLength": 7,
            "description": "Buttons, links, focus rings."
          },
          "accentColor": {
            "type": "string",
            "maxLength": 7,
            "description": "Secondary surfaces and highlights."
          },
          "accentColorLight": {
            "type": "string",
            "maxLength": 7,
            "description": "accentColor override for the light scheme."
          },
          "backgroundColor": {
            "type": "string",
            "maxLength": 7,
            "description": "Page background (dark mode); raised surfaces and borders derive from it."
          },
          "foregroundColor": {
            "type": "string",
            "maxLength": 7,
            "description": "Text color (dark mode); unset picks white or near-black to read on the background."
          },
          "backgroundColorLight": {
            "type": "string",
            "maxLength": 7,
            "description": "Page background (light mode)."
          },
          "foregroundColorLight": {
            "type": "string",
            "maxLength": 7,
            "description": "Text color (light mode)."
          },
          "dangerColor": {
            "type": "string",
            "maxLength": 7,
            "description": "Errors and destructive actions."
          },
          "dangerColorLight": {
            "type": "string",
            "maxLength": 7,
            "description": "dangerColor override for the light scheme."
          },
          "successColor": {
            "type": "string",
            "maxLength": 7,
            "description": "Success states."
          },
          "successColorLight": {
            "type": "string",
            "maxLength": 7,
            "description": "successColor override for the light scheme."
          },
          "warningColor": {
            "type": "string",
            "maxLength": 7,
            "description": "Warnings."
          },
          "warningColorLight": {
            "type": "string",
            "maxLength": 7,
            "description": "warningColor override for the light scheme."
          },
          "infoColor": {
            "type": "string",
            "maxLength": 7,
            "description": "Informational notes."
          },
          "infoColorLight": {
            "type": "string",
            "maxLength": 7,
            "description": "infoColor override for the light scheme."
          },
          "liveColor": {
            "type": "string",
            "maxLength": 7,
            "description": "The live badge."
          },
          "chatBadges": {
            "type": "string",
            "knownValues": ["all", "custom", "none"],
            "description": "Badges beside chat names: all, only badges issued by badge definitions (no built-in streamer / moderator / bot marks), or none."
          },
          "chatNameColors": {
            "type": "string",
            "knownValues": ["on", "off"],
            "description": "Whether chat shows display names in each user's chosen color (on) or the default text color (off)."
          },
          "chatLayout": {
            "type": "string",
            "knownValues": ["compact", "avatar"],
            "description": "Chat message shape: compact one-line rows, or avatar rows with name, handle and time above the text."
          },
          "navLinks": {
            "type": "array",
            "items": {
              "type": "ref",
              "ref": "#link"
            }
          },
          "navCta": {
            "type": "ref",
            "ref": "#link"
          },
          "legalLinks": {
            "type": "array",
            "items": {
              "type": "ref",
              "ref": "#legalLink"
            }
          },
          "mobileAppBanner": {
            "type": "string",
            "knownValues": ["on", "off"],
            "description": "Whether the web app suggests the Streamplace mobile app on phones: the in-page banner and the iOS Smart App Banner meta tag. Default on; off for a node that is its own product."
          },
          "socialHeading": {
            "type": "string",
            "maxLength": 1024,
            "description": "Heading above the social links at the bottom of the sidebar."
          },
          "socialLinks": {
            "type": "array",
            "items": {
              "type": "ref",
              "ref": "#link"
            }
          },
          "mainLogo": {
            "type": "blob",
            "accept": [
              "image/svg+xml",
              "image/png",
              "image/jpeg",
              "image/webp",
              "image/gif",
              "image/x-icon",
              "image/vnd.microsoft.icon"
            ],
            "maxSize": 512000,
            "description": "The logo mark (SVG preferred)."
          },
          "favicon": {
            "type": "blob",
            "accept": [
              "image/svg+xml",
              "image/png",
              "image/jpeg",
              "image/webp",
              "image/gif",
              "image/x-icon",
              "image/vnd.microsoft.icon"
            ],
            "maxSize": 102400,
            "description": "Browser tab icon."
          },
          "sidebarBackgroundImage": {
            "type": "blob",
            "accept": [
              "image/svg+xml",
              "image/png",
              "image/jpeg",
              "image/webp",
              "image/gif",
              "image/x-icon",
              "image/vnd.microsoft.icon"
            ],
            "maxSize": 512000,
            "description": "Decorative image at the bottom of the sidebar."
          },
          "linkBanner": {
            "type": "blob",
            "accept": [
              "image/svg+xml",
              "image/png",
              "image/jpeg",
              "image/webp",
              "image/gif",
              "image/x-icon",
              "image/vnd.microsoft.icon"
            ],
            "maxSize": 2097152,
            "description": "OpenGraph image for the front page's link card (1200x630)."
          },
          "verifiedIcon": {
            "type": "blob",
            "accept": [
              "image/svg+xml",
              "image/png",
              "image/jpeg",
              "image/webp",
              "image/gif",
              "image/x-icon",
              "image/vnd.microsoft.icon"
            ],
            "maxSize": 65536,
            "description": "Badge shown beside verified chat users (SVG preferred); default a check in the primary color."
          },
          "networkIcon": {
            "type": "blob",
            "accept": [
              "image/svg+xml",
              "image/png",
              "image/jpeg",
              "image/webp",
              "image/gif",
              "image/x-icon",
              "image/vnd.microsoft.icon"
            ],
            "maxSize": 65536,
            "description": "Icon on chat profile links to the network (SVG preferred); default the Bluesky butterfly."
          },
          "socialIcon1": {
            "type": "blob",
            "accept": [
              "image/svg+xml",
              "image/png",
              "image/jpeg",
              "image/webp",
              "image/gif",
              "image/x-icon",
              "image/vnd.microsoft.icon"
            ],
            "maxSize": 65536,
            "description": "Custom icon for socialLinks (SVG preferred; use currentColor to take the sidebar's icon color)."
          },
          "socialIcon2": {
            "type": "blob",
            "accept": [
              "image/svg+xml",
              "image/png",
              "image/jpeg",
              "image/webp",
              "image/gif",
              "image/x-icon",
              "image/vnd.microsoft.icon"
            ],
            "maxSize": 65536,
            "description": "Custom icon for socialLinks (SVG preferred; use currentColor to take the sidebar's icon color)."
          },
          "socialIcon3": {
            "type": "blob",
            "accept": [
              "image/svg+xml",
              "image/png",
              "image/jpeg",
              "image/webp",
              "image/gif",
              "image/x-icon",
              "image/vnd.microsoft.icon"
            ],
            "maxSize": 65536,
            "description": "Custom icon for socialLinks (SVG preferred; use currentColor to take the sidebar's icon color)."
          },
          "socialIcon4": {
            "type": "blob",
            "accept": [
              "image/svg+xml",
              "image/png",
              "image/jpeg",
              "image/webp",
              "image/gif",
              "image/x-icon",
              "image/vnd.microsoft.icon"
            ],
            "maxSize": 65536,
            "description": "Custom icon for socialLinks (SVG preferred; use currentColor to take the sidebar's icon color)."
          },
          "appName": {
            "type": "string",
            "maxLength": 1024,
            "description": "Product name of the built apps: the iOS/Android home-screen name, the desktop app, the docs. Default siteTitle."
          },
          "appWordmark": {
            "type": "string",
            "maxLength": 1024,
            "description": "Text beside the mark in the app's lockup; a . in it gets the accent treatment. Default appName."
          },
          "appMonochrome": {
            "type": "string",
            "knownValues": ["on", "off"],
            "description": "on when mainLogo is single-color art drawn with fill=\"currentColor\", so the app can tint it. Default off."
          },
          "appColors": {
            "type": "ref",
            "ref": "#appColors"
          },
          "appBundleId": {
            "type": "string",
            "maxLength": 1024,
            "description": "iOS bundle identifier and Android package of the built app, reverse-DNS (com.example.live). SP_BUNDLE_OVERRIDE wins over it."
          },
          "appHost": {
            "type": "string",
            "maxLength": 1024,
            "description": "Hostname of the node the built app talks to by default, and the domain its universal/app links open (live.example.com). Default stream.place."
          },
          "appStory": {
            "type": "string",
            "maxLength": 16384,
            "description": "The mark's design story for the /brand guidelines page: {tagline, readingsIntro, readings, geometry, constructionNotes, specs, usage}."
          },
          "appIcon": {
            "type": "blob",
            "accept": [
              "image/svg+xml",
              "image/png",
              "image/jpeg",
              "image/webp",
              "image/gif",
              "image/x-icon",
              "image/vnd.microsoft.icon"
            ],
            "maxSize": 512000,
            "description": "Full-bleed square app icon (SVG or 1024px PNG). Default: mainLogo at 62% on appColors.iconBackground."
          },
          "appIconForeground": {
            "type": "blob",
            "accept": [
              "image/svg+xml",
              "image/png",
              "image/jpeg",
              "image/webp",
              "image/gif",
              "image/x-icon",
              "image/vnd.microsoft.icon"
            ],
            "maxSize": 512000,
            "description": "Android adaptive icon foreground; keep the art in the inner ~66%. Default: mainLogo at 45%."
          },
          "appSplash": {
            "type": "blob",
            "accept": [
              "image/svg+xml",
              "image/png",
              "image/jpeg",
              "image/webp",
              "image/gif",
              "image/x-icon",
              "image/vnd.microsoft.icon"
            ],
            "maxSize": 512000,
            "description": "Splash screen logo, shown on appColors.splashBackground. Default: mainLogo at 50%."
          },
          "appWordmarkImage": {
            "type": "blob",
            "accept": [
              "image/svg+xml",
              "image/png",
              "image/jpeg",
              "image/webp",
              "image/gif",
              "image/x-icon",
              "image/vnd.microsoft.icon"
            ],
            "maxSize": 512000,
            "description": "Wordmark lettering (SVG) for the downloadable lockup. Default: appWordmark set in type."
          }
        }
      }
    },
    "link": {
      "type": "object",
      "required": ["label", "url"],
      "properties": {
        "label": {
          "type": "string",
          "maxLength": 256
        },
        "url": {
          "type": "string",
          "maxLength": 2048
        },
        "icon": {
          "type": "string",
          "maxLength": 256,
          "description": "A built-in icon name or, in socialLinks, socialIcon1..4."
        }
      }
    },
    "legalLink": {
      "type": "object",
      "required": ["text", "url"],
      "properties": {
        "text": {
          "type": "string",
          "maxLength": 256
        },
        "url": {
          "type": "string",
          "maxLength": 2048
        }
      }
    },
    "appColors": {
      "type": "object",
      "description": "Icon and splash colors of a built app; any CSS color.",
      "properties": {
        "ink": {
          "type": "string",
          "maxLength": 64
        },
        "paper": {
          "type": "string",
          "maxLength": 64
        },
        "iconBackground": {
          "type": "string",
          "maxLength": 64
        },
        "iconForeground": {
          "type": "string",
          "maxLength": 64
        },
        "adaptiveIconBackground": {
          "type": "string",
          "maxLength": 64
        },
        "adaptiveIconForeground": {
          "type": "string",
          "maxLength": 64
        },
        "splashBackground": {
          "type": "string",
          "maxLength": 64
        },
        "splashForeground": {
          "type": "string",
          "maxLength": 64
        },
        "tileBackground": {
          "type": "string",
          "maxLength": 64
        },
        "tileForeground": {
          "type": "string",
          "maxLength": 64
        },
        "tileHairline": {
          "type": "string",
          "maxLength": 64
        },
        "bannerBackground": {
          "type": "string",
          "maxLength": 64
        },
        "bannerForeground": {
          "type": "string",
          "maxLength": 64
        }
      }
    }
  }
}
```
