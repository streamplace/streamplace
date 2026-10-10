# @streamplace/app

Expo (SDK 55) app for iOS/Android. Native projects (`ios/`, `android/`) are
gitignored and generated via `expo prebuild`; shared UI lives in
`@streamplace/components` and logic in `@streamplace/core`.

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
front door also loads its stream/video screen lazily. HLS playback and stream-key
generation load their libraries only when needed.

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
to confirm HLS, stream-key crypto, and secondary screens remain deferred.

`.expo/atlas.jsonl` contains source code and inlined public environment variables;
keep it local. Rebuild with `make app` and `make dev` before exercising embedded
UI with `hack/e2e-web-local.sh`.

`bootstrap.spec.ts` runs in Chromium and Firefox: it enforces a sub-1-MB first
loading payload, holds app downloads to check the visible spinner, then checks
the handoff to Home and recovery from a failed common-chunk request. This
HTML-only bootstrap does not apply to native installs.

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
