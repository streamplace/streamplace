# Working on Streamplace: a field guide

How to get a Streamplace checkout from "cloned" to "built, tested, reviewed
and stacked as PRs". Everything here was exercised on a Linux box in
September 2026 unless marked *(unverified)* — that mark means the note comes
from earlier notes and was not re-checked when this file was written, so
re-run it before trusting it.

Cold start, in order: §1 (container) → `make dev-setup` (§2) → `make dev`
(§4) → `hack/e2e-web-local.sh` (§5). The first three are prerequisites for
each other; the pitfall list in §7 is worth skimming first if something
fails for no apparent reason.

## 1. Where to work

- You are assigned a sibling checkout, `~/code/streamplace-N`, and (maybe) a
  long-running container named after it. Only edit inside your checkout;
  other siblings (`muxl`, `dasl.ing`, another `streamplace-M`) belong to
  another agent's assignment. Commit freely on branches.
- `~/testvids/STREAMPLACE-N.md` is the canonical per-checkout brief
  (container spin-up, muxl override recipe, commit rules). Read it first;
  this file adds what it does not say.
- Start your container if it is not already up. `docker ps` **hides stopped
  containers**, and `docker exec` on a stopped one fails with "can only
  create exec sessions on running containers":

  ```sh
  docker ps -a --format '{{.Names}}\t{{.Status}}'   # streamplace-N may be Exited
  docker start streamplace-N
  ```

  Sometimes the running container is named `streamplace-N-builder` instead,
  and both can exist at once. Pick whichever is `Up`.
- Check what the container actually mounts, and what its environment already
  provides, before relying on either:

  ```sh
  docker inspect streamplace-N --format '{{range .Mounts}}{{.Source}} -> {{.Destination}}{{"\n"}}{{end}}'
  docker exec streamplace-N printenv PKG_CONFIG_PATH LD_LIBRARY_PATH
  ```

  A container that mounts only your checkout cannot see a sibling's
  `build-linux-amd64`. Containers created from the brief's `docker run`
  recipe also arrive with `PKG_CONFIG_PATH`/`LD_LIBRARY_PATH` already
  pointed at *your* checkout's build dir, so non-interactive `docker exec`
  is not the problem there — but a container created some other way may
  have neither set.

## 2. First build, then the build-and-check loop

A fresh checkout has no `build-linux-amd64/` (the meson-built GStreamer,
FFmpeg, iroh and friends that cgo links against) and no `js/app/dist`
(the Expo web bundle that `pkg/api`, `pkg/media` and `pkg/spxrpc` embed
with `//go:embed all:dist/**`). Build both at once, once:

```sh
make dev-setup     # ~5 min on 16 cores; ~1.6 GB of build-linux-amd64
```

Until that has run, plain Go commands fail on the missing embeds with
`pattern all:dist/**: no matching files found`, and creating empty
`js/app/dist js/web/dist` directories does **not** fix it — the patterns
need real files. If you want to skip the meson half because a sibling
already has one, you still need the frontend half (`pnpm install &&
pnpm run build`, or `make app-cached`).

Once both exist, iteration is fast. If a sibling checkout already has
`build-linux-amd64/`, point at it instead; the `.pc` files carry absolute
paths, so they resolve from any directory on the host:

```sh
export PKG_CONFIG_PATH=/home/iameli/code/streamplace/build-linux-amd64/lib/pkgconfig
export LD_LIBRARY_PATH=/home/iameli/code/streamplace/build-linux-amd64/lib
export CGO_LDFLAGS=-lm
go build ./pkg/... ./cmd/...        # what CI and lint cover
go vet ./pkg/<touched>/...
go test -count=1 ./pkg/<touched>/...
```

- The host toolchain and the container's are the same Go version; the host
  is fine for compile, vet, tests and lint as long as the env above is set.
  Wrap it in a tiny script so every call is identical.
- `go build ./...` also works today (meson leaves only `.c`/`.S` in
  `build-linux-amd64`, which the Go tool ignores), but `./pkg/... ./cmd/...`
  is the shape CI and `golangci-lint` use; if meson ever drops generated Go
  into the build dir, `./...` will try to link it.
- `make lexicons` regenerates Go, JS (gitignored) and the docs
  (`js/docs/src/content/docs/lex-reference/*.md` + `openapi.json`). CI's
  `ci-lexicons` job fails if the committed docs differ from a fresh run, so
  run it and commit the result on every branch that touches `lexicons/`.
  It does not delete outputs for removed lexicons; remove those by hand.
- The full CI check set, in the order that catches the most first:
  1. `go build` / `go vet` as above
  2. `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint run -c ./.golangci.yaml` (needs `PKG_CONFIG_PATH`)
  3. `gofmt -l pkg cmd` must print nothing
  4. `cd js/app && npx tsc -p . --noEmit`
  5. `pnpm run check` (knip + workspace tsc + `prettier --check` over `git ls-files`)
  6. `make lexicons && git status --short` must be clean
- `prettier --check` runs over `git ls-files`, so a file you deleted with
  `rm` but not `git rm` makes it fail with "no files matching". Stage
  deletions.
- The pre-commit hook runs lint-staged (prettier), knip and a full `check`
  of `js/app` against the **gitignored generated lexicon types on disk**,
  which belong to whichever branch last ran `make js-lexicons`. Either
  regenerate before committing or commit with `--no-verify` and run the
  checks yourself; otherwise you get phantom "Property X does not exist on
  type ...lexicons" errors from a different branch.

## 3. Tests: what to run and what to ignore *(unverified)*

- Run targeted packages. `./pkg/media` as a whole takes ~17 minutes and one
  test (`TestMPEGTSVideoMP4AudioToMP4Invalid`) can hang in a GStreamer
  state change and eat the whole timeout; use `-run` filters.
- Known environment-only failures on this box, present on plain `next`:
  `TestChatMessage` in `pkg/atproto` (the devenv PDS the test spawns comes
  up without accounts). Confirm a failure exists on `origin/next` before
  chasing it: `git checkout --detach origin/next && go test -run X ./pkg/...`.
- The GitLab mirror pipeline (`git.stream.place`, reported as a check on
  every PR) runs `make dev-test` **with leak checking on**; GitHub's Linux
  job sets `STREAMPLACE_IGNORE_LEAKS=true`. `TestConcatDemuxBin` has failed
  the leak check on every branch since mid-September, so a red
  `git.stream.place` check is not evidence about your change unless the
  failing test is one you touched. `STREAMPLACE_TEST_COUNT=1` is what CI
  uses for the repeat count.
- `make test-vod` needs the static `build-linux-amd64/streamplace` binary
  and downloads fixtures; it is a CI smoke test, not part of the local loop.
- Two `git worktree`s do not help for cross-branch test comparison: the
  devenv-backed tests need `node_modules` and the built dev-env, which a
  worktree lacks. Check out the other branch in place instead.

## 4. Running a node locally

- `make dev` builds `build-linux-amd64/libstreamplace` and installs a
  launcher at `build-linux-amd64/streamplace` that sets `LD_LIBRARY_PATH`,
  `SP_DEV_FRONTEND_PROXY=http://127.0.0.1:38081` and `SP_DEV_PUBLIC_OAUTH=true`.
  `make dev-web` is the web-only variant.
- The 38080/38081 pair belongs to the long-lived dev node and its metro.
  Never bind those. A scratch stack on alternate ports that coexists with it:

  ```sh
  env -u SP_ACCESS_POLICY SP_DATA_DIR=<scratch> \
    SP_HTTP_ADDR=:38090 SP_HTTP_INTERNAL_ADDR=127.0.0.1:39091 SP_HTTPS_ADDR=:38444 \
    SP_RTMP_ADDR=:19350 SP_RTMPS_ADDR=:19351 SP_RTMPS_ADDON_ADDR=:19352 \
    SP_MIST_ADMIN_PORT=14243 SP_MIST_RTMP_PORT=11936 SP_MIST_HTTP_PORT=28081 \
    SP_DEV_FRONTEND_PROXY=http://127.0.0.1:38091 SP_BROADCASTER_HOST=localhost:38090 \
    ./build-linux-amd64/libstreamplace
  # metro for it, in js/app:
  EXPO_PUBLIC_STREAMPLACE_URL=http://127.0.0.1:38090 npx expo start --port 38091
  ```

  Note `libstreamplace`, not `streamplace`: the launcher hardcodes
  `SP_DEV_FRONTEND_PROXY` as an assignment on its `exec` line, so an
  environment override passed to the launcher is **silently ignored** —
  your "scratch" node would proxy its frontend to the primary node's metro
  on 38081 (watch for `using frontend proxy instead of bundled frontend
  destination=...` in the log to confirm which way it went). Invoke
  `libstreamplace` directly with `LD_LIBRARY_PATH=build-linux-amd64/lib`.
  `SP_DEV_FRONTEND_PROXY=false` (or unset) disables the proxy and serves
  the bundle embedded in the binary instead, which needs no metro at all.
- Do not start metro with `CI=1`: that disables file watching.
- `js/app/.env.development` pins `EXPO_PUBLIC_STREAMPLACE_URL` to the
  38080 node; override it on the command line for a scratch node.
- Branding and other per-node state can be seeded straight into the
  scratch node's `state.sqlite` (`branding_blobs`, `broadcaster_id` is the
  node's `did:web:host:port`). *(unverified)*
- Screenshots and UI checks: Playwright with a Chromium the *harness* can
  launch. On this box `/usr/bin/chromium` does not exist and the system
  browser is a snap whose confinement refuses a profile directory under
  `~/.omp` or `~/.cache` ("Failed to create … SingletonLock: Permission
  denied"); Playwright's own download
  (`~/.cache/ms-playwright/chromium-<rev>/chrome-linux64/chrome`, or
  `pnpm --filter @streamplace/e2e-web install-browser` in the container)
  works as `executablePath`. A persistent profile directory in your
  scratchpad keeps an OAuth session across runs. The iOS-simulator loop for
  the Expo app is the `streamplace-app` skill in `.claude/skills/`.
  *(unverified)*
- A TLS dev environment for a real hostname (own cert in `/shared/codes`,
  `/etc/hosts` entry, `iptables` 443 → the scratch HTTPS port) is what made
  OAuth against a strict PDS work; `SP_DEV_PUBLIC_OAUTH=false SP_SECURE=true
  SP_TLS_CERT/KEY=...` on the node. *(unverified)*

## 5. End-to-end tests

The e2e harness is **not on `next`**. Upstream it lives on `natb/e2e`
(mobile, Maestro) and `natb/e2e-web` (headless Chromium, Playwright); both
drive the same `streamplace e2e` subcommand (`pkg/cmd/e2e.go`), which boots
`js/dev-env/run.mjs` (local PDS + PLC), creates a throwaway account, forks a
server node, registers a stream key, loops a WHIP stream and prints
`SERVER_URL`, `ACCOUNT_HANDLE`, `ACCOUNT_DID`.

### The web (Playwright) suite, as verified

```sh
make dev                                            # harness binary
pnpm --filter @streamplace/e2e-web install-browser  # once; needs root for --with-deps
hack/e2e-web-local.sh                               # harness + flows, 4 tests
```

Run this **inside the container**: the container is not host-networked
(pasta), so harness ports bound in there are not reachable from the host,
and Playwright has to live in the same network namespace. To watch it
directly instead of through the script, `./build-linux-amd64/streamplace e2e`
prints its env on stdout and logs on stderr; the script does exactly that
and then runs `pnpm exec playwright test` in `js/e2e-web`. The global setup
waits for the harness's looping stream to be live before driving the app,
so a run that never goes green usually means the stream, not the UI.

### Running the suite against a branch of your own *(verified on a next-based branch)*

`natb/e2e-web` is hundreds of commits behind, so its harness does not test
your code. Bringing it forward is a small, mechanical port, not a rebase —
graft the files rather than merging the branch:

```sh
git checkout origin/natb/e2e-web -- \
  pkg/cmd/e2e.go js/e2e-web hack/e2e-web-local.sh .maestro
```

then register the command (`makeE2eCommand(build)` in `pkg/cmd/streamplace.go`'s
`app.Commands`) and fix the drift. As of September 2026 that is:

- `pkg/cmd/e2e.go`: the lexicon package is `pkg/placestream`, not
  `pkg/streamplace`; and indigo's `lexutil.LexiconTypeDecoder` refuses glex
  records ("can only handle record fields with const $type"), so write
  records through the repo's own `pkg/comatproto` types with
  `glex.LexiconTypeDecoder{Val: rec}` via `client.Do(...)`.
- `pkg/aqhttp/aqhttp.go`: add the `SP_TRUST_PRIVATE_NETWORK=true` escape
  hatch that `pkg/cmd/e2e.go` already sets, or the node cannot resolve the
  test account's DID through the loopback PLC.
- `js/e2e-web/global-setup.ts`: request
  `place.stream.live.getLiveUsers?limit=50`. Without an explicit `limit`
  the handler truncates the streamer list to 0 and answers `{}` forever, so
  the readiness probe never sees the stream.
- `js/e2e-web/flows/`: the desktop sidebar labels its links `Home` /
  `Settings` (not `Link to /settings`), and `/live` now opens the login
  modal instead of rendering a "Live Dashboard".
- `js/app`, `js/components`: testIDs on the custom-node toggle, URL field,
  save button and home stream card (`aqlink.tsx` needs to forward `testID`),
  plus the guards that stop an undefined node URL from crashing the app.

### Things only running the flows teaches you

- The node's PLC lookup refuses loopback unless `SP_TRUST_PRIVATE_NETWORK=true`.
- iOS collapses whole modals into one accessibility element, so Maestro text
  matches need substrings. *(unverified)*
- Fresh iOS sims show a notifications permission dialog that dims everything
  (pre-grant with `applesimutils`). *(unverified)*
- The web feed fetches once on mount, so the global setup must wait for the
  harness stream to be live.
- A killed harness leaves orphaned node/PDS processes behind. They hold
  ports and their data dirs under `/tmp` look like the run you are
  debugging, so sweep them before re-running:
  `pkill -f 'libstreamplace e2e'; pkill -f 'js/dev-env/run.mjs'`.
- `js/dev-env` needs Node 22 (better-sqlite3 pin).

## 6. Git and GitHub *(unverified)*

- Two GitHub identities are usually present: `GH_TOKEN`/`GITHUB_TOKEN`
  fine-grained PATs in the environment that can push but **cannot create
  PRs** (403 "Resource not accessible by personal access token"), and a
  keyring login from `gh auth login` that can. `gh` prefers the env vars,
  so run PR operations with `env -u GH_TOKEN -u GITHUB_TOKEN gh ...`. Probe
  with `gh api -X POST repos/<o>/<r>/pulls -f title=probe`: a 422 means the
  token is good, a 403 means it is not.
- Stacked PRs use GitHub's `gh stack` extension (`gh extension install
  github/gh-stack`). `gh stack init --base next b1 b2 b3` adopts existing
  branches bottom to top; `gh stack submit --auto` pushes and opens PRs
  (drafts; `--open` for ready, or `gh pr ready N` later); `gh stack sync`
  does the cascading rebase after a lower PR merges or changes, and pushes.
  `gh stack add <branch>` always appends at the **top of the stack**,
  whichever branch you are on; reordering is `gh stack modify`
  (interactive). GitHub refuses `gh pr edit --base` on a PR that is in a
  stack; `gh stack unstack <n>` then re-`init` is the way to restructure.
- History rewriting is routine (message reformats, fixups). Two traps:
  after an autosquash, `branch@{1}` is the pre-autosquash tip, **not** the
  pre-rebase tip, so `git rebase --onto new old@{1} upper` replays the
  wrong range and duplicates commits; take the old tip from the PR's head
  SHA on GitHub instead. And a `fixup!` whose target already contains a
  later commit's additions in the same file will conflict at replay; check
  `git log -S` before choosing the target.
- Rebuilding a long feature branch as layers: cherry-pick commits in the
  original branch order (a helper that sorts SHAs by `git log --reverse`
  saves real pain), drop client-only paths with `git rm --cached` on
  modify/delete conflicts, and resolve "pure addition next to deleted
  code" conflicts mechanically (base→theirs is insert-only, so append the
  inserts to the HEAD side). Anything else, read.
- Commit messages: subject at most 72 characters, blank line, wrapped
  body, `(cherry picked from commit <sha>)` when it applies. No
  `Co-Authored-By` trailer on this repo; reviewers read it as "unreviewed".
- A Greptile review lands on every PR; read it with
  `gh api repos/<o>/<r>/pulls/<n>/comments` (strip the badge HTML), verify
  each finding against the code before fixing, and put the fix **on the
  PR's own branch** when the PR is still open. Only findings on already
  merged PRs get a follow-up PR.

## 7. Small things that cost time

- `build-linux-amd64/` is not branch-aware, and `make dev` only runs
  `dev-setup` when the directory is *missing*. Switch to a branch whose
  `meson.build` differs and you keep the old configuration — including
  whatever subprojects the old branch downloaded. Concrete symptom:
  `make lexicons` dies on `stat ./subprojects/atproto/lexicons: no such
  file or directory` on a pre-glex branch. Either `rm -rf build-linux-amd64
  && make dev-setup`, or fetch the wrap by hand.
- `make dev` / `make app-cached` **silently skip the JS build** when
  `js/app/dist/index.html` exists, printing only "not rebuilding". After a
  branch switch that leaves you with the previous branch's frontend *and*
  `node_modules`, which surfaces later as bizarre failures (e.g. an old
  `lex` CLI rejecting `gen-api`). `rm -rf js/app/dist && pnpm install`
  before `make dev` whenever you change branches.
- `make lexicons` also rewrites `lexicons.json` (trailing newline
  differences after a glex bump); commit it with the regeneration. On some
  branches it ends in `make fix`, which prettier-formats the whole tree —
  expect unrelated diffs afterwards. *(the `lexicons.json` half is unverified)*
- `pnpm add` in `js/app` re-runs `js/streamplace`'s prepare step and drops
  the generated lexicon types; rerun `make js-lexicons`. *(unverified)*
- Package tests that spawn a node write under `SP_DATA_DIR`; keep scratch
  data under your scratchpad, not the checkout. *(unverified)*
- A "failed" GitHub check named `${{ matrix.image }} image` after a force
  push is usually a cancelled run from the superseded commit; open the run
  and look at `conclusion` before rerunning anything. *(unverified)*
- The `sync` (tangled mirror) job rate-limits when many branches push at
  once; `gh run rerun <id> --failed` clears it. *(unverified)*