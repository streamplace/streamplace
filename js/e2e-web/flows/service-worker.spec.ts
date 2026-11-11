import {
  test as base,
  chromium,
  expect,
  type BrowserContext,
  type Page,
} from "@playwright/test";
import { readFileSync } from "node:fs";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import {
  createServer,
  type IncomingMessage,
  type Server,
  type ServerResponse,
} from "node:http";
import { tmpdir } from "node:os";
import path from "node:path";
import { Readable } from "node:stream";
import { STORAGE_STATE } from "../storage";

// Web-only. A real local server, not page.route, since a browser's service
// worker update fetch cannot be intercepted; it serves dist with a changed /sw.js.
const DIST = path.resolve(__dirname, "../../app/dist");
const SETTINGS = "/settings";
const STARTUP_WINDOW_MS = 3_000;

const PROBE_CACHE_CONTROL: Record<string, string> = {
  immutable: "public, max-age=31536000, immutable",
  "no-store": "public, max-age=31536000, immutable, no-store",
  private: "private, max-age=31536000, immutable",
  revalidated: "public, max-age=31536000",
};
const MIME: Record<string, string> = {
  ".css": "text/css",
  ".html": "text/html; charset=utf-8",
  ".ico": "image/x-icon",
  ".js": "text/javascript",
  ".json": "application/json",
  ".png": "image/png",
  ".svg": "image/svg+xml",
  ".ttf": "font/ttf",
  ".wasm": "application/wasm",
  ".woff": "font/woff",
  ".woff2": "font/woff2",
};

type WorkerMode = "ok" | "unavailable" | "broken";

class Deployment {
  deployment = "v1";
  worker: WorkerMode = "ok";
  // Resets every connection, like a dropped network: only a cache can answer.
  offline = false;
  readonly requests: { path: string; document: boolean; offline: boolean }[] =
    [];

  private constructor(
    private readonly server: Server,
    readonly origin: string,
  ) {}

  static async start() {
    let deployment!: Deployment;
    const server = createServer((req, res) => {
      deployment.handle(req, res).catch(() => res.destroy());
    });
    await new Promise<void>((resolve) =>
      server.listen(0, "127.0.0.1", resolve),
    );
    const { port } = server.address() as { port: number };
    deployment = new Deployment(server, `http://127.0.0.1:${port}`);
    return deployment;
  }

  documents(pathname: string) {
    return this.requests.filter(
      (r) => r.document && !r.offline && r.path === pathname,
    ).length;
  }

  workerRequests() {
    return this.requests.filter((r) => r.path === "/sw.js").length;
  }

  async close() {
    this.server.closeAllConnections();
    await new Promise((resolve) => this.server.close(resolve));
  }

  private async handle(req: IncomingMessage, res: ServerResponse) {
    const { headers } = req;
    const url = new URL(req.url ?? "/", this.origin);
    this.requests.push({
      path: url.pathname + url.search,
      document: headers["sec-fetch-dest"] === "document",
      offline: this.offline,
    });
    if (this.offline) return void req.socket.destroy();
    const send = (
      status: number,
      head: Record<string, string>,
      body: Buffer | string,
    ) => {
      res.writeHead(status, head);
      res.end(body);
    };

    if (url.pathname === "/sw.js") {
      if (this.worker === "unavailable") {
        return send(503, { "Cache-Control": "no-store" }, "unavailable");
      }
      const marker = `\nself.__deployment = ${JSON.stringify(this.deployment)};\n`;
      const broken =
        this.worker === "broken" ? 'throw new Error("broken");\n' : "";
      return send(
        200,
        { "Content-Type": "text/javascript", "Cache-Control": "no-cache" },
        (await readFile(path.join(DIST, "sw.js"), "utf8")) + marker + broken,
      );
    }

    // Synthetic assets whose headers and status the test chooses.
    if (/^\/((_expo\/static|api)\/)?probe\//.test(url.pathname)) {
      const body = Buffer.from("x".repeat(64));
      const range = /^bytes=(\d+)-(\d+)$/.exec(headers.range ?? "");
      const head: Record<string, string> = {
        "Content-Type": "text/javascript",
        "Cache-Control":
          PROBE_CACHE_CONTROL[url.searchParams.get("cache") ?? "immutable"],
      };
      if (range) {
        const [start, end] = [Number(range[1]), Number(range[2])];
        head["Content-Range"] = `bytes ${start}-${end}/${body.length}`;
        return send(206, head, body.subarray(start, end + 1));
      }
      return send(Number(url.searchParams.get("status") ?? 200), head, body);
    }

    const rel = decodeURIComponent(url.pathname).replace(/^\/+/, "");
    const file = path.resolve(DIST, rel);
    const hashed = rel.startsWith("_expo/static/");
    if (rel && file.startsWith(DIST + path.sep)) {
      try {
        const body = await readFile(file);
        return send(
          200,
          {
            "Content-Type":
              MIME[path.extname(file).toLowerCase()] ??
              "application/octet-stream",
            ...(hashed
              ? { "Cache-Control": "public, max-age=31536000, immutable" }
              : {}),
          },
          body,
        );
      } catch {
        // Not a file: an app route or a missing asset.
      }
    }
    if (hashed || path.extname(rel)) {
      return send(404, { "Cache-Control": "no-store" }, "file not found");
    }
    const wantsHTML =
      headers["sec-fetch-dest"] === "document" ||
      !!headers.accept?.includes("text/html");
    if (!wantsHTML) return this.proxy(req, url, res);
    return send(
      200,
      { "Content-Type": MIME[".html"], "Cache-Control": "no-cache" },
      await readFile(path.join(DIST, "index.html")),
    );
  }

  // Same-origin API calls the app makes reach the harness node, as they would
  // on a real node serving its own app.
  private async proxy(req: IncomingMessage, url: URL, res: ServerResponse) {
    const headers: Record<string, string> = {};
    for (const name of ["accept", "authorization", "content-type", "range"]) {
      const value = req.headers[name];
      if (typeof value === "string") headers[name] = value;
    }
    const init: RequestInit & { duplex?: "half" } = {
      method: req.method,
      headers,
    };
    if (req.method !== "GET" && req.method !== "HEAD") {
      init.body = Readable.toWeb(req) as unknown as BodyInit;
      init.duplex = "half";
    }
    const upstream = await fetch(
      process.env.SERVER_URL! + url.pathname + url.search,
      init,
    );
    const head = Object.fromEntries(upstream.headers);
    delete head["content-encoding"];
    delete head["content-length"];
    res.writeHead(upstream.status, head);
    if (!upstream.body) return void res.end();
    Readable.fromWeb(
      upstream.body as Parameters<typeof Readable.fromWeb>[0],
    ).pipe(res);
  }
}

// The harness node's saved Settings -> Advanced choice (see global-setup),
// replayed into each deployment's own origin.
function nodeSettings() {
  const state = JSON.parse(readFileSync(STORAGE_STATE, "utf8")) as {
    origins: {
      origin: string;
      localStorage: { name: string; value: string }[];
    }[];
  };
  const origin = new URL(process.env.SERVER_URL!).origin;
  const entries = state.origins.find((o) => o.origin === origin)?.localStorage;
  if (!entries?.length) throw new Error(`no saved node settings for ${origin}`);
  return entries;
}

type Lab = {
  serve(): Promise<Deployment>;
  // One persistent browser profile per test, reused by every launch.
  launch(deployment: Deployment): Promise<BrowserContext>;
};

const test = base.extend<{ lab: Lab }>({
  lab: async ({}, use) => {
    const servers: Deployment[] = [];
    const contexts: BrowserContext[] = [];
    const profile = await mkdtemp(path.join(tmpdir(), "sp-service-worker-"));
    try {
      await use({
        async serve() {
          const deployment = await Deployment.start();
          servers.push(deployment);
          return deployment;
        },
        async launch(deployment) {
          const context = await chromium.launchPersistentContext(profile, {
            headless: true,
          });
          contexts.push(context);
          await context.addInitScript(
            ({ origin, entries }) => {
              if (location.origin !== origin) return;
              for (const { name, value } of entries) {
                if (localStorage.getItem(name) === null) {
                  localStorage.setItem(name, value);
                }
              }
            },
            { origin: deployment.origin, entries: nodeSettings() },
          );
          return context;
        },
      });
    } finally {
      await Promise.all(contexts.map((c) => c.close().catch(() => {})));
      await Promise.all(servers.map((s) => s.close()));
      await rm(profile, { recursive: true, force: true });
    }
  },
});

async function builtAsset() {
  const html = await readFile(path.join(DIST, "index.html"), "utf8");
  const manifest =
    /<script id="bootstrap-scripts" type="application\/json">(.*?)<\/script>/.exec(
      html,
    );
  if (!manifest) throw new Error("dist has no bootstrap manifest: make app");
  return new URL((JSON.parse(manifest[1]) as string[])[0], "http://x").pathname;
}

const searchBox = (page: Page) =>
  page.getByRole("combobox", { name: "Search", exact: true });

// "commit": a deployment reload may replace the document before it loads.
async function openSettings(context: BrowserContext, deployment: Deployment) {
  const page = await context.newPage();
  await page.goto(deployment.origin + SETTINGS, { waitUntil: "commit" });
  await expect(searchBox(page)).toBeVisible();
  return page;
}

const controlled = (page: Page) =>
  page.waitForFunction(() => navigator.serviceWorker.controller !== null);
const olderThanStartupWindow = (page: Page) =>
  page.waitForFunction((ms) => performance.now() > ms, STARTUP_WINDOW_MS + 500);
// A reload would replace the document and lose this.
const mark = (page: Page) =>
  page.evaluate(() => {
    Object.assign(window, { __marked: true });
  });
const marked = (page: Page) => page.evaluate(() => "__marked" in window);
const navigationType = (page: Page) =>
  page.evaluate(
    () =>
      (
        performance.getEntriesByType(
          "navigation",
        )[0] as PerformanceNavigationTiming
      ).type,
  );
const registrationState = (page: Page) =>
  page.evaluate(async () => {
    const registration = await navigator.serviceWorker.getRegistration();
    return {
      installing: !!registration?.installing,
      waiting: !!registration?.waiting,
    };
  });
const checkForUpdate = (page: Page) =>
  page.evaluate(async () => {
    await (await navigator.serviceWorker.getRegistration())!.update();
  });

// Deployment marker -> lifecycle state of every worker the browser knows.
async function workers(context: BrowserContext) {
  const states: Record<string, string> = {};
  for (const worker of context.serviceWorkers()) {
    try {
      const { deployment, state } = await worker.evaluate(() => {
        const scope = self as unknown as {
          __deployment: string;
          serviceWorker: { state: string };
        };
        return {
          deployment: scope.__deployment,
          state: scope.serviceWorker.state,
        };
      });
      states[deployment] = state;
    } catch {
      // A worker that just terminated is not a candidate.
    }
  }
  return states;
}

type FetchOptions = Pick<RequestInit, "cache" | "mode"> & {
  headers?: Record<string, string>;
};

// "Fetched from the network or not": a status, or a network error.
const outcome = (page: Page, url: string, init: FetchOptions = {}) =>
  page.evaluate(
    async ([url, init]) => {
      try {
        const response = await fetch(url, init);
        await response.arrayBuffer();
        return response.status;
      } catch {
        return "network error";
      }
    },
    [url, init] as [string, FetchOptions],
  );

// Leave only Cache Storage able to answer: no HTTP cache, no network.
async function goOffline(
  context: BrowserContext,
  page: Page,
  ...deployments: Deployment[]
) {
  const cdp = await context.newCDPSession(page);
  await cdp.send("Network.clearBrowserCache");
  await cdp.detach();
  for (const deployment of deployments) deployment.offline = true;
}

const cacheEntries = (page: Page) =>
  page.evaluate(async () => {
    let entries = 0;
    for (const name of await caches.keys()) {
      entries += (await (await caches.open(name)).keys()).length;
    }
    return entries;
  });
const isCached = (page: Page, url: string) =>
  page.evaluate(async (url) => !!(await caches.match(url)), url);

test("service worker: first install takes control without reloading", async ({
  lab,
}) => {
  const deployment = await lab.serve();
  const context = await lab.launch(deployment);
  const page = await openSettings(context, deployment);
  await mark(page);
  await controlled(page);

  await expect.poll(() => workers(context)).toEqual({ v1: "activated" });
  // Past the startup window, a late reload would no longer be possible.
  await olderThanStartupWindow(page);
  expect(await marked(page)).toBe(true);
  expect(await navigationType(page)).toBe("navigate");
  expect(deployment.documents(SETTINGS)).toBe(1);
});

test("service worker: caches only public immutable same-origin assets", async ({
  lab,
}) => {
  const deployment = await lab.serve();
  const other = await lab.serve();
  const context = await lab.launch(deployment);
  const asset = await builtAsset();
  const page = await openSettings(context, deployment);
  await controlled(page);

  const probe = (name: string, query = "") =>
    `/_expo/static/probe/${name}.js${query}`;
  // Each is fetched once with its own URL; none may end up in Cache Storage.
  const uncached: [string, string, FetchOptions?][] = [
    ["no-store response", probe("a", "?cache=no-store")],
    ["private response", probe("b", "?cache=private")],
    ["response without immutable", probe("c", "?cache=revalidated")],
    ["server error", probe("d", "?status=500")],
    ["range request", probe("e"), { headers: { Range: "bytes=0-3" } }],
    [
      "authenticated request",
      probe("f"),
      { headers: { Authorization: "Bearer x" } },
    ],
    ["cache: no-store", probe("g"), { cache: "no-store" }],
    ["cache: no-cache", probe("h"), { cache: "no-cache" }],
    ["cache: reload", probe("i"), { cache: "reload" }],
    [
      "request Cache-Control: no-store",
      probe("j"),
      { headers: { "Cache-Control": "no-store" } },
    ],
    ["API path with immutable headers", "/api/probe/k.js"],
    ["missing hashed asset", "/_expo/static/js/web/missing-0000.js"],
    ["HTML document", "/policy-html", { headers: { Accept: "text/html" } }],
  ];
  const crossOrigin = other.origin + probe("l");
  await outcome(page, asset);
  for (const [, url, init] of uncached) await outcome(page, url, init);
  await outcome(page, crossOrigin, { mode: "no-cors" });
  await expect.poll(() => isCached(page, asset)).toBe(true);

  await goOffline(context, page, deployment, other);
  expect(await outcome(page, asset)).toBe(200);
  for (const [name, url] of uncached) {
    expect(await outcome(page, url), name).toBe("network error");
  }
  expect(await outcome(page, crossOrigin, { mode: "no-cors" })).toBe(
    "network error",
  );

  // FIFO-bounded at 128 entries, whatever the page requests.
  deployment.offline = false;
  for (let batch = 0; batch < 14; batch++) {
    await Promise.all(
      Array.from({ length: 10 }, (_, i) =>
        outcome(page, probe(`bounded-${batch * 10 + i}`)),
      ),
    );
  }
  await expect.poll(() => cacheEntries(page)).toBe(128);
});

test("service worker: cached asset survives a browser restart", async ({
  lab,
}) => {
  const deployment = await lab.serve();
  const asset = await builtAsset();
  let context = await lab.launch(deployment);
  let page = await openSettings(context, deployment);
  await controlled(page);
  await outcome(page, asset);
  await expect.poll(() => isCached(page, asset)).toBe(true);
  await context.close();

  context = await lab.launch(deployment);
  page = await openSettings(context, deployment);
  // The persisted worker controls the first navigation; same bytes, no reload.
  expect(
    await page.evaluate(() => navigator.serviceWorker.controller !== null),
  ).toBe(true);
  expect(await navigationType(page)).toBe("navigate");
  expect(deployment.documents(SETTINGS)).toBe(2);

  await goOffline(context, page, deployment);
  expect(await outcome(page, asset)).toBe(200);
  expect(await outcome(page, asset, { cache: "reload" })).toBe("network error");
  expect(await outcome(page, asset, { headers: { Range: "bytes=0-3" } })).toBe(
    "network error",
  );
  expect(await outcome(page, "/_expo/static/probe/unseen.js")).toBe(
    "network error",
  );
  expect(await outcome(page, "/policy-html")).toBe("network error");
});

test("service worker: an update inside the startup window reloads only the new tab", async ({
  lab,
}) => {
  const deployment = await lab.serve();
  const asset = await builtAsset();
  const context = await lab.launch(deployment);
  const established = await openSettings(context, deployment);
  await controlled(established);
  await outcome(established, asset);
  await expect.poll(() => isCached(established, asset)).toBe(true);
  await searchBox(established).fill("keep my draft");
  await mark(established);
  await olderThanStartupWindow(established);

  deployment.deployment = "v2";
  const before = deployment.documents(SETTINGS);
  const fresh = await openSettings(context, deployment);

  await expect.poll(() => workers(context)).toMatchObject({ v2: "activated" });
  await expect
    .poll(() => navigationType(fresh).catch(() => "navigating"))
    .toBe("reload");
  await expect(searchBox(fresh)).toBeVisible();
  expect(fresh.url()).toBe(deployment.origin + SETTINGS);
  // Past the startup window, a repeat reload would no longer be possible.
  await olderThanStartupWindow(fresh);
  expect(deployment.documents(SETTINGS)).toBe(before + 2);

  // The established tab was claimed by v2 but keeps its page and its assets.
  expect(await marked(established)).toBe(true);
  await expect(searchBox(established)).toHaveValue("keep my draft");
  await goOffline(context, established, deployment);
  expect(await outcome(established, asset)).toBe(200);
});

test("service worker: a late update waits and activates on the next navigation", async ({
  lab,
}) => {
  const deployment = await lab.serve();
  const context = await lab.launch(deployment);
  const page = await openSettings(context, deployment);
  await controlled(page);
  await searchBox(page).fill("keep my draft");
  await mark(page);
  await olderThanStartupWindow(page);

  deployment.deployment = "v2";
  await checkForUpdate(page);
  await expect
    .poll(() => registrationState(page))
    .toMatchObject({ waiting: true });
  await page.waitForTimeout(1_000);
  expect(await marked(page)).toBe(true);
  await expect(searchBox(page)).toHaveValue("keep my draft");
  await expect
    .poll(() => workers(context))
    .toMatchObject({ v1: "activated", v2: "installed" });

  const before = deployment.documents(SETTINGS);
  await page.reload({ waitUntil: "commit" });
  await expect.poll(() => workers(context)).toMatchObject({ v2: "activated" });
  await expect(searchBox(page)).toBeVisible();
  // The explicit reload, then the one deployment reload that activates v2.
  await expect.poll(() => deployment.documents(SETTINGS)).toBe(before + 2);
  await page.waitForTimeout(1_000);
  expect(deployment.documents(SETTINGS)).toBe(before + 2);
});

test("service worker: failed updates keep the working version", async ({
  lab,
}) => {
  const deployment = await lab.serve();
  const asset = await builtAsset();
  const context = await lab.launch(deployment);
  const page = await openSettings(context, deployment);
  await controlled(page);
  await outcome(page, asset);
  await expect.poll(() => isCached(page, asset)).toBe(true);

  deployment.deployment = "v2";
  for (const mode of ["unavailable", "broken"] as const) {
    deployment.worker = mode;
    const documents = deployment.documents(SETTINGS);
    const checks = deployment.workerRequests();
    await page.reload({ waitUntil: "commit" });
    await expect(searchBox(page)).toBeVisible();
    await expect
      .poll(() => deployment.workerRequests())
      .toBeGreaterThan(checks);
    await expect
      .poll(() => registrationState(page).catch(() => undefined))
      .toEqual({ installing: false, waiting: false });
    await page.waitForTimeout(500);
    expect(deployment.documents(SETTINGS), mode).toBe(documents + 1);
    const states = await workers(context);
    expect(states.v1, mode).toBe("activated");
    expect(states.v2, mode).not.toBe("activated");
  }

  await goOffline(context, page, deployment);
  expect(await outcome(page, asset)).toBe(200);
  deployment.offline = false;

  // The next good deployment still installs.
  deployment.worker = "ok";
  await page.reload({ waitUntil: "commit" });
  await expect.poll(() => workers(context)).toMatchObject({ v2: "activated" });
});
