# @streamplace/app

Expo (SDK 55) app for iOS/Android. Native projects (`ios/`, `android/`) are
gitignored and generated via `expo prebuild`; shared UI lives in
`@streamplace/components` and logic in `@streamplace/core`.

## User search

Search is available without signing in. Wide windows (981px and up, matching
the sidebar breakpoint) have a persistent header Search field. Typing searches
immediately, throttled to one request per 300ms with the latest query sent at
the end of the interval. Results appear in a dropdown over the current page;
only selecting an account navigates to its stream. Escape, an outside press,
or clearing the query dismisses results. An empty field leaves page controls
usable and clearing preserves keyboard focus. Wide headers reserve the title
area for Search without changing browser tab titles. Embeds and OBS/popout
utilities do not include app search chrome.

Up/Down highlights results (wrapping at either end) and keeps the highlighted
account in view; Enter opens it. Editing or dismissing clears the highlight.
Short lists fit their content on first opening and reopening; long lists scroll
within half the window height. The popup has no entrance layout animation,
which would pin it to the loading state's height on web.
Completed results stay visible and selectable while a replacement query is
pending, including during the throttle interval. The loading placeholder only
appears before a completed result exists. New responses replace the list and
clear its keyboard highlight; clearing or dismissing still removes it immediately.

On smaller windows the header field is hidden entirely. The native Search tab
and web sidebar/tab still open `/search`, where Search or the keyboard's
search/Enter key submits the query. On mobile, submitting dismisses the keyboard
to keep results and empty states visible.

Both surfaces share `components/user-search.tsx` and use
`place.stream.live.searchActorsTypeahead` to search the current node's indexed
users in its database. Matching ignores case, surrounding whitespace, and a
leading `@`. Up to 20 active accounts are shown, ordered by handle; search does
not include display names, videos, or the wider Bluesky network.

Both surfaces use `components/search-results.tsx`, the existing profile cache
and `useAvatars` hook, and the shared `Avatar` component. Profile pictures appear
when available, with initials otherwise. The existing live-users poller supplies
live status and the Avatar's red ring; search adds no separate polling.

`header-user-search.spec.ts` covers automatic searches, unchanged routes,
selection/dismissal, late responses, retained results during pending queries,
responsive availability, loading-to-results sizing, scrolling, keyboard selection,
loaded profile images, and live rings.
The shared Maestro user-search flow retains native route coverage, checks the
live avatar, and checks that the header field is absent on phones.

## Daily loop (iOS simulator)

Three commands, first time only for the middle one:

```
make dev            # backend (Rust libstreamplace) on :38080, proxies the frontend from :38081
pnpm app start:fast # metro on :38081, no cache clear
pnpm app dev:ios    # boot sim (if needed) + launch dev client into metro; no native build
```

JS edits hot-reload in seconds, including edits inside `js/components` and
`js/core` (metro watches the whole monorepo and resolves `@streamplace/dev` to
source, so shared packages don't need rebuilding).

`start:fast` skips the `-c` cache clear that `start` performs. Use plain
`pnpm app start` when metro's cache is stale and you want a fresh transform.

## One-time setup

- `make dev-setup` builds the backend and native dependencies.
- `pnpm app ios` builds and installs the dev client on the simulator
  (xcodebuild, slow). Only needed once, and again whenever native code changes.

## When you need a native rebuild

Only when native code changes:

- adding/upgrading an npm package with native code (`expo-*`, `react-native-*`)
- editing `app.config.ts` or a config plugin
- editing `ios/` or `android/` directly

Then run `pnpm app ios` (or `pnpm app android`). Bump `runtimeVersion` in
`package.json` only when native dependencies change (it gates expo-updates).

## Web bundle splitting and Atlas

Secondary screens use module-scope `lazyScreen` imports with screen-local
Suspense and error boundaries. A failed download leaves navigation usable, and
retry creates a fresh lazy import instead of reusing React's cached rejection.
Keep the home feed and navigation shell eager; use direct
imports instead of barrels that re-export deferred screens. The broadcaster
front door also loads its stream/video screen lazily. HLS loads its playback
library only when needed. Stream keys use the existing atproto Secp256k1 keypair,
multibase private key and DID; no Ethereum address is derived or stored.
HLS library downloads retry every two seconds when native HLS is unavailable,
including recorded videos; leaving the player cancels the pending retry.
`vod-start-time.spec.ts` checks real playback and seeking after a failed chunk.

Expo splits production **web** exports. iOS/Android still include these modules
in the installed bundle; the same loading boundaries work without downloading
web chunks.

Production web HTML renders a dependency-free bootstrap spinner before loading
Expo's scripts. `scripts/bootstrap-web.mjs` runs after normal and Atlas exports:
it replaces the eager script tags with a hashed, bare DOM bootloader, preserving
Expo's runtime/common/entry order and asset base URL. React replaces the spinner
inside `#root` on its first committed render. Failed downloads offer a retry
without replaying successfully loaded scripts. Styles come from design tokens
at build time; no React, theme provider, fonts, or app modules load to show it.
Development Metro and installed iOS/Android entrypoints are unchanged.

The loading UI uses the regular web Loader's SVG ring with no visible loading
text (an accessible progress label remains). `pkg/linking` injects the node's
existing `primaryColor` and dark `backgroundColor` as CSS variables in the
initial HTML, so the spinner and page are branded before JavaScript runs.
Unset or invalid colors retain the design-token defaults; only validated hex
colors are embedded in CSS. Static hosting uses those defaults too.

The bootstrap build uses the project's installed TypeScript compiler to read
design tokens. It does not require Node's optional native TypeScript support or
`--experimental-strip-types`; the existing Node 22 requirement is unchanged.

This reduces **bytes required for the first loading UI**, not the bytes required
for a usable Home screen. The app still downloads automatically after that first
paint; route-level chunks remain deferred until opened.

Run the production analyzer from the builder container:

```sh
pnpm app analyze:web
```

The command prepares generated brand assets before exporting. Open the URL
printed by Expo Atlas (in a browser that can reach the container).
The export is in `.expo/atlas-web`, leaving the embedded `dist` bundle untouched.
Atlas's graph still describes the full Expo application, including deferred
modules; the separately emitted bootstrap is not in that graph. Measure the
bootstrap by summing the HTML and **all** script URLs directly referenced by
`.expo/atlas-web/index.html`. The `bootstrap-scripts` JSON manifest lists the
runtime/common/entry scripts that download after paint: include those too when
measuring bytes required for Home. Check source maps and browser network requests
to confirm HLS and secondary screens remain deferred, and no `viem` modules
remain in the Expo graph.

`.expo/atlas.jsonl` contains source code and inlined public environment variables;
keep it local. Rebuild with `make app` and `make dev` before exercising embedded
UI with `hack/e2e-web-local.sh`.

`bootstrap.spec.ts` runs in Chromium and Firefox: it enforces a sub-1-MB first
loading payload, holds app downloads to check the visible spinner, then checks
the handoff to Home and recovery from a failed common-chunk request.
`bootstrap-branding.spec.ts` edits the real node's branding through the admin UI,
checks spinner/page colors while Expo downloads are held, and restores the
original branding bundle. This HTML-only bootstrap does not apply to native
installs.

`lazy-navigation.spec.ts` checks deferred settings requests, navigation away from
a pending chunk, recovery from a failed chunk, and a cold settings deep link. The platform-neutral
`.maestro/logged-out/lazy-navigation.yaml` covers first-use navigation and deep
links on iOS/Android.

## i18n

FTL strings live in `../i18n/locales` and are compiled before use. Edits won't
show up unless `pnpm -F @streamplace/i18n compile:watch` is running (or you run
`pnpm -F @streamplace/i18n compile` once).

## Visual feedback for the agent

To see what the app actually looks like after a change, drive it and capture
screenshots + video with maestro (already installed on this machine):

```
pnpm app ui:shoot home    # shell sweep: Home -> Videos -> Settings; create controls stay hidden
pnpm app ui:shoot stream  # opens the first live stream, captures the player
```

If maestro isn't installed, the script prints the official install command and
offers to run it (non-interactive runs just exit with the hint).

Each run writes into a deterministic dir:

- `artifacts/<flow>/NN-step.png` — one screenshot per step
- `artifacts/<flow>/run.mp4` — video of the whole run
- `artifacts/<flow>/hierarchy.json` — end-state accessibility tree (readable
  text, no vision needed)

Flows live in `maestro/*.yaml` and use real UI selectors (tab labels, stream
card titles). Add flows by copying an existing one; run `maestro studio` to
record taps interactively. `artifacts/` is gitignored. The app must be running
(`pnpm app dev:ios` handles launching it, and the script re-runs it).
