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

## 2025-08-10 (night): maestro install prompt in ui-shoot.sh

`ui-shoot.sh` now locates maestro before doing anything: `command -v maestro`,
falling back to `~/.maestro/bin/maestro` (official installer's home), and if
both miss, prints the official install command and interactively offers to run
it (non-TTY runs exit 1 with the hint — no hanging for agents/CI). Replaced
bare `maestro` calls with `$MAESTRO`. Verified: syntax, missing branch (message

- exit 1), home-fallback full run (5 screenshots + video + tree). Did NOT test
  the interactive "y" install path (would reinstall maestro via curl|bash).
  README + skill updated with the behavior.

## 2026-09-24: AGENTS.md quality-gate revision + design skill + token ratchet

Goal: make AGENTS.md self-describing so it works for contributors who don't carry
a personal global `~/.claude/CLAUDE.md`. The repo was implicitly leaning on that
global guide to paper over gaps in the one file that actually travels with it.

### what changed

- `AGENTS.md`: new "Quality gate" section naming the real bar (`make check`,
  `make fix`, the pre-commit subset) and demoting `go vet` to a fast pre-check;
  documented the `.golangci.yaml` excludes (ST1003/ST1005/ST1006/SA5008/QF1003,
  `unused` disabled) so nobody "fixes" generated code or re-enables a check;
  pointed UI work at the design skill and Go idioms at the conventions doc.
- `docs/go-conventions.md` (new): the unwritten Go idioms — `log.Log` (there is
  no `log.Info`), `%w` wrapping, `errors.WriteHTTP*`, `testify/require`. Grounded
  in `pkg/log` + `pkg/errors`, not just call counts.
- `.claude/skills/streamplace-design/SKILL.md` (new): the design system, moved
  out of `docs/redesign/DESIGN-SYSTEM.md` (deleted; git shows it as a rename).
  Token tables preserved verbatim (spot-checked against `tokens.ts`); ratchet
  section now reflects that it's actually enforced; fixed the dangling
  `MIGRATION.md` reference.
- Token ratchet wired in: `package.json` `check:tokens` script, added to the root
  `check` chain (so `make check` covers it) and to `.husky/pre-commit`.
- Fixed the 4 live token violations the unwired ratchet had let accumulate:
  `verified-badge.tsx` `#fff`→`colors.white` (tokenized, theme-aware component);
  `brand.tsx` palette swatch + two `error-boundary.tsx` crash-screen literals
  marked `token-ok` (both intentionally theme-free). Ratchet green at baseline 0.
- `docs/redesign/AUDIT.md`: repointed the live "current values" ref at the skill,
  genericized the historical build-order cell.

### verified

- `pnpm run check:tokens` → 0 literals, exit 0. `pnpm run knip` → exit 0.
  `prettier --check` clean on all 8 changed + 2 new files.
- js/components tsc: zero errors in touched files. brand.tsx change is
  comment-only; zero tsc errors reference it.

### notes / side observations (not fixed)

- `cd js/app && pnpm run check` currently FAILS (exit 2) on PRE-EXISTING stale
  generated lexicons: source `lexicons/place/stream/branding/{export,import}Bundle.json`
  exist but the gitignored generated `js/streamplace/src/lexicons/.../branding.ts`
  (Sep 22) lacks them. Errors are in branding-admin.tsx / player.tsx /
  chat-access.tsx — none in my diff. Fix is `make js-lexicons`. Until then,
  pre-commit is red in this checkout for ANY commit.
- The gap-analysis claim "a new export in js/streamplace/src can fail a commit
  even with zero importers" is FALSE — probed it: knip `include: ["unlisted"]`
  reports undeclared deps, not unused exports, and its project/entry patterns
  match nothing (knip emits "move to workspaces" config hints). knip is
  effectively latent. Documented conservatively in AGENTS.md, no false trap.
- `js/scripts/check-tokens.mjs` usage comment claims `--update` rewrites the
  baseline "only downward", but the code writes the current total unconditionally
  (would ratchet UP too). Documented actual behavior in the skill; left the script
  as-is (out of scope).

## 2026-09-24 (later): humanize + STE100 pass on the docs

Goal: make the docs read well for humans, not just agents. Applied a
humanizer-plus-Simplified-Technical-English pass to `AGENTS.md` and
`docs/go-conventions.md`. STE100 is the controlling standard for these reference
docs, so the humanizer's "add soul / opinions / mess" guidance is deliberately
NOT applied.

What the pass did: active voice and second person ("you"); short sentences split
out of dense multi-clause ones; em dashes replaced with colons/periods; plain
words ("just because" not "merely because", "print" not "emit"); removed
idioms/metaphors ("bite people", "ride along", "cuts the chain", "cascading");
removed the negative-parallelism tail in the Quality gate hook paragraph; no
contractions. Preserved every heading, command, link, the `#quality-gate` anchor,
and all factual content (verified by spot-check + tell-scan).

Also gave `streamplace-design/SKILL.md` a light pass: em dashes to colons,
semicolon splices split, tailing negations and the focus-state parallelism fixed,
contractions expanded, and "reach for" / "must be flawless" / "update the skill"
removed. Kept the reference tables, token values, and the functional bold that
marks hard constraints. Verified the special characters (arrows, range en dashes,
minus signs, ellipsis) and all 14 hex values survived intact.
