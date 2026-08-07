# UI Redesign — Phase 0 Audit

Target: Linear.app (restraint, precision, dark-first, consistency) × YouTube
(content-forward, video-first layout grammar). Visual/interaction redesign
only — hooks, data flow, and functionality untouched.

## 1. Screen map

Navigation: React Navigation v8 alpha (not expo-router). Three levels:
RootStack → Tab.Navigator → per-tab stacks (`js/app/src/shell.tsx`,
`js/app/src/root-navigator.tsx`). Web shows a header + collapsible sidebar
(`js/app/src/router.tsx`, `useSidebarControl`); native uses bottom tabs.

### Core screens (redesign targets)

| Screen                | File                                                                                                       | Notes                                                                                                                                                                      |
| --------------------- | ---------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Watch page (live)     | `js/app/src/screens/mobile-stream.tsx` → `js/app/components/mobile/player.tsx`                             | The core product. Player + chat side panel (web ≥1024) / below (mobile). `BottomMetadata` title row exists but is gated to `showFullDesktopMode` (aspect>1 && width>1200). |
| Home / discovery      | `js/app/src/screens/home.tsx` + `js/app/components/home/cards.tsx`                                         | Custom responsive grid (1/2/3/4 cols), LiquidGlassView cards, LIVE badge, activity tags.                                                                                   |
| Chat                  | `js/components/src/components/chat/{chat,chat-message,chat-box}.tsx` + `js/app/components/mobile/chat.tsx` | Gesture-handler FlatList, 25 msgs native / 100 web, swipe reply/actions, per-user RGB colors.                                                                              |
| VOD player            | `js/app/src/screens/vod.tsx` (VodPlayer), `video.tsx` (legacy)                                             | Seek bar via gesture-handler, vod-controls.                                                                                                                                |
| VOD gallery           | `js/app/src/screens/video-list.tsx`                                                                        | Grid, pagination, per-user via `?did`.                                                                                                                                     |
| Broadcaster dashboard | `js/app/src/screens/live-dashboard.tsx` + `js/app/components/live-dashboard/`                              | BentoGrid metrics, stream-key (WHIP/RTMP), stream-monitor.                                                                                                                 |
| Go live               | `js/app/src/screens/{launch-go-live,mobile-go-live}.tsx`                                                   | Player in ingest mode; `useLivestreamInfo` drives title/countdown/toggle.                                                                                                  |
| Upload / drafts       | `js/app/src/screens/upload.tsx`                                                                            | 2193 lines, multi-phase tus upload + draft editor. Restyle in place only.                                                                                                  |
| Settings              | `js/app/components/settings/` (16 category screens)                                                        | Stack of categories under SettingsTab.                                                                                                                                     |
| Auth / login          | `js/app/components/login/{login,login-form,pds-host-selector-modal}.tsx`                                   | ATProto OAuth, PDS selector.                                                                                                                                               |
| Misc                  | `about.tsx`, `download.tsx`, `support.tsx`, `app-return.tsx`                                               | Low traffic.                                                                                                                                                               |

### Overlay surfaces — MUST keep transparent/minimal roots (do not re-skin)

These render inside OBS browser sources, iframes, or popout windows. They must
never inherit an opaque `surface0` root background.

- `danmu-obs.tsx` (OBS comment overlay)
- `embed.tsx`, `vod-embed.tsx` (iframe embeds)
- `chat-popout.tsx` / `chat-popout.native.tsx` (query flags: reverse, hideAfter…)
- `info-widget-embed.tsx`, `popout-info-widget.tsx`
- `popout-livestream.tsx`, `popout-multistream.tsx`, `popout-stream-monitor.tsx`
- `multi.tsx` (multi-stream wall)

Intentional literals in these files get `// token-ok` markers (ratchet allowlist).

## 2. Current styling system inventory

Lives in `js/components/src/lib/theme/` (tokens.ts, theme.tsx, atoms.ts,
branded-theme-provider.tsx). Consumed via `useTheme()` → `{ theme.colors,
zero (pairified atoms), icons, isDark, setTheme }`.

What exists today:

- **Colors**: 23 full Tailwind ramps (50–950) + semantic destructive/success/
  warning ramps + iOS/Android platform system colors. Semantic `Theme.colors`
  (background, card, popover, primary, muted, border, ring, text, textMuted, …)
  generated from a palette by `generateThemeColorsFromPalette`. Dark/light/
  system switching works. `BrandedThemeProvider` overrides `primary/ring/accent`.
- **Typography**: THREE parallel systems (iOS HIG 11 styles, Material 13
  styles, universal 8 sizes) + mono scale + fontSize atoms 12–128. Font:
  Atkinson Hyperlegible Next + Mono, 7 weights, loaded in
  `js/app/components/provider/provider.shared.tsx`; family names referenced
  only in tokens.ts.
- **Spacing**: 0–384 in 4px steps (31 keys — far more than needed).
- **Radii**: none/3/8/12/16/20/24/full.
- **Shadows**: sm–xl. **Motion**: 150/200/300/500, no shared easing.
  **Touch targets**: 44/48/56.
- **Primitives** (`js/components/src/components/ui/`): Button (6 variants,
  6 sizes), Text (12 variants + conveniences), Input (3 variants), Dialog
  (modal/sheet/fullscreen), Checkbox, Toast (reanimated), Dropdown, Select,
  Slider, Loader, Menu (Radix on web), Tooltip, Textarea, Portal, View,
  Icons, InfoBox/InfoRow, Admonition. Chat-only Badge. **Missing**: Skeleton,
  Tabs, IconButton, Avatar (with live ring), general Badge, Surface/Card.
- **reanimated ~4.2.1**: present, used in toast, live-dot, gradient, chat,
  player, sidebar-overlay, etc.

### Hardcoded-literal debt (ratchet baseline, 2026-07-05)

`node js/scripts/check-tokens.mjs` — counts hex, rgb()/rgba(), raw ramp
indexing (`colors.gray[...]`) outside `lib/theme`:

| Directory         | Literals |
| ----------------- | -------- |
| js/app/src        | 25       |
| js/app/components | 195      |
| js/components/src | 140      |
| **Total**         | **360**  |

Representative offenders: `shell.tsx` `#06f` accent fallback, `home.tsx`
`#774316`/`#99889988`, `video.tsx` `#000/#111/#aaa`, `vod.tsx` rgba scrims,
`ui/` modal scrims `rgba(0,0,0,0.5)`, chat mention styling, dashboard colors.
The baseline may only decrease (`docs/redesign/token-baseline.json`); the
final state is allowlist-only.

**Final state (end of Phase 3): 24 literals** — all in `js/app/components`,
all either dynamic color computation (name-color-picker contrast math,
badge-picker swatches, sidebar tint alpha-injection) or data defaults
(branding config values). Every intentional literal in component styles
carries a `// token-ok` marker. Everything else flows through the token
system.

## 3. Highest-leverage surfaces (by user exposure)

1. Watch page (player + metadata + controls) — every viewer session
2. Chat — open during every live session
3. Home/discovery grid — every session entry point
4. App shell (header/sidebar/tab bar) — chrome on every screen
5. Player async states (buffering/offline/error) — first impression quality
6. Broadcaster dashboard + go-live — every streamer session
7. VOD gallery + VOD player
8. Settings (16 screens) — lower traffic, high consistency payoff
9. Login — first-run moment
10. Upload — creators only, but long dwell time

## 4. Build order

| Commit | Scope                                                                                                                                               |
| ------ | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| C0     | This audit + ratchet script                                                                                                                         |
| C1–C2  | Token rebuild (surfaces/text/borders/indigo accent/live red, 7-step type scale, motion), Geist swap, theme.tsx semantic extension, DESIGN-SYSTEM.md |
| C3     | Text/Button/IconButton + focus ring                                                                                                                 |
| C4     | Inputs (input/textarea/select/checkbox/slider)                                                                                                      |
| C5     | New primitives: Surface, Badge/LiveBadge, Avatar (live ring), Skeleton, Tabs                                                                        |
| C6     | Overlays (dialog/toast/menu/dropdown/tooltip/loader/info-\*/admonition) + ui/ sweep                                                                 |
| C7     | Watch page                                                                                                                                          |
| C8     | Chat                                                                                                                                                |
| C9     | Home/discovery                                                                                                                                      |
| C10    | Broadcaster/go-live                                                                                                                                 |
| C11    | Shell, settings, auth, upload, video-list, misc                                                                                                     |
| C12    | Overlay-surface transparency pass, delete deprecated aliases, MIGRATION.md, final ratchet                                                           |

Decisions locked with the user: typeface = **Geist** (Sans + Mono, 400/500/600
only); accent = **Linear-style indigo** (~#5E6AD2); red reserved for LIVE.
