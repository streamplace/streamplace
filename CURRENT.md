# current work

## 2025-08-10: faster app dev loop (iOS simulator)

Goal: cut the change-and-see cycle on `js/app` (expo 55, dev client, iOS sim).

### what changed

- `js/app/package.json`: added `start:fast` (metro without `-c` cache clear) and
  `dev:ios` (launches dev client on the sim via deep link, no native build).
- `js/app/scripts/dev-ios.sh`: boots a sim if needed, verifies metro on :38081,
  verifies the dev client is installed, then deep-links
  `tv.aquareum.dev://expo-development-client/?url=…` (format confirmed from
  `@expo/cli` UrlCreator source). Honors `SP_METRO_PORT`/`SP_APP_SCHEME`/
  `SP_BUNDLE_OVERRIDE` to match `app.config.ts`.
- `js/app/README.md`: rewritten with daily loop vs one-time setup vs native
  rebuild triggers, plus the i18n watch gotcha.

### verified

- metro-not-running gate prints hint, exits 1.
- full happy path: booted iPhone 17 Pro sim, dev client (`tv.aquareum.dev`)
  installed, deep link opened, metro received the iOS bundle request from the
  app.

### notes / side observations (not fixed)

- metro logs are noisy with `exports` resolution warnings (lucide-react-native,
  @formatjs polyfills, `@streamplace/components` subpath). pre-existing.
- `js/app` package.json `i18n:*` scripts point at a nonexistent `src/i18n`;
  real FTL sources live in `js/i18n` (README documents `pnpm -F @streamplace/i18n
compile:watch`).

## 2025-08-10 (later): agent visual feedback loop (maestro)

Added a scriptable UI-capture loop so the agent can see its own work on the sim.

- `js/app/maestro/home.yaml` — shell sweep (Home -> Videos -> Go Live -> Settings),
  taps real tab labels, asserts visibility per stop.
- `js/app/maestro/stream.yaml` — taps the first `LIVE, .*` card, captures player, backs out.
- `js/app/scripts/ui-shoot.sh` — `pnpm app ui:shoot <flow>`: relaunches app,
  records run.mp4 (simctl recordVideo, needs `--display internal`), runs the
  maestro flow from inside `artifacts/<flow>` so screenshots land there, dumps
  end-state hierarchy.json. Deterministic output dir, wiped per run.
- `.gitignore`: `artifacts/`.
- README: "Visual feedback for the agent" section.

Learned: maestro `text` selectors are regex by default (no `regex: true` prop);
`takeScreenshot` names are relative to maestro's cwd; simctl recordVideo fails
with SimRenderServer error 2 unless `--display internal` and errors "Host
recording is already in progress" if a previous recorder is still running
(kill with SIGINT, not SIGKILL). Verified both flows end to end on the sim.

Left running after session: metro on :38081 (background job), sim booted.

## 2025-08-10 (even later): streamplace-app skill

Created `.claude/skills/streamplace-app/SKILL.md` — project-scoped skill encoding
the app dev loop + maestro visual feedback workflow. Description covers trigger
words (app UI, visual verification, screenshots, expo).

Git mechanics: `~/.gitignore_global` ignores `.claude/`, so added negations to
repo `.gitignore` (`!.claude/` + `!.claude/skills/**`) and re-ignored
`/.claude/settings.local.json`. Skill verified trackable.

Skill loading: discovered at session start from `~/.claude/skills/` (user) and
`.claude/skills/` (project). Won't appear in the skill list until the next
session — could not verify discovery mid-session. Fallback if it doesn't show:
move to `~/.claude/skills/streamplace-app/`.
