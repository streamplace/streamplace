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

| Playwright (`flows/`) | Maestro (`.maestro/`)  |
| --------------------- | ---------------------- |
| `global-setup.ts`     | `00-server-setup.yaml` |
| `01-smoke.spec.ts`    | `01-smoke.yaml`        |
| `02-tabs.spec.ts`     | `02-tabs.yaml`         |
| `03-go-live.spec.ts`  | `03-go-live.yaml`      |
| `04-stream.spec.ts`   | `04-stream.yaml`       |

The web app renders the **desktop layout** (a sidebar of nav links), not the
mobile tab bar, so a couple of flows adapt to that surface while keeping the
same intent: `02-tabs` drives the sidebar (Home ↔ Settings) instead of the tab
bar, and `03-go-live` reaches the streaming entry point via the Live Dashboard
(`/live`), which — logged out — surfaces the same Log In prompt as the mobile
"Go Live → Start streaming" path.

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
