---
name: streamplace-design
description: Streamplace design system and token discipline. Use when styling components under js/app, js/components, or js/web, picking colors/typography/spacing/radius/motion, or resolving a streamplace/no-token-literals lint failure.
version: 1.0.0
---

# Streamplace design system

Follow `AGENTS.md` for repository-wide rules. This skill covers the design-system
contract and the token reference.

Every visual decision is a token in `js/components/src/lib/theme/tokens.ts`,
consumed through `useTheme()` (`js/components/src/lib/theme/theme.tsx`).

## Always use theme tokens for colors

**Component code never contains a raw hex value, `rgba()`, numeric font size,
spacing, radius, or duration.** Read every value from the theme instead:
`theme.colors.*`, `theme.spacing[n]`, `theme.typeScale.*`, `theme.borderRadius.*`,
`theme.motion.*`.

You will generally not want to suppress a line with eslint:
`// eslint-disable-line streamplace/no-token-literals -- <reason>` (the
`/* … */` form inline in JSX).

Legitimate cases are narrow: a literal that must
render when the theme provider itself may have crashed (the whole-app crash
screen), a transparent overlay root composited over OBS content, or a
brand-guideline swatch that displays the raw palette by design.

### Enforcement

The rule is `streamplace/no-token-literals`, defined in
`js/scripts/eslint/no-token-literals.mjs` and wired up in the repo-root
`eslint.config.mjs`. It flags raw hex, `rgb()`/`rgba()`, and raw palette-ramp
indexing (`colors.primary[500]`) across `js/app/src`, `js/app/components`,
`js/app/hooks`, and `js/components/src`, excluding the token definitions in
`js/components/src/lib/theme`.

Run it directly:

```sh
pnpm run lint                                             # the four dirs
node --test js/scripts/eslint/no-token-literals.test.mjs  # rule tests
```

Enforcement is automatic: `.husky/pre-commit` runs `pnpm run lint`, and
`make check` runs it via the root `check` chain. There is no baseline and no
count to maintain. Every literal is an error, so use a token instead of
silencing the rule. The same ESLint pass also runs
`react-hooks/rules-of-hooks` (as a warning) over this code.

## Color

Read colors from `theme.colors`. There is a light and a dark variant, and
broadcaster branding can override the accent, so never copy a literal value out
of this file: pick the token whose role matches.

The neutral ramp (surfaces, text, borders) is a clean, untinted near-black/white
system. The accents (`primary`, `secondary`) align with the web app's CSS tokens
(`js/web/src/styles.css`).

The web app (`js/web`) is a Tailwind v4 + shadcn app on Base UI, and its
`src/styles.css` defines the shadcn variables (`--background`, `--card`,
`--primary`, `--ring`, …). These tokens mirror those names, so keep the two in
step. `surface0/1/2` are the same slots as the web's
`--background`/`--card`/`--popover`, which is where the legacy aliases come from.

### Surfaces (`theme.colors.surface0–3`, `surfaceHover`)

Untinted near-black on dark, off-white on light; never pure black. Surfaces
separate with hairline borders instead of shadows. `surface0` is the app
background; `surface1` cards, panels, and inputs; `surface2` popovers, menus, and
sheets; `surface3` hovered overlay rows and tooltips. `surfaceHover` matches
`surface3`.

Legacy aliases (kept working): `background`→surface0, `card`→surface1,
`popover`→surface2.

### Text (`theme.colors.text1–4`)

White on dark / ink on light, at decreasing alphas: `text1` primary (titles,
body), `text2` secondary (metadata, descriptions), `text3` tertiary
(placeholders, timestamps), `text4` disabled.

Legacy aliases: `text`→text1, `textMuted`→text2, `textDisabled`→text4.

### Borders (`borderSubtle` / `border` / `borderStrong`)

1px hairlines. Subtle for surface separation, default for controls at rest,
strong for hover.

### Accent and secondary

One accent (`colors.primary`, the web's `--primary`). Use it
sparingly: primary buttons, focus rings, active states, links, the Go Live
moment. Not for large fills or decoration. Broadcaster branding may override
`primary`/`ring`; `focus` follows `ring` automatically.

Secondary: teal (`colors.secondary`, the web's `--secondary`/`--accent`).

### Status

| Token                                 | Rule                                                                                          |
| ------------------------------------- | --------------------------------------------------------------------------------------------- |
| `live` / `liveDim` / `liveForeground` | **Reserved for the LIVE state only**: badges, live avatar rings, on-air dots. Not for errors. |
| `success`                             | Healthy ingest, confirmations                                                                 |
| `warning`                             | Degraded states                                                                               |
| `danger` / `destructive`              | Errors, destructive actions                                                                   |
| `overlay`                             | Modal scrims                                                                                  |

## Typography

For typography sizes, use `typeScale`: sizes
12/13/14/16/20/24/32, line heights in the token file, tight letter-spacing
from 20px up:

| Key    | Size/Line  | Weight | Use                                   |
| ------ | ---------- | ------ | ------------------------------------- |
| `xs`   | 12/16      | 400    | Badges, timestamps, overlines         |
| `sm`   | 13/18      | 400    | Chat messages, dense metadata         |
| `base` | 14/20      | 400    | Default UI text                       |
| `md`   | 16/24      | 400    | Stream titles (rows), emphasized body |
| `lg`   | 20/26 −0.2 | 500    | Section headings                      |
| `xl`   | 24/30 −0.3 | 600    | Page titles                           |
| `xxl`  | 32/38 −0.5 | 600    | Hero moments only                     |

- **Counts, timers, and durations always use `tabularNums`**
  (`fontVariant: ["tabular-nums"]`) so digits do not jitter.
- `typography.mono.*` for stream keys, ingest URLs, diagnostics.
- `typography.ios` / `typography.android` / fontFamily keys outside
  regular/medium/semiBold are **deprecated remaps**. Do not use them in new code.

## Spacing & layout

4px grid. Canonical steps and their token keys:

| Key | 1   | 2   | 3   | 4   | 6   | 8   | 12  | 16  |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| px  | 4   | 8   | 12  | 16  | 24  | 32  | 48  | 64  |

All padding/margin/gap from this set (via `spacing[n]`, atoms `p`/`m`/`gap`).
Off-grid keys (5, 7, 9, 10, 11, 14, 20+) are deprecated.

Radii (`borderRadius`): `sm` 4 (small controls), `md` 8 (cards, inputs,
buttons), `lg` 12 (thumbnails, modals), `full` (avatars, pills). `xl/2xl/3xl`
are deprecated aliases of `lg`.

Hit targets: minimum 44px on touch platforms (`touchTargets.minimum`).

## Motion (`motion`)

| Token                | Value                      | Use                                             |
| -------------------- | -------------------------- | ----------------------------------------------- |
| `motion.fast`        | 120ms                      | Micro: hover, press feedback                    |
| `motion.base`        | 200ms                      | Standard: reveals, toggles, fades               |
| `motion.slow`        | 300ms                      | Structural: sheets, panels                      |
| `motion.bezier`      | (0.25, 0.1, 0.25, 1)       | `Easing.bezier(...motion.bezier)` in reanimated |
| `motion.easingCss`   | same, as CSS string        | Web transitions                                 |
| `motion.sheetSpring` | damping 30 / stiffness 300 | **The only allowed spring**, sheets only        |

Everything that appears fades and translates 4–8px. Nothing pops. No bounce.

## Elevation

Use a raised surface and a hairline border instead of a shadow. Reserve
shadows (`shadows.sm` only, in practice) for floating layers: menus, toasts,
and popovers.

## Focus (web)

Every interactive element gets a 2px `focus`-colored ring with a 2px offset
(`outline` on web, border fallback on native). Keyboard navigation must work
correctly. Do not remove focus states. Style them.
