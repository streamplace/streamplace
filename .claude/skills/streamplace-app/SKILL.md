---
name: streamplace-app
description: Iterate on the Streamplace Expo app (js/app) with visual feedback. Launch the iOS-simulator dev loop (backend + metro + dev client), drive the app with maestro UI flows, and read the results back as screenshots, video, and accessibility trees. Use when working on the app UI, verifying a visual change, or needing to see what a screen actually looks like after editing code.
version: 1.0.0
---

# Streamplace app dev loop (iOS simulator)

Operational playbook for editing `js/app` (Expo SDK 55, React Native, expo-dev-client)
and actually seeing the result. The whole point: JS edits hot-reload in seconds,
and every run is capturable as screenshots + video for visual review.

## The fast loop (daily)

Two terminals, plus one capture command whenever you want visual proof:

```
make dev            # Rust backend (libstreamplace) on :38080, proxies frontend from :38081
pnpm app start:fast # metro on :38081, no cache clear (use plain `start` only when the cache is stale)
pnpm app dev:ios    # boot sim if needed + launch the dev client into metro (no native build)
```

After editing JS anywhere in the monorepo (including `js/components` and
`js/core`, which metro watches), fast refresh applies changes automatically.
To prove what the UI looks like now:

```
pnpm app ui:shoot home    # shell sweep: Home -> Videos -> Go Live -> Settings
pnpm app ui:shoot stream  # open first live card, capture the player, back out
```

## Reading the results

Each `ui:shoot` wipes and rewrites `js/app/artifacts/<flow>/`:

- `NN-step.png` — one screenshot per flow step (retina, ~1200x2600)
- `run.mp4` — h264 video of the whole run
- `hierarchy.json` — end-state accessibility tree

Without vision, the hierarchy is the primary feedback: it's the full text tree of
labels/titles/text on screen at flow end. Dump it live at any moment with
`maestro hierarchy` (strip the first two preamble lines before JSON-parsing).
With vision, read the PNGs directly; compare consecutive runs by re-shooting.

## Authoring flows

Flows live in `js/app/maestro/*.yaml` and target `appId: tv.aquareum.dev`
(dev bundle id; prod is `tv.aquareum`). The app's dev bundle/scheme honor
`SP_BUNDLE_OVERRIDE` / `SP_APP_SCHEME` if overridden.

- Selectors are real accessibility labels/text (tab labels, stream card titles).
- `text:` matching is **regex by default** — no `regex: true` property.
- `takeScreenshot: NN-name` — filenames are relative to maestro's cwd. The
  wrapper cd's into the artifacts dir first; if you run `maestro test` manually,
  cd into `artifacts/<flow>` first or the PNGs land in the app root.
- Record new flows interactively with `maestro studio` (taps are recorded), then
  copy the YAML into `maestro/`.
- `launchApp` restarts the app; the dev client reconnects to metro on :38081.

## First-time setup (or after native changes)

`make dev-setup` builds backend + native deps. `pnpm app ios` builds and installs
the dev client on the simulator (xcodebuild — slow, only needed once and again
when native code changes). `dev:ios` and `ui:shoot` both verify metro and the
installed dev client and print the exact fix when something's missing.

## Gotchas

- Metro must be running before `dev:ios` / `ui:shoot`; both check `:38081` and
  tell you to run `pnpm app start:fast`.
- `simctl recordVideo` requires `--display internal`, and the previous recorder
  must be dead — a stuck one fails with "Host recording is already in progress"
  (kill it with `pkill -INT -f "simctl io booted recordVideo"`).
- The backend (`:38080`) being down doesn't block UI flows — the shell renders,
  screens may show connection errors. Start `make dev` for full data.
- i18n: FTL edits in `js/i18n/locales` need `pnpm -F @streamplace/i18n compile:watch`
  running, or they won't appear.
- Native rebuild is needed only when native deps/config change; bump
  `runtimeVersion` in `js/app/package.json` then (it gates expo-updates).
