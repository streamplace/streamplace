# Streamplace Design System

Dark-first. Linear-grade restraint, YouTube-familiar layout grammar. Every
visual decision is a token in `js/components/src/lib/theme/tokens.ts`,
consumed through `useTheme()` (`js/components/src/lib/theme/theme.tsx`).

**The one rule: component code never contains a raw hex value, rgba(),
numeric font size, spacing, radius, or duration.** The ratchet
(`js/scripts/check-tokens.mjs`) enforces this. Intentional exceptions
(e.g. transparent OBS overlay roots) carry a `// token-ok` comment.

## Color

The neutral ramp (surfaces, text, borders) is a clean, untinted
near-black/white system. The accent colors (`primary`, `secondary`) are
aligned with the web app's CSS tokens (`js/web/src/styles.css`).

### Surfaces (`theme.colors.surface0–3`, `surfaceHover`)

Clean near-black, never pure black and never tinted. Surfaces separate with
hairline borders, not shadows.

| Token          | Dark       | Light      | Use                            |
| -------------- | ---------- | ---------- | ------------------------------ |
| `surface0`     | `#0a0a0b`  | `#ffffff`  | App background                 |
| `surface1`     | `#111113`  | `#fafafa`  | Cards, panels, inputs          |
| `surface2`     | `#18181b`  | `#f4f4f5`  | Popovers, menus, sheets        |
| `surface3`     | `#1f1f23`  | `#ececef`  | Hovered overlay rows, tooltips |
| `surfaceHover` | = surface3 | = surface3 | Hover fill on interactive rows |

Legacy aliases (kept working): `background`→surface0, `card`→surface1,
`popover`→surface2.

### Text (`theme.colors.text1–4`)

White (dark) / ink (light) at fixed alphas.

| Token   | Dark                    | Light              | Use                                |
| ------- | ----------------------- | ------------------ | ---------------------------------- |
| `text1` | `rgba(255,255,255,.92)` | `rgba(9,9,11,.92)` | Primary: titles, body              |
| `text2` | `rgba(255,255,255,.65)` | `rgba(9,9,11,.66)` | Secondary: metadata, descriptions  |
| `text3` | `rgba(255,255,255,.45)` | `rgba(9,9,11,.46)` | Tertiary: placeholders, timestamps |
| `text4` | `rgba(255,255,255,.30)` | `rgba(9,9,11,.32)` | Disabled                           |

Legacy aliases: `text`→text1, `textMuted`→text2, `textDisabled`→text4.

### Borders (`borderSubtle` / `border` / `borderStrong`)

1px hairlines: `rgba(255,255,255,0.06 / 0.08 / 0.10)` in dark. Subtle for
surface separation, default for controls at rest, strong for hover.

### Accent (`primary`, `ring`, `focus`) & secondary

One accent: pink/magenta `#e955c2` (`colors.primary` ramp, the web's
`--primary`). Used **sparingly** — primary buttons, focus rings, active
states, links, the Go Live moment. Never for large fills or decoration.
Broadcaster branding may override `primary`/`ring` (and `focus` follows
`ring` automatically).

Secondary/accent: teal `#1abbc0` (`colors.secondary` ramp, the web's
`--secondary`/`--accent`).

### Status

| Token                                 | Dark              | Rule                                                                                                  |
| ------------------------------------- | ----------------- | ----------------------------------------------------------------------------------------------------- |
| `live` / `liveDim` / `liveForeground` | `#f23041`         | **Reserved for the LIVE state only** — badges, live avatar rings, on-air dots. Never used for errors. |
| `success`                             | `#3dd68c`         | Healthy ingest, confirmations                                                                         |
| `warning`                             | `#ffb224`         | Degraded states                                                                                       |
| `danger` / `destructive`              | `#ff3b5c`         | Errors, destructive actions                                                                           |
| `overlay`                             | `rgba(0,0,0,0.6)` | Modal scrims                                                                                          |

## Typography

One typeface: **Geist** (+ **Geist Mono**), weights 400/500/600 only,
static (no variable fonts). Canonical scale
`typeScale` — sizes 12/13/14/16/20/24/32, line heights in the token file,
tight letter-spacing from 20px up:

| Key    | Size/Line  | Weight | Use                                   |
| ------ | ---------- | ------ | ------------------------------------- |
| `xs`   | 12/16      | 400    | Badges, timestamps, overlines         |
| `sm`   | 13/18      | 400    | Chat messages, dense metadata         |
| `base` | 14/20      | 400    | Default UI text                       |
| `md`   | 16/24      | 400    | Stream titles (rows), emphasized body |
| `lg`   | 20/26 −0.2 | 500    | Section headings                      |
| `xl`   | 24/30 −0.3 | 600    | Page titles                           |
| `xxl`  | 32/38 −0.5 | 600    | Hero moments only                     |

- **Counts, timers, durations always use `tabularNums`**
  (`fontVariant: ["tabular-nums"]`) so digits don't jitter; long-form timers
  use Geist Mono.
- `typography.mono.*` (Geist Mono) for stream keys, ingest URLs, diagnostics.
- `typography.ios` / `typography.android` / fontFamily keys outside
  regular/medium/semiBold are **deprecated remaps** — do not use in new code.

## Spacing & layout

4px grid. Canonical steps and their token keys:

| Key | 1   | 2   | 3   | 4   | 6   | 8   | 12  | 16  |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| px  | 4   | 8   | 12  | 16  | 24  | 32  | 48  | 64  |

All padding/margin/gap from this set (via `spacing[n]`, atoms `p`/`m`/`gap`).
Off-grid keys (5, 7, 9, 10, 11, 14, 20…) are deprecated.

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

Everything that appears fades + translates 4–8px. Nothing pops. No bounce.

## Elevation

Prefer a raised surface + hairline border over shadow. Shadows (`shadows.sm`
only, in practice) are reserved for floating layers: menus, toasts, popovers.

## Focus (web)

Every interactive element: 2px `focus`-colored ring with 2px offset
(`outline` on web, border fallback native). Keyboard navigation must be
flawless; focus states are never removed, only styled.

## Migration status

Deprecated-but-working during the redesign (removal tracked in
MIGRATION.md at the end of Phase 3): platform typography scales, fontFamily
weight aliases, off-grid spacing keys, radius xl/2xl/3xl, `animations`
(use `motion`), the 17 unused Tailwind ramps.
