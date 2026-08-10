# Brand assets

This directory is the single source of truth for Streamplace's visual
identity. Everything else — the Expo app icon and splash screen (and from
those, every iOS and Android icon format via `expo prebuild`), favicons, the
OG link banner, desktop ICO/ICNS icons, docs logos, the downloadable SVGs on
`/brand`, and the logo components rendered in the app itself — is generated
from these files by `js/brand/generate.mjs` and gitignored.

Run the generator with `pnpm run brand` from the repo root. It also runs
automatically on `pnpm install` and before app, docs, and desktop builds.

## White-labeling

To ship your own identity, point the generator at your own flat directory of
files, either of:

- `brand/custom/` — a gitignored sibling of this directory; if it contains a
  `brand.json` it takes precedence.
- `SP_BRAND_DIR=/path/to/your/brand` — explicit override, wins over both.

The directory in git holds the generic open-source identity.

## File contract

Only two files are required; everything else is synthesized from the mark
and `brand.json` colors when absent.

| File                           | Required | Purpose                                                                                                            |
| ------------------------------ | -------- | ------------------------------------------------------------------------------------------------------------------ |
| `brand.json`                   | yes      | Name, wordmark text, colors, `monochrome` flag, optional `story` for the /brand guidelines page.                   |
| `mark.svg`                     | yes      | The logo mark. Must declare a `viewBox`. If `monochrome` is true, use `fill="currentColor"` so the UI can tint it. |
| `icon.svg` / `icon.png`        | no       | Full-bleed square app icon art. Default: mark at 62% on `colors.iconBackground`.                                   |
| `icon-foreground.svg` / `.png` | no       | Android adaptive icon foreground (keep art in the inner ~66% safe zone). Default: mark at 45% on transparent.      |
| `splash.svg` / `splash.png`    | no       | Splash screen logo, shown on `colors.splashBackground`. Default: mark at 50% on transparent.                       |
| `wordmark.svg`                 | no       | Wordmark lettering for downloads/lockup. Default: SVG `<text>` of the wordmark string.                             |
| `linkbanner.svg` / `.png`      | no       | 1200×630 OG/social card. Default: mark centered on `colors.bannerBackground`.                                      |

### brand.json

```json
{
  "name": "Streamplace",
  "wordmark": "stream.place",
  "monochrome": false,
  "colors": {
    "ink": "#0A0A0B",
    "paper": "#ffffff",
    "iconBackground": "#ffffff",
    "iconForeground": null,
    "adaptiveIconBackground": "#111113",
    "adaptiveIconForeground": null,
    "splashBackground": "#ffffff",
    "splashForeground": null,
    "tileBackground": "#111113",
    "tileForeground": "#ffffff",
    "tileHairline": "rgba(255,255,255,0.10)",
    "bannerBackground": null,
    "bannerForeground": null
  },
  "story": null
}
```

All colors are optional; `*Foreground` colors default to `ink` and only
apply to monochrome marks (a multi-color mark renders as-is). `wordmark` is
the text rendered next to the mark in the app's lockup — a `.` in it gets
the accent treatment. `story` optionally carries the mark's design story for
the `/brand` guidelines screen (`tagline`, `readings`, `geometry`, `specs`,
`usage` — see `BrandStory` in the generated `js/app/assets/generated/brand.ts`);
sections without data are hidden.
