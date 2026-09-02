# @streamplace/components — redesign migration notes

The Linear × YouTube redesign restructured the design-token system and
primitives. Most public APIs are preserved via deprecated aliases; this file
lists everything a downstream consumer might notice.

## Typeface

- **Atkinson Hyperlegible → Geist / Geist Mono.** Apps embedding the
  components must load `Geist-Regular/Medium/SemiBold` and
  `GeistMono-Regular/Medium/SemiBold` (see
  `js/app/components/provider/provider.shared.tsx` for the expo-font map).
  Static weights only — the app does not load variable fonts.
- `fontFamilies.light/extraLight` now alias `regular`; `bold/extraBold`
  alias `semiBold`. The design system uses weights 400/500/600 only.

## Tokens

- `colors.primary` ramp is now pink (`#e955c2` at 500), aligned with the web
  app's `--primary`; `colors.secondary` is a teal ramp (`#1abbc0` at 500).
  `theme.colors.primary`/`ring` no longer use iOS systemBlue on iOS — one
  accent everywhere. Broadcaster branding overrides still apply.
- New `theme.colors` keys: `surface0–3`, `surfaceHover`, `text1–4`,
  `borderSubtle`, `borderStrong`, `live`, `liveDim`, `liveForeground`,
  `overlay`, `focus`, `danger`. Legacy keys (`background`, `card`,
  `popover`, `text`, `textMuted`, `textDisabled`, …) remain and map onto
  the new scale.
- `borderRadius`: `sm` is 4 (was 3); `xl`/`2xl`/`3xl` are deprecated
  aliases of `lg` (12).
- `spacing`: off-grid keys (5, 7, 9, 10, 11, 14, …) are deprecated; use
  the 4/8/12/16/24/32/48/64 grid (keys 1/2/3/4/6/8/12/16).
- `typography.ios` / `typography.android` are deprecated remaps onto the
  single universal scale (12/13/14/16/20/24/32). `typeScale` is the
  canonical scale. `usePlatformTypography()` returns remapped values.
- `animations` is deprecated in favor of `motion`
  (`fast` 120 / `base` 200 / `slow` 300, `bezier`, `easingCss`,
  `sheetSpring`). `animations.fast` changed 150→120; `slower` now equals
  300 (was 500).
- New exports: `surfaces`, `textAlphas`, `borderAlphas`, `statusColors`,
  `scrims`, `typeScale`, `fontWeights`, `tabularNums`, `motion`.

## Components

- **Button**: canonical variants are `primary/secondary/ghost/danger` and
  sizes `sm/md/lg`. `outline`→secondary, `destructive`→danger,
  `success`→primary, `xl`→lg, `pill`→`pill` prop, `icon`→use the new
  `IconButton`. Old names still compile.
- **Text**: `size` values map to the new scale — note `sm` is now 13
  (was 14), `base` 14 (was 16), `lg` 16 (was 18), `3xl`/`4xl` are 32.
  `weight` values outside normal/medium/semibold clamp to the nearest
  supported weight. New `tabular` prop for counts/timers.
- **View**: the `Card` and `Surface` convenience wrappers (View variants)
  were removed — both had no known usages. Use the new level-based
  `Surface`/`Card` from `components/ui/surface`.
- **New primitives**: `Surface`/`Card`, `Badge`/`LiveBadge`/`LivePulseDot`,
  `Avatar` (with `live` ring), `Skeleton`, `Tabs`, `IconButton`.
- **Focus rings**: on web the root `ThemeProvider` injects a global
  `:focus-visible` rule (2px `focus` ring, 2px offset). Remove any custom
  outline resets that fight it.
- **ButtonPrimitive.Root** accepts a new `pressedStyle` prop; its style
  prop is now a Pressable style function internally.

## App-internal (not published, listed for fork maintainers)

- `js/app/components/home/avatar.tsx` (unused `UserAvatar`) removed.
- `BottomMetadata` gained a `compact` prop and now renders on mobile
  portrait watch layouts, not just full desktop.
- Player control auto-hide is standardized at 3 s.
