# e2e-web (Playwright)

Headless-web e2e flows for the Streamplace app — the web counterpart to the
mobile `.maestro/` suite. Shared harness (`streamplace e2e`) and testIDs;
Playwright headless Chromium drives the web-specific surface because Maestro
is mobile-only.

## How it fits together

- **Harness:** `streamplace e2e` (see `pkg/cmd/e2e.go`) boots a node + local
  PDS/PLC + a looping WHIP test stream and prints `SERVER_URL` /
  `ACCOUNT_HANDLE` / `ACCOUNT_DID` (and `E2E_FIXTURE_MP4`, the local file it
  streams). The node also serves the web app (embedded
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
  `.maestro/setup/server-setup.yaml`.

## Adding scenarios

Name specs `flows/<feature-or-behavior-slug>.spec.ts`, without sequence numbers.
Playwright automatically discovers specs under `flows/`; adding a scenario
does not require editing the config, a manifest, or this README.

Each test must establish its own prerequisites: navigate to the page it needs,
log in through `loginThroughPds` when authentication is required, and wait for
the relevant stream or application state. Tests get isolated browser contexts
seeded with the harness node configuration, not authentication from another
test. Never rely on prior test state or filename execution order. The suite runs
serially to limit contention on its shared harness account and looping stream,
not to establish dependencies between scenarios.

## Mobile coverage convention

For shared behavior, use the corresponding semantic slug and mention its mobile
path in the spec comment; no central parity manifest is needed. Logged-out
smoke, navigation, and stream coverage correspond to
`.maestro/logged-out/{smoke,tabs,stream}.yaml`. OAuth setup corresponds to
`.maestro/setup/oauth-login.yaml`; authenticated chat/profile behavior is covered
on mobile by `.maestro/logged-in/{chat-reply,chat-profile}.yaml`.

The web app renders the **desktop layout** (a sidebar of nav links), not the
mobile tab bar, so `tabs` drives Home ↔ Settings through the sidebar.
`go-live` checks that the Live Dashboard (`/live`) requires login; this is
web-only coverage, with no current native Go Live scenario. Chat popout is also
web-only, as is `chat-wheel-scroll` (trackpad wheel input in the inverted
chat list), which also runs in a Firefox project because the bug it covers
was reported there. `vod` opens the VOD the harness publishes and checks both the node's
page title and the app's tab title; document titles have no native counterpart.

`stream` also covers the narrow portrait web layout at 320 × 568, where the
live video sits above chat. It reveals the player chrome, exercises mute and
fullscreen entry/exit, and verifies that faded controls reveal instead of
accepting an unseen tap.

## OAuth over real HTTPS

`oauth-login` logs in the way a user does, through the node's OAuth proxy
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

Unauthenticated flows keep using the plain-HTTP `SERVER_URL`; OAuth and chat
popout tests establish their own login using the HTTPS URLs. Run with
`E2E_HTTPS_PDS_HOSTNAME=` to skip HTTPS; those tests then skip themselves.
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
