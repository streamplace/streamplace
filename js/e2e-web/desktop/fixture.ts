import {
  _electron,
  test as base,
  expect,
  type ElectronApplication,
  type Page,
} from "@playwright/test";
import { createSocket } from "node:dgram";
import { once } from "node:events";
import { createWriteStream } from "node:fs";
import { mkdtemp, rm } from "node:fs/promises";
import { createRequire } from "node:module";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { finished } from "node:stream/promises";

async function freePort(): Promise<number> {
  const server = createServer();
  const listening = once(server, "listening");
  server.listen(0, "127.0.0.1");
  await listening;
  const address = server.address();
  if (!address || typeof address === "string") {
    throw new Error("expected a TCP listener address");
  }
  const port = address.port;
  const closed = once(server, "close");
  server.close();
  await closed;
  return port;
}

async function freeUDPPort(): Promise<number> {
  const socket = createSocket("udp4");
  const listening = once(socket, "listening");
  socket.bind(0, "127.0.0.1");
  await listening;
  const port = socket.address().port;
  const closed = once(socket, "close");
  socket.close();
  await closed;
  return port;
}

type Desktop = {
  nodeURL: string;
  page: Page;
  restart: () => Promise<Page>;
};

export const test = base.extend<{ desktop: Desktop }>({
  desktop: async ({}, use, testInfo) => {
    for (const name of [
      "E2E_UPSTREAM_STATION",
      "UPSTREAM_STATION_URL",
      "ATMOQ_RELAY_URL",
      "E2E_PLC_URL",
      "E2E_PROXY_URL",
      "E2E_TLS_CA",
      "E2E_TLS_SPKI",
      "PDS_HTTPS_URL",
      "ACCOUNT_HANDLE",
      "ACCOUNT_PASSWORD",
      "ACCOUNT_DID",
    ]) {
      if (!process.env[name]) {
        throw new Error(`${name} is not set — run hack/e2e-desktop-local.sh`);
      }
    }
    const desktopDir = resolve(__dirname, "../../desktop");
    const localHost = `127.0.0.1:${await freePort()}`;
    const nodeURL = `http://${localHost}`;
    const state = await mkdtemp(join(tmpdir(), "streamplace-desktop-e2e-"));
    const env = Object.fromEntries(
      Object.entries(process.env).filter(
        ([key, value]) =>
          !key.startsWith("SP_") &&
          !key.startsWith("AQD_") &&
          value !== undefined,
      ),
    ) as Record<string, string>;
    Object.assign(env, {
      XDG_CONFIG_HOME: join(state, "config"),
      XDG_DATA_HOME: join(state, "data"),
      WEBPACK_SERVE: "true",
      AQD_SKIP_NODE: "false",
      AQD_NODE_FRONTEND: "true",
      AQD_NO_UPDATE: "true",
      SP_UPSTREAM_STATION: `did:web:${new URL(process.env.UPSTREAM_STATION_URL!).hostname}`,
      SP_RELAY_HOST: process.env.ATMOQ_RELAY_URL!,
      SP_HTTP_ADDR: localHost,
      SP_HTTP_INTERNAL_ADDR: `127.0.0.1:${await freePort()}`,
      SP_RTMP_ADDR: `127.0.0.1:${await freePort()}`,
      SP_MOQ_ADDR: `127.0.0.1:${await freeUDPPort()}`,
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
      ...(process.platform === "linux" && process.getuid?.() === 0
        ? ["--no-sandbox"]
        : []),
      `--user-data-dir=${join(state, "electron")}`,
      `--proxy-server=${process.env.E2E_PROXY_URL}`,
      "--proxy-bypass-list=127.0.0.1;localhost",
      `--ignore-certificate-errors-spki-list=${process.env.E2E_TLS_SPKI}`,
      resolve(
        process.env.E2E_DESKTOP_MAIN ??
          join(desktopDir, ".webpack", process.arch, "main/index.cjs"),
      ),
    ];
    let app: ElectronApplication | undefined;
    let generation = 0;
    const logPath = testInfo.outputPath("desktop.log");
    const log = createWriteStream(logPath);
    const stop = async () => {
      if (!app) return;
      const current = app;
      app = undefined;
      try {
        if (testInfo.status !== testInfo.expectedStatus) {
          const page = current.windows()[0];
          if (page && !page.isClosed()) {
            const path = testInfo.outputPath(`desktop-${generation}.png`);
            await page.screenshot({ path });
            await testInfo.attach("screenshot", {
              path,
              contentType: "image/png",
            });
          }
        }
        const path = testInfo.outputPath(`trace-${generation}.zip`);
        await current.context().tracing.stop({ path });
        await testInfo.attach("trace", {
          path,
          contentType: "application/zip",
        });
      } finally {
        await current.close();
      }
    };
    const launch = async () => {
      app = await _electron.launch({
        executablePath: createRequire(join(desktopDir, "package.json"))(
          "electron",
        ) as string,
        args,
        cwd: desktopDir,
        env,
      });
      generation++;
      for (const stream of [app.process().stdout, app.process().stderr]) {
        stream?.on("data", (chunk: Buffer) => {
          process.stdout.write(chunk);
          log.write(chunk);
        });
      }
      await app.context().tracing.start({ screenshots: true, snapshots: true });
      const page = await app.firstWindow();
      await expect(page).toHaveURL(nodeURL + "/desktop", { timeout: 60_000 });
      return page;
    };
    try {
      const page = await launch();
      await use({
        nodeURL,
        page,
        restart: async () => {
          await stop();
          return launch();
        },
      });
    } finally {
      try {
        await stop();
      } finally {
        await rm(state, { recursive: true, force: true });
        await finished(log.end());
        await testInfo.attach("desktop log", {
          path: logPath,
          contentType: "text/plain",
        });
      }
    }
  },
});

export { expect } from "@playwright/test";
