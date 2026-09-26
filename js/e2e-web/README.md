# e2e-web (Playwright)

Headless-web e2e flows for the Streamplace app — the web counterpart to the
mobile `.maestro/` suite. Same harness (`streamplace e2e`), same testIDs, same
flows; a different driver (Playwright headless Chromium) because Maestro is
mobile-only.

## How it fits together

- **Harness:** `streamplace e2e` (see `pkg/cmd/e2e.go`) boots a node + local
  PDS/PLC + a looping WHIP test stream and prints `SERVER_URL` /
  `ACCOUNT_HANDLE` / `ACCOUNT_DID`. The node also serves the web app (embedded
  via `//go:embed all:dist/**`), so Playwright just points a browser at
  `SERVER_URL`.
- **Shared testIDs:** react-native-web maps `testID` -> `data-testid`, so the
  very same IDs the Maestro flows use (`home-stream-card`,
  `settings-use-custom-node`, `settings-custom-node-url`, `settings-save-node`)
  are Playwright selectors here — no separate web instrumentation.
- **Server setup:** the app ships pointed at production, so `global-setup.ts`
  opens `/settings/advanced`, enters the test node's URL, and saves the
  resulting browser state (`storageState`). Every flow reuses it and starts
  already pointed at the harness — the web analogue of
  `.maestro/00-server-setup.yaml`.

## Flow ↔ Maestro parity

| Playwright (`flows/`)    | Maestro (`.maestro/`)  |
| ------------------------ | ---------------------- |
| `global-setup.ts`        | `00-server-setup.yaml` |
| `01-smoke.spec.ts`       | `01-smoke.yaml`        |
| `02-tabs.spec.ts`        | `02-tabs.yaml`         |
| `03-go-live.spec.ts`     | `03-go-live.yaml`      |
| `04-stream.spec.ts`      | `04-stream.yaml`       |
| `05-oauth-login.spec.ts` | —                      |

The web app renders the **desktop layout** (a sidebar of nav links), not the
mobile tab bar, so a couple of flows adapt to that surface while keeping the
same intent: `02-tabs` drives the sidebar (Home ↔ Settings) instead of the tab
bar, and `03-go-live` reaches the streaming entry point via the Live Dashboard
(`/live`), which — logged out — surfaces the same Log In prompt as the mobile
"Go Live → Start streaming" path.

`04-stream` also covers the narrow portrait web layout at 320 × 568, where the
live video sits above chat. It reveals the player chrome, exercises mute and
fullscreen entry/exit, and verifies that faded controls reveal instead of
accepting an unseen tap.

## OAuth over real HTTPS

`05-oauth-login` logs in the way a user does, through the node's OAuth proxy
and the local PDS's sign-in and consent pages. atproto OAuth will not run over
plain HTTP, with ports, or on `.test`-style names, and parts of it resolve
`did:plc` against a hardcoded `https://plc.directory`. So
`hack/e2e-web-local.sh` starts the harness with public DNS names for 127.0.0.1:
`--https-pds-hostname localhost-pds.streamplace.network` (plus a wildcard record
under it, for account handles) and
`--https-station-hostname localhost-station.streamplace.team` (the node's
broadcaster host: the app and the OAuth client). The two must be on different
registrable domains, as in production — the PDS rejects a same-site navigation
to its sign-in page. In that mode the harness:

- mints a throwaway CA and serves both on 127.0.0.1:443, routed by SNI (so it
  must be able to bind 443 — the container runs as root);
- publishes this build's lexicons into a local lexicon authority account (the
  stand-in for the account behind `_lexicon.stream.place`) and has the PDS
  resolve lexicons from it, so the OAuth permission set the app asks for
  (`include:place.stream.authFull`) is this branch's, not production's;
- runs a small proxy that sends `plc.directory` to the local PLC; the node and
  the browser use it;
- has each process trust the CA on its own (Go `SSL_CERT_DIR`, Node
  `NODE_EXTRA_CA_CERTS`, Chromium the leaf's SPKI) — nothing is installed
  system-wide;
- additionally prints `SERVER_HTTPS_URL`, `PDS_HTTPS_URL`, `E2E_PROXY_URL` and
  `E2E_TLS_SPKI`, which the config and the flow pick up.

The other flows keep using the plain-HTTP `SERVER_URL`. Run with
`E2E_HTTPS_PDS_HOSTNAME=` to skip HTTPS; the OAuth flow then skips itself.
Details are in `pkg/cmd/e2e_https.go`.

## Run it locally

```bash
# once: install the browser
pnpm --filter @streamplace/e2e-web install-browser

# build a streamplace binary that embeds the web app, then run the suite
make dev
hack/e2e-web-local.sh
```

`hack/e2e-web-local.sh` starts the harness, waits for `SERVER_URL`, runs the
flows, and tears the harness down. On a host without the cgo runtime, run it
inside the build container.

Artifacts on failure (traces, screenshots, video) land in `test-results/` and a
report in `playwright-report/` (`pnpm --filter @streamplace/e2e-web report`).
