# Working on Streamplace as an agent

Field notes on getting a Streamplace checkout from "cloned" to "built,
tested, reviewed and stacked as PRs". Everything here was exercised on a
Linux box in September 2026; recipes marked *earlier* come from prior
sessions' notes and were verified then, not re-run for this document.

## 1. Where to work

- You are assigned a sibling checkout, `~/code/streamplace-N`, and (maybe) a
  long-running container named after it. Only edit inside your checkout;
  other siblings (`muxl`, `dasl.ing`, another `streamplace-M`) belong to
  someone else's session. Commit freely on branches.
- `~/testvids/STREAMPLACE-N.md` is the canonical per-checkout brief
  (container spin-up, muxl override recipe, commit rules). Read it first;
  this file adds what it does not say.
- Check what the container actually mounts before relying on it:
  `docker inspect <name> --format '{{range .Mounts}}{{.Source}} -> {{.Destination}}{{"\n"}}{{end}}'`.
  A container that mounts only your checkout cannot see a sibling's
  `build-linux-amd64`, and non-interactive `docker exec` starts with an
  empty `PKG_CONFIG_PATH`.

## 2. The build-and-check loop that works

A fresh checkout has no `build-linux-amd64/` (the meson-built GStreamer,
FFmpeg, iroh and friends that cgo links against). `make dev-setup` builds
it, slowly. If a sibling checkout already has one, point at it instead;
the `.pc` files carry absolute paths, so they resolve from any directory on
the host:

```sh
export PKG_CONFIG_PATH=/home/iameli/code/streamplace/build-linux-amd64/lib/pkgconfig
export LD_LIBRARY_PATH=/home/iameli/code/streamplace/build-linux-amd64/lib
export CGO_LDFLAGS=-lm
go build ./pkg/... ./cmd/...        # not ./..., see below
go vet ./pkg/<touched>/...
go test -count=1 ./pkg/<touched>/...
```

- Never `go build ./...`: `build-linux-amd64/` contains meson conftest `.c`
  files that Go treats as a package and fails to link.
- The host toolchain and the container's are the same Go version; the host
  is fine for compile, vet, tests and lint as long as the env above is set.
  Wrap it in a tiny script so every call is identical.
- `make lexicons` regenerates Go (`pkg/placestream`), JS
  (`js/streamplace/src/lexicons`, gitignored) and the docs
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
- The pre-commit hook runs lint-staged, knip and a full `tsc` of `js/app`
  against the **gitignored generated lexicon types on disk**, which belong
  to whichever branch last ran `make js-lexicons`. Either regenerate before
  committing or commit with `--no-verify` and run the checks yourself;
  otherwise you get phantom "Property X does not exist on type ...lexicons"
  errors from a different branch.

## 3. Tests: what to run and what to ignore

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

## 4. Running a node locally (*earlier*, verified in prior sessions)

- `make dev` builds `build-linux-amd64/libstreamplace` and installs a
  launcher at `build-linux-amd64/streamplace` that sets `LD_LIBRARY_PATH`,
  `SP_DEV_FRONTEND_PROXY=http://127.0.0.1:38081` and
  `SP_DEV_PUBLIC_OAUTH=true`. `make dev-web` is the web-only variant.
- Eli's own dev node lives on 38080 (backend) and 38081 (metro). Never
  bind those. A scratch stack on alternate ports that coexisted with it:

  ```sh
  env -u SP_ACCESS_POLICY SP_DATA_DIR=<scratch> \
    SP_HTTP_ADDR=:38090 SP_HTTP_INTERNAL_ADDR=127.0.0.1:39091 SP_HTTPS_ADDR=:38444 \
    SP_RTMP_ADDR=:19350 SP_RTMPS_ADDR=:19351 SP_RTMPS_ADDON_ADDR=:19352 \
    SP_MIST_ADMIN_PORT=14243 SP_MIST_RTMP_PORT=11936 SP_MIST_HTTP_PORT=28081 \
    SP_DEV_FRONTEND_PROXY=http://127.0.0.1:38091 SP_BROADCASTER_HOST=localhost:38090 \
    ./build-linux-amd64/streamplace
  # metro for it, in js/app:
  EXPO_PUBLIC_STREAMPLACE_URL=http://127.0.0.1:38090 npx expo start --port 38091
  ```

  Do not start metro with `CI=1`: that disables file watching.
- `js/app/.env.development` pins `EXPO_PUBLIC_STREAMPLACE_URL` to the
  38080 node; override it on the command line for a scratch node.
- Branding and other per-node state can be seeded straight into the
  scratch node's `state.sqlite` (`branding_blobs`, `broadcaster_id` is the
  node's `did:web:host:port`).
- Screenshots and UI checks: Playwright with the system Chromium
  (`executablePath: /usr/bin/chromium`), a persistent profile directory in
  your scratchpad keeps an OAuth session across runs. The iOS-simulator
  loop for the Expo app is the `streamplace-app` skill in `.claude/skills/`.
- A TLS dev environment for a real hostname (own cert in `/shared/codes`,
  `/etc/hosts` entry, `iptables` 443 → the scratch HTTPS port) is what made
  OAuth against a strict PDS work; `SP_DEV_PUBLIC_OAUTH=false SP_SECURE=true
  SP_TLS_CERT/KEY=...` on the node.

## 5. End-to-end tests (*earlier*, branch-specific)

The e2e harness is not on `next` as of this writing. It lives on
`natb/e2e` (mobile, Maestro) and `natb/e2e-web` (headless Chromium,
Playwright); both drive the same `streamplace e2e` subcommand
(`pkg/cmd/e2e.go`), which boots `js/dev-env/run.mjs` (local PDS + PLC),
creates a throwaway account, forks a server node, registers a stream key,
loops a WHIP stream and prints `SERVER_URL`, `ACCOUNT_HANDLE`,
`ACCOUNT_DID`.

- Local run: `make dev`, then `./build-linux-amd64/streamplace e2e` (env on
  stdout, logs on stderr), then `maestro test -e APP_ID=... -e
  SERVER_URL=http://10.0.2.2:<port> -e ACCOUNT_HANDLE=<handle> .maestro/`
  against an Android emulator, or `hack/e2e-web-local.sh` for the web
  flows. `js/dev-env` needs Node 22 (better-sqlite3 pin).
- Things that were only found by running the flows, and are cheap to
  remember: the node's PLC lookup refuses loopback unless
  `SP_TRUST_PRIVATE_NETWORK=true`; iOS collapses whole modals into one
  accessibility element, so Maestro text matches need substrings; fresh
  iOS sims show a notifications permission dialog that dims everything
  (pre-grant with `applesimutils`); the web feed fetches once on mount, so
  the global setup must wait for the harness stream to be live.

## 6. Git and GitHub

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

- The doc file for containers says `docker exec streamplace-N`; the actual
  container may be named `streamplace-N-builder`. `docker ps` first.
- `make lexicons` also rewrites `lexicons.json` (trailing newline
  differences after a glex bump); commit it with the regeneration.
- `pnpm add` in `js/app` re-runs `js/streamplace`'s prepare step and drops
  the generated lexicon types; rerun `make js-lexicons`.
- Package tests that spawn a node write under `SP_DATA_DIR`; keep scratch
  data under your session scratchpad, not the checkout.
- A "failed" GitHub check named `${{ matrix.image }} image` after a force
  push is usually a cancelled run from the superseded commit; open the run
  and look at `conclusion` before rerunning anything.
- The `sync` (tangled mirror) job rate-limits when many branches push at
  once; `gh run rerun <id> --failed` clears it.
