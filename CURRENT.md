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

## 2026-02-06: stream moderation delegation in js/web

Ported the delegation model from js/app (js/components) to the true-DOM web
app, on natb/cleanup-web-ui.

### what changed

- `js/core` websocket-consumer: handles
  `place.stream.moderation.defs#permissionView` (server publishes it on
  delegation create); merges into `moderationPermissions`, deduped by
  moderator+createdAt (records are immutable). Fixes real-time grants for
  js/app too — the store field existed but nothing consumed the pushes.
- `js/web/src/lib/moderation.ts`: pure logic — record filtering +
  `moderationPermissionsFor` (owner short-circuit, union of unexpired
  delegations). Tested in `lib/moderation.test.ts`.
- `js/web/src/hooks/use-can-moderate.ts`: binds logic to session agent +
  LivestreamStore; listRecords fetch deduped across the three surfaces that
  mount it (chat panel, stream info, pinned banner).
- `js/web/src/hooks/use-moderation-actions.ts`: blockUser / hideMessage /
  pinMessage / unpinMessage / updateStreamTitle / submitReport — owner
  writes own repo, delegated moderators hit place.stream.moderation.\*
  (server-side CheckPermission is the authority).
- chat: per-message "…" hover menu (pin durations, hide w/ optimistic
  reduceChat filter, block, report, delete-own two-click) replacing the
  owner-only pin button; pinned banner unpin works for moderators with
  message.pin.
- reporting: `stream/report-dialog.tsx` ports the app's report modal (six
  AT Protocol reason types + optional comment, createReport via submitReport).
  Entry point: report others' messages from the chat menu. Dialog takes any
  subject, so stream-record/account entry points can reuse it.
- dashboard: `moderators` widget (registry + `dashboard/moderators.tsx`) —
  list/add/remove permission records in own repo, handle resolution,
  permission switches, expiry display.
- stream page: pencil beside title when canManageLivestream → title-only
  dialog (owner: chapter-marker record; mod: updateLivestream XRPC).
- i18n: en-US keys under ## Moderation / ## Moderators / ## Report + chat &
  stream-info additions; compiled via `pnpm --filter @streamplace/i18n compile`.

### verified

- js/core vitest 53/53 (new websocket-consumer.test.ts), js/web vitest 101/101,
  web tsc clean, i18n check passes.
- NOT tested end-to-end: needs two OAuth accounts (streamer + moderator)
  against a live node; delegated XRPC paths are byte-for-byte ports of the
  js/components calls js/app ships with.

### gotchas

- lexicon-typed client brands did/uri/datetime strings; delegated calls cast
  those fields `as any` (same as js/components).
- i18next key separator is "." — never put lexicon strings like
  `livestream.manage` inside key names (used `moderators-permission-manage`).
- server publishes permissionView on CREATE only; deletions aren't pushed, so
  a revoked moderator keeps flags client-side until reload (server still
  rejects the action) — same gap exists in js/app.
- lint-staged stash-dance breaks if a file is partially staged while a
  concurrent process edits it; unstage everything before running commits.

## 2026-09-08: split natb/cleanup-web-ui into two stacked PRs

- `natb/web-moderation-delegation`: 12 feature commits cherry-picked onto
  origin/next (core permissionView through stream-level report menu).
- `natb/web-ui-cleanup`: 7 cleanup commits stacked on the feature branch
  (sidebar scroll, debug toggle, comment-out link, branding agent wait,
  decorator signal, multistream trim, i18n nits sweep).
- Verified: `git diff` between natb/web-ui-cleanup and the original
  natb/cleanup-web-ui tip is empty (tree-identical). Feature branch alone:
  core vitest 53/53, web vitest 101/101, web tsc clean, core check + i18n
  compile clean. Neither branch pushed yet.
