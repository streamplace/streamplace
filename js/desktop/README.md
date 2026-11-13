# Streamplace Desktop

## `pnpm run start`

Boot up Streamplace Desktop for development. Two environment variable options:

- `AQD_SKIP_NODE`: (default `false`): Skip booting up an Streamplace node from
  within the app so you can just test the app against the node you're already
  running locally.

- `AQD_NODE_FRONTEND`: (default `false`): Use frontend bundled with the
  Streamplace node instead of the dev server on `http://127.0.0.1:38081`.
  The bundle is embedded at build time: after frontend changes run `make app`
  and then `make dev` (`make dev` alone skips an existing `js/app/dist`), and
  restart the node that serves it.

Sign-in opens in the system browser. The local node receives the return at
`/app-return/desktop/<token>` and holds it for the app, which polls the node
and completes the login; the browser shows "You're logged in" and may be
closed. If a dev login stays inside the window, the renderer is running an old
bundle: restart `pnpm run start`, and restart Expo for the dev-server frontend.

## `pnpm run package` / `pnpm run make`

Bundles the node binary (`make <platform>-<arch>`) and the OBS plugin from the
same commit (`make obs-plugin-<platform>-<arch>`, in `bin/`). Desktop installs
or updates that plugin in OBS (`src/obs.ts`); `pnpm run start` uses the copy the
last package run unpacked into `out/obs-plugin/`.

## `pnpm test`

Unit tests for the OBS plugin installer and profile writer.

## Desktop E2E (Linux)

Desktop has its own Playwright config and flows in `js/e2e-web/desktop/`.
It shares the web suite's tooling, dummy PDS/PLC harness, and OAuth login helper,
not its full-node scenarios, browser storage state, or live-stream setup.

Inside the pinned builder, after `make provision`:

```sh
make obs-plugin-linux-amd64
pnpm --filter streamplace-desktop package
cargo install --git https://github.com/streamplace/atmoq --rev 09a604b0b00b1eb063295746de1878a258e88d5b --locked atmoq
apt-get update && apt-get install -y xvfb
xvfb-run -a hack/e2e-desktop-local.sh
# Just the login smoke:
xvfb-run -a hack/e2e-desktop-local.sh -- login.spec.ts
```

The runner starts an isolated HTTPS station and dummy PDS, with no media ingest.
Each test launches the real Electron entry point and its own single-user node,
using temporary node/browser/XDG config directories and unused loopback ports.
The login smoke signs in and authorizes at the PDS, returns to Desktop, and checks
the account remains signed in after a reload. The existing lifecycle flow also
checks PDS writes across closing and reopening Desktop.

`SP_E2E_ATMOQ_BIN` selects an installed atmoq executable; `E2E_DESKTOP_MAIN`
overrides the packaged `.webpack/<arch>/main/index.cjs` entry point.
After Desktop source changes, rerun `pnpm --filter streamplace-desktop package`.
After embedded frontend changes, run `make app && make dev` first.

Add Desktop scenarios as `js/e2e-web/desktop/*.spec.ts`, using `test` from
`./fixture`. No web global setup or existing login state is required.
`e2e-desktop` in GitHub Actions runs this suite separately and uploads
`desktop-test-results/`, `desktop-playwright-report/`, and the harness log on
failure. Traces, Electron/node logs, and failure screenshots are attached to
the Playwright report; local results live under `js/e2e-web/`.

## Single-user node

Desktop's bundled node is a single-user AppView: it runs the same indexing as
any node, but only for the accounts logged into it. It starts with
`SP_UPSTREAM_STATION=did:web:stream.place` and
`SP_RELAY_HOST=moqt://streamplace.network`. Set `SP_UPSTREAM_STATION` to
another station DID to override it, or explicitly to an empty string to run an
ordinary standalone node (which then keeps the default relays).

With a station:

- **Firehose.** The node subscribes to one per-DID track per logged-in
  account on each `--relay-host`, which must all be MoQ relays (atmoq serves a
  per-DID track on demand). It never takes the aggregate firehose, and its own
  PDS firehose is left out.
- **Indexing.** Startup, sweeps, history backfill and event handling use the
  same persisted OAuth eligibility; previously indexed repos do not authorize
  new scraping, and no other account's identity is resolved. Logout removes
  eligibility when the last local grant ends. Existing indexed records remain
  on disk, but no longer receive updates. After subscribing or reconnecting,
  the node queues a head check and shallow repair for that DID behind any
  active sweep. Reconnect bursts coalesce into one pending repair per DID; a
  gap during an active repair retains one follow-up.
- **OAuth.** Three independent DPoP sessions: frontend → loopback node → HTTPS
  station → PDS. Only the station needs a public HTTPS callback and retains the
  long-lived PDS session, so logins last as long as a confidential client's.
  The station is the `#streamplace` HTTPS origin in its DID document; startup
  stops if it is missing. The local node's issuer and frontend callback use its
  actual loopback port; `SP_HTTP_ADDR` and `SP_HTTP_INTERNAL_ADDR` can override
  the Desktop listener defaults. Sign-in opens in the system browser, so
  passkeys work; the node's `/app-return/desktop` page holds the callback for
  the running app, which polls and completes the login.
- **Streams.** Unless `SP_PEERS` is set, the node pushes the streams it
  ingests to the station over MoQ (see `docs/moq.md`).
- **Labelers** are not followed: their firehoses carry other accounts.

For development with the embedded frontend, set `AQD_NODE_FRONTEND=true` and
`SP_DEV_FRONTEND_PROXY=` after `make app && make dev`.
Auto-update behavior is unchanged.
