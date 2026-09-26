# Brand assets

This directory is the single source of truth for Streamplace's visual
identity. Everything else — the Expo app icon and splash screen (and from
those, every iOS and Android icon format via `expo prebuild`), favicons, the
OG link banner, desktop ICO/ICNS icons, docs logos, the downloadable SVGs on
`/brand`, and the logo components rendered in the app itself — is generated
from these files by `js/brand/generate.mjs` and gitignored.

Run the generator with `pnpm run brand` from the repo root. It also runs
automatically on `pnpm install` and before app, docs, and desktop builds.

## One brand, everywhere

A brand directory is a node **branding bundle**, unzipped: `branding.yaml`
plus the files it names, in the same vocabulary as Settings → Branding
(`pkg/branding/vocab.go`). So the same directory:

- brands the **builds** — this generator reads it;
- brands a **node** — `streamplace branding import path/to/dir`, or zip it
  and use Settings → Branding → Import bundle;
- is what a node **exports** (`streamplace branding export path/to/dir`),
  and what a custom domain's brand record **pulls** into
  (`streamplace branding pull at://… path/to/dir`).

Runtime keys (`siteTitle`, `primaryColor`, `mainLogo`, …) are served by the
node; build-time `app*` keys are only read by builds. See the operator guide,
_Branding bundles_, for the whole picture, custom domains included.

## White-labeling

Point the generator at your own brand directory, either of:

- `brand/custom/` — a gitignored sibling of this directory; if it contains a
  `branding.yaml` (or legacy `brand.json`) it takes precedence.
- `SP_BRAND_DIR=/path/to/your/brand` — explicit override, wins over both.

The directory in git holds the generic open-source identity.

## What a build reads

Only the mark is required; everything else is synthesized from it and
`appColors` when absent.

| Key                 | Required | Purpose                                                                                                                        |
| ------------------- | -------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `mainLogo`          | yes      | The mark, an SVG that declares a `viewBox`. If `appMonochrome` is `on`, use `fill="currentColor"` so the UI can tint it.       |
| `appName`           | yes\*    | Product name (\*or `siteTitle`, its default).                                                                                  |
| `appWordmark`       | no       | Text in the app's lockup; a `.` gets the accent treatment. Default `appName`.                                                  |
| `siteTitle`         | no       | What an unbranded node calls itself until its operator sets one. Default `My <appName> Node`.                                  |
| `appMonochrome`     | no       | `on` for single-color marks.                                                                                                   |
| `appColors`         | no       | Colors below.                                                                                                                  |
| `appBundleId`       | no       | iOS bundle id / Android package (`SP_BUNDLE_OVERRIDE` wins). Default `tv.aquareum`.                                            |
| `appHost`           | no       | Node the built app talks to by default, and its app-link domain. Default `stream.place`.                                       |
| `appIcon`           | no       | Full-bleed square app icon (SVG/PNG). Default: mark at 62% on `iconBackground`.                                                |
| `appIconForeground` | no       | Android adaptive icon foreground (keep art in the inner ~66% safe zone). Default: mark at 45% on transparent.                  |
| `appSplash`         | no       | Splash screen logo, shown on `splashBackground`. Default: mark at 50% on transparent.                                          |
| `appWordmarkImage`  | no       | Wordmark lettering (SVG) for downloads/lockup. Default: SVG `<text>` of the wordmark string.                                   |
| `linkBanner`        | no       | 1200×630 OG/social card (SVG/PNG). Default: mark centered on `bannerBackground`.                                               |
| `appStory`          | no       | The mark's design story for the `/brand` guidelines screen (`BrandStory` in the generated `js/app/assets/generated/brand.ts`). |

### appColors

```yaml
appColors:
  ink: "#0A0A0B"
  paper: "#ffffff"
  iconBackground: "#ffffff"
  iconForeground: null
  adaptiveIconBackground: "#111113"
  adaptiveIconForeground: null
  splashBackground: "#ffffff"
  splashForeground: null
  tileBackground: "#111113"
  tileForeground: "#ffffff"
  tileHairline: "rgba(255,255,255,0.10)"
  bannerBackground: null
  bannerForeground: null
```

All colors are optional; `*Foreground` colors default to `ink` and only
apply to monochrome marks (a multi-color mark renders as-is).

### Legacy brand.json directories

A directory with a `brand.json` (`name`, `wordmark`, `defaultSiteTitle`,
`monochrome`, `colors`, `story`) and conventionally named files (`mark.svg`,
`icon.png`, `icon-foreground.svg`, `splash.svg`, `wordmark.svg`,
`linkbanner.png`) still builds. To make it importable into a node as well,
move those values into `branding.yaml` under the keys above.
