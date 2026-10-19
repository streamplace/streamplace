# Streamplace Desktop

## `pnpm run start`

Boot up Streamplace Desktop for development. Two environment variable options:

- `AQD_SKIP_NODE`: (default `false`): Skip booting up an Streamplace node from
  within the app so you can just test the app against the node you're already
  running locally.

- `AQD_NODE_FRONTEND`: (default `false`): Use frontend bundled with the
  Streamplace node instead of the dev server on `http://localhost:8081`.

## `pnpm run package` / `pnpm run make`

Bundles the node binary (`make <platform>-<arch>`) and the OBS plugin from the
same commit (`make obs-plugin-<platform>-<arch>`, in `bin/`). Desktop installs
or updates that plugin in OBS (`src/obs.ts`); `pnpm run start` uses the copy the
last package run unpacked into `out/obs-plugin/`.

## `pnpm test`

Unit tests for the OBS plugin installer and profile writer.

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
  the Desktop listener defaults.
- **Streams.** Unless `SP_PEERS` is set, the node pushes the streams it
  ingests to the station over MoQ (see `docs/moq.md`).
- **Labelers** are not followed: their firehoses carry other accounts.

For development with the embedded frontend, set `AQD_NODE_FRONTEND=true` and
`SP_DEV_FRONTEND_PROXY=` after `make app && make dev`.
Auto-update behavior is unchanged.
