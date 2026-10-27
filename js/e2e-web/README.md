# e2e-web (Playwright)

Headless-web e2e flows for the Streamplace app — the web counterpart to the
mobile `.maestro/` suite. Shared harness (`streamplace e2e`) and testIDs;
Playwright headless Chromium drives the web-specific surface because Maestro
is mobile-only.

## How it fits together

- **Harness:** `streamplace e2e` (see `pkg/cmd/e2e.go`) boots a node + local
  PDS/PLC + a looping WHIP test stream and prints `SERVER_URL` /
  `ACCOUNT_HANDLE` / `ACCOUNT_DID` / `STREAM_KEY` (and `E2E_FIXTURE_MP4`, the
  local file it streams). The node also serves the web app (embedded
  via `//go:embed all:dist/**`), so Playwright just points a browser at
  `SERVER_URL`. `SP_E2E_INGEST=fmp4` loops the stream through the OBS plugin's
  fragmented-MP4 ingest instead (CI runs `flows/stream.spec.ts` that way too),
  and `SP_E2E_INGEST=none` streams nothing, so you can push with `STREAM_KEY`
  from OBS yourself. `SP_E2E_VIDEO=av1` streams
  `test/fixtures/av1-h264-opus.mp4` instead: an AV1 source track with an H.264
  fallback track over fMP4, the AV1 track alone over WHIP. `flows/av1.spec.ts`
  runs only then (CI runs it both ways, `flows/stream.spec.ts` too over fMP4).
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

`invalid-handle` runs against both Expo web and the Vite beta. It moves the
harness account to a real `.test` handle, which cannot pass public
bidirectional verification, and calls `com.atproto.identity.refreshIdentity`.
The home feed must display the DID without `@`, open its DID-addressed stream,
and play real media. It also exercises an invalid typeahead result. Only public
AppView discovery (the harness account is not publicly indexed) and that search
boundary are substituted; the feed, PDS/PLC, refresh endpoint, websocket and
playback remain real. Cleanup restores the original verified handle and waits
for the live-feed cache to catch up, including after a failed assertion.
The corresponding shared native flow is
`.maestro/logged-out/invalid-handle.yaml`.
The login typeahead case changes an invalid result back to a verified handle
without changing its DID or avatar, then accepts the corrected completion. It
covers the shared login hook; the public AppView boundary is substituted because
the local PDS account is not publicly searchable.

The web app renders the **desktop layout** (a sidebar of nav links), not the
mobile tab bar, so `tabs` drives Home ↔ Settings through the sidebar.
`go-live` checks that the Live Dashboard (`/live`) requires login; this is
web-only coverage, with no current native Go Live scenario. Chat popout is also
web-only, as is `chat-wheel-scroll` (trackpad wheel input in the inverted
chat list), which also runs in a Firefox project because the bug it covers
was reported there. `vod` opens the VOD the harness publishes and checks both the node's
page title and the app's tab title; document titles have no native counterpart.
`branding-favicons` exercises authenticated light/dark favicon uploads, scheme
switches, persistence and deletion fallbacks, including the HTML before
hydration. It also decodes `/favicon.ico` as a browser image, verifying the ICO
signature, MIME type, branded pixels and bundled fallback after deletion.
It is web-only: native apps have no browser tab favicon, and branding image
uploads are already limited to the web.

`stream` also covers the narrow portrait web layout at 320 × 568, where the
live video sits above chat. It reveals the player chrome, exercises mute and
fullscreen entry/exit, and verifies that faded controls reveal instead of
accepting an unseen tap. Its readiness check uses the video itself: accumulated
chat history can virtualize the stream's initial system message, so that row is
not a reliable player prerequisite.

`player-viewport` opts into the Vite web beta and starts the real harness's HLS
stream at phone, tablet, square, and desktop sizes. After confirming playback,
it pauses through the normal shortcut and checks control visibility, viewport
containment, opacity, and pointer hit targets with chat open and closed. It
changes volume in both states, opens Settings, and checks fullscreen entry/exit
and control availability in fullscreen. Cases straddle the square-layout and
player-width boundaries; compact players retain theatre and danmu in Settings.
The touch case uses the embed player to verify 44px buttons and a working volume
hit area at narrow and tablet widths. Public AppView identity discovery is
substituted in the chat cases because the harness account is not publicly
indexed. The embed test uses real services throughout. This is web-only: iOS
and Android use the separate Expo/native player.

`player-breakpoints` exercises live playback at 23 widths in portrait and
landscape, including 1080 × 722 and immediately below, at, and above the theme
breakpoints (640, 768, 1024, 1280, 1536), sidebar boundary (981), and desktop
player boundary (1201). It checks that the mute button and whole volume slider
are fully inside the viewport and change the actual video element's audio state.
Side-chat layouts repeat those checks with chat open, closed, and reopened. This catches
controls clipped below the web shell's header: the compact player's height must
fill its available parent, not the whole browser window. These controls are
web-only; native live playback uses device volume controls.

`auto-publish-vods` saves and reloads the automatic publishing preference,
checks that the stream form's Create tab and Privacy & Security share it,
including the popout form, and hides the control without VOD beta access or
outside the Create tab. The compact checkbox keeps persistent-consent details in
its tooltip. The native Maestro flow opens the shared form by deep link, enables
the preference there, and verifies persistence in settings after a relaunch.
The browser flow also holds a recording opt-out request to verify another privacy toggle cannot
overwrite it while saving. Deleting the node's settings record also withdraws
automatic publishing consent; the PDS-backed regression for that index deletion
lives in `pkg/atproto/auto_publish_vod_test.go`. It also injects a deletion failure
and checks that later deletions still run while the commit watermark advances
only after successful reprocessing.

`syndication` plays the test stream from the harness's second node, which pulls
it from the first over Media over QUIC (`SERVER2_URL`; see `docs/moq.md`).
`push` plays it from the third node, which the first node pushes it to
(`SERVER3_URL`, the first node's `--peers`). Both are web-only: node-to-node
transfer has no mobile surface, and the mobile flows play from the ingest node.

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

## Upstream-station topology

This is opt-in; the ordinary single-node suite is unchanged. Build the real
atmoq relay at commit `09a604b0b00b1eb063295746de1878a258e88d5b` with locked
Cargo dependencies, as CI does:

```bash
cargo install --git https://github.com/streamplace/atmoq \
  --rev 09a604b0b00b1eb063295746de1878a258e88d5b --locked atmoq
```

This includes the certificate-watcher fix from
[atmoq#20](https://github.com/streamplace/atmoq/pull/20): v0.1.1 treats Linux
certificate reads as changes, causing a continuous reload loop that can starve
live indexing. Build in an isolated directory rather than changing a sibling
checkout. Set `SP_E2E_ATMOQ_BIN` to the executable when it is not on `PATH`.

```bash
# Inside the pinned builder, after make app && make dev:
SP_E2E_ATMOQ_BIN=/path/to/atmoq \
  hack/e2e-web-local.sh --upstream-station -- upstream-station.spec.ts
# Equivalent flag source:
SP_E2E_UPSTREAM_STATION=true SP_E2E_ATMOQ_BIN=/path/to/atmoq \
  hack/e2e-web-local.sh upstream-station.spec.ts
```

The harness starts the existing PDS/PLC, an HTTPS upstream station, a separate
HTTP loopback node configured with `SP_UPSTREAM_STATION=did:web:<station>`,
and `atmoq serve` bridging the PDS's actual WebSocket firehose to QUIC.
The loopback node takes the bridge as its `SP_RELAY_HOST`. QUIC verifies the
throwaway CA and the leaf's loopback IP SAN; there is no insecure TLS override,
mock frame source or fake protocol server. The bridge uses a one-frame group
for sparse per-user traffic. URLs printed as evidence are `SERVER_URL`,
`UPSTREAM_STATION_URL` and `ATMOQ_RELAY_URL`.

The browser completes local → station → PDS login and consent, writes through
both independently DPoP-bound proxies, and verifies the exact returned record
URI appears in the local durable index after initial login sync. It checks the
real indexed PDS remains the PDS, while the local OAuth session targets the
station. A separately created, followed-but-unrelated account emits real PDS
commits before and after login. The local transport audit checks it is neither
scraped nor enumerated, and the local DB checks it is not indexed. Released
relay logs must show the user's DID track and no aggregate `atproto` track:
bandwidth selection comes from these per-DID subscriptions, not from merely
replacing WebSocket with MoQ. The bridge itself still ingests its PDS firehose.

Renewal coverage uses the actual browser's persisted DPoP key and local
`/oauth/token`. Only the stored access-token renewal timestamps are advanced
to make genuine station/PDS refresh deterministic; keys, tokens and protocol
responses are never faked. A grant-scoped fixture trigger keeps the old token's
timestamp expired through concurrent nonce writes; genuine token rotation ends
that override. Each attempt replaces the trigger so CI retries can reuse the
harness databases. A harness-stdin `restart-local` command restarts the real Go node
with its on-disk sessions and acknowledges readiness in a private file.
Authenticated writes and indexed history must survive that restart.
Real browser logout revokes the local downstream session and stops the final
local DID subscription without destroying the station's confidential PDS grant.
Chat submission waits for the restored OAuth session and initial streamer
profile; Enter during initialization leaves the draft intact rather than
clearing a message that cannot yet be sent.

Prerequisites additionally include Python 3 with SQLite support (for fixture
inspection), a POSIX shell with `mkfifo`, free loopback HTTPS/TCP/UDP ports, and
the HTTPS DNS/trust prerequisites above. The FIFO, child process groups, TLS
keys, temporary databases and relay store are owned by and removed with the run.
Browser traces may contain temporary test-account credentials; treat the test
artifacts as sensitive and do not reuse these accounts outside the harness.

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
