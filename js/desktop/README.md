# Streamplace Desktop

## `pnpm run start`

Boot up Streamplace Desktop for development. Two environment variable options:

- `AQD_SKIP_NODE`: (default `false`): Skip booting up an Streamplace node from
  within the app so you can just test the app against the node you're already
  running locally.

- `AQD_NODE_FRONTEND`: (default `false`): Use frontend bundled with the
  Streamplace node instead of the dev server on `http://localhost:8081`.

## Upstream station mode

Desktop starts its bundled node with `SP_UPSTREAM_STATION=did:web:stream.place`.
Set `SP_UPSTREAM_STATION` to another station DID to override it, or explicitly
set it to an empty string to run the standalone server mode.

The station must advertise both `#streamplace` (an HTTPS origin) and `#atmoq`
(a raw-QUIC MoQ origin) in its DID document. Missing or invalid advertisements
stop startup; Desktop never falls back to the full public firehose.

The local node subscribes to exact-DID tracks only for successfully authenticated
local users. Startup, sweeps, history backfill and event handling use the same
persisted OAuth eligibility; previously indexed repos do not authorize new
scraping. Logout removes eligibility when the last local grant ends. Existing
indexed records remain on disk, but no longer receive updates.
After subscribing or reconnecting, the node queues a head check and shallow
repair for that DID behind any active sweep. Sparse tracks do not depend on
firehose replay or wait for the next periodic sweep to recover missed writes.

OAuth has three independent DPoP sessions: frontend → loopback node → HTTPS
station → PDS. Only the station needs a public HTTPS callback and retains the
long-lived PDS session. The local node's issuer and frontend callback use its
actual loopback port; `SP_HTTP_ADDR` and `SP_HTTP_INTERNAL_ADDR` can override
the Desktop listener defaults.

For development with the embedded frontend, set `AQD_NODE_FRONTEND=true` and
`SP_DEV_FRONTEND_PROXY=` after `make app && make dev`.
Auto-update behavior is unchanged.
