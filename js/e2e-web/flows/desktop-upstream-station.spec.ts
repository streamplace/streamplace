import {
  _electron,
  expect,
  test,
  type ElectronApplication,
} from "@playwright/test";
import { mkdtemp, rm } from "node:fs/promises";
import { createRequire } from "node:module";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { loginThroughPds } from "./login";

const MAIN = process.env.E2E_DESKTOP_MAIN;
test.skip(
  !MAIN || !process.env.E2E_UPSTREAM_STATION,
  "requires built Electron Desktop and upstream harness",
);

async function freePort(): Promise<number> {
  const server = createServer();
  await new Promise<void>((done) => server.listen(0, "127.0.0.1", done));
  const address = server.address();
  if (!address || typeof address === "string") {
    throw new Error("expected a TCP listener address");
  }
  const port = address.port;
  await new Promise<void>((done, reject) =>
    server.close((error) => (error ? reject(error) : done())),
  );
  return port;
}

// Exercise the actual Electron entry point and its bundled-node launcher, not a
// copied spawn configuration. Mobile counterparts live in upstream-station.yaml.
test("desktop-upstream-station: login and writes survive closing Desktop", async () => {
  test.setTimeout(180_000);
  const desktopDir = resolve(__dirname, "../../desktop");
  const localPort = await freePort();
  const localHost = `127.0.0.1:${localPort}`;
  const nodeURL = `http://${localHost}`;
  const state = await mkdtemp(join(tmpdir(), "streamplace-desktop-upstream-"));
  const env: Record<string, string> = Object.fromEntries(
    Object.entries(process.env).filter(
      ([key, value]) => !key.startsWith("SP_") && value !== undefined,
    ),
  ) as Record<string, string>;
  Object.assign(env, {
    WEBPACK_SERVE: "true",
    AQD_SKIP_NODE: "false",
    AQD_NODE_FRONTEND: "true",
    AQD_NO_UPDATE: "true",
    SP_UPSTREAM_STATION: `did:web:${new URL(process.env.UPSTREAM_STATION_URL!).hostname}`,
    SP_HTTP_ADDR: localHost,
    SP_HTTP_INTERNAL_ADDR: `127.0.0.1:${await freePort()}`,
    SP_RTMP_ADDR: `127.0.0.1:${await freePort()}`,
    SP_DATA_DIR: join(state, "node"),
    SP_BROADCASTER_HOST: localHost,
    SP_DEV_FRONTEND_PROXY: "",
    SP_DEV_PUBLIC_OAUTH: "false",
    SP_TRUST_PRIVATE_NETWORK: "true",
    SP_PLC_URL: process.env.E2E_PLC_URL!,
    SSL_CERT_FILE: process.env.E2E_TLS_CA!,
    HTTPS_PROXY: process.env.E2E_PROXY_URL!,
  });
  const args = [
    "--no-sandbox",
    `--user-data-dir=${join(state, "electron")}`,
    `--proxy-server=${process.env.E2E_PROXY_URL}`,
    "--proxy-bypass-list=127.0.0.1;localhost",
    `--ignore-certificate-errors-spki-list=${process.env.E2E_TLS_SPKI}`,
    resolve(MAIN!),
  ];
  const launch = async (): Promise<ElectronApplication> => {
    const launched = await _electron.launch({
      executablePath: createRequire(join(desktopDir, "package.json"))(
        "electron",
      ) as string,
      args,
      cwd: desktopDir,
      env,
    });
    // Include the real launcher/node's startup errors in CI failure output.
    launched
      .process()
      .stdout?.on("data", (chunk) => process.stdout.write(chunk));
    launched
      .process()
      .stderr?.on("data", (chunk) => process.stderr.write(chunk));
    return launched;
  };
  let app: ElectronApplication | undefined;
  try {
    app = await launch();
    let page = await app.firstWindow();
    await expect(page).toHaveURL(nodeURL + "/", { timeout: 60_000 });
    await loginThroughPds(page, nodeURL);
    await page.goto(nodeURL + "/settings/privacy");
    const toggleName = `Allow ${localHost} to record your livestream for debugging and improving the service`;
    const toggle = page.getByRole("switch", { name: toggleName });
    await expect(toggle).toBeVisible();
    const initial = (await toggle.getAttribute("aria-checked")) === "true";
    const saved = page.waitForResponse(
      (response) =>
        new URL(response.url()).origin === nodeURL &&
        new URL(response.url()).pathname === "/xrpc/com.atproto.repo.putRecord",
    );
    await toggle.click();
    expect((await saved).ok()).toBe(true);
    await expect(toggle).toHaveAttribute("aria-checked", String(!initial));
    // Read the actual PDS record: an optimistic toggle alone proves no write.
    const recordURL = new URL(
      "/xrpc/com.atproto.repo.getRecord",
      process.env.PDS_HTTPS_URL!,
    );
    recordURL.search = new URLSearchParams({
      repo: process.env.ACCOUNT_DID!,
      collection: "place.stream.server.settings",
      rkey: localHost,
    }).toString();
    const persisted = await page.request.get(recordURL.toString(), {
      ignoreHTTPSErrors: true,
    });
    expect(persisted.ok()).toBe(true);
    expect((await persisted.json()).value.debugRecording).toBe(!initial);
    await app.close();
    app = undefined;
    app = await launch();
    page = await app.firstWindow();
    await page.goto(nodeURL + "/settings/privacy");
    await expect(
      page.getByRole("switch", { name: toggleName }),
    ).toHaveAttribute("aria-checked", String(!initial));
    const restoredWrite = page.waitForResponse(
      (response) =>
        new URL(response.url()).origin === nodeURL &&
        new URL(response.url()).pathname === "/xrpc/com.atproto.repo.putRecord",
    );
    await page.getByRole("switch", { name: toggleName }).click();
    expect((await restoredWrite).ok()).toBe(true);
    await expect(
      page.getByRole("switch", { name: toggleName }),
    ).toHaveAttribute("aria-checked", String(initial));
  } finally {
    await app?.close();
    await rm(state, { recursive: true, force: true });
  }
});
