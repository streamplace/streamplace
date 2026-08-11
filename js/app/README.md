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

## i18n

FTL strings live in `../i18n/locales` and are compiled before use. Edits won't
show up unless `pnpm -F @streamplace/i18n compile:watch` is running (or you run
`pnpm -F @streamplace/i18n compile` once).

## Visual feedback for the agent

To see what the app actually looks like after a change, drive it and capture
screenshots + video with maestro (already installed on this machine):

```
pnpm app ui:shoot home    # shell sweep: Home -> Videos -> Go Live -> Settings
pnpm app ui:shoot stream  # opens the first live stream, captures the player
```

Each run writes into a deterministic dir:

- `artifacts/<flow>/NN-step.png` — one screenshot per step
- `artifacts/<flow>/run.mp4` — video of the whole run
- `artifacts/<flow>/hierarchy.json` — end-state accessibility tree (readable
  text, no vision needed)

Flows live in `maestro/*.yaml` and use real UI selectors (tab labels, stream
card titles). Add flows by copying an existing one; run `maestro studio` to
record taps interactively. `artifacts/` is gitignored. The app must be running
(`pnpm app dev:ios` handles launching it, and the script re-runs it).
