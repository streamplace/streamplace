import { afterEach, expect, it, vi } from "vitest";
import { createEdge } from "./handler";

const TEMPLATE =
  "<!doctype html><html><head><!-- sp-head --><title>Default</title><!-- /sp-head --></head><body></body></html>";

function assets(files: Record<string, string>) {
  return async (request: Request): Promise<Response> => {
    const path = new URL(request.url).pathname;
    return path in files
      ? new Response(files[path])
      : new Response("not found", { status: 404 });
  };
}

function urlOf(input: RequestInfo | URL): string {
  return input instanceof Request ? input.url : String(input);
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

it("serves the shell with the node's head", async () => {
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = new URL(urlOf(input));
    expect(url.pathname).toBe("/api/page-head");
    expect(url.searchParams.get("url")).toBe("https://edge.test/ada");
    return new Response(JSON.stringify({ head: "<title>Spliced</title>" }), {
      headers: { "content-type": "application/json" },
    });
  });
  vi.stubGlobal("fetch", fetchMock);

  const edge = createEdge({
    upstreams: ["https://node.test"],
    assets: assets({ "/index.html": TEMPLATE }),
  });
  const res = await edge(new Request("https://edge.test/ada"), null);

  expect(res.status).toBe(200);
  expect(res.headers.get("cache-control")).toBe("no-cache");
  const body = await res.text();
  expect(body).toContain("<title>Spliced</title>");
  expect(body).not.toContain("Default");
});

it("serves the template when no node answers", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => {
      throw new Error("down");
    }),
  );

  const edge = createEdge({
    upstreams: ["https://node.test"],
    assets: assets({ "/index.html": TEMPLATE }),
  });
  const res = await edge(new Request("https://edge.test/"), null);

  expect(res.status).toBe(200);
  expect(await res.text()).toContain("<title>Default</title>");
});

it("fails over on a GET and skips a node that just failed", async () => {
  // 0.999 makes shuffle a no-op, so the configured order is the tried order.
  vi.spyOn(Math, "random").mockReturnValue(0.999);
  let clock = 1_000;
  const calls: string[] = [];
  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const origin = new URL(urlOf(input)).origin;
    calls.push(origin);
    if (origin === "https://a.test") throw new Error("down");
    return new Response("ok");
  });
  vi.stubGlobal("fetch", fetchMock);

  const edge = createEdge({
    upstreams: ["https://a.test", "https://b.test"],
    assets: assets({}),
    now: () => clock,
  });
  const path = "https://edge.test/xrpc/place.stream.config.getEnv";

  expect((await edge(new Request(path), null)).status).toBe(200);
  expect(calls).toEqual(["https://a.test", "https://b.test"]);

  calls.length = 0;
  await edge(new Request(path), null);
  expect(calls).toEqual(["https://b.test"]);

  // Past the down window, the failed node is a candidate again.
  clock += 10_001;
  calls.length = 0;
  fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
    const origin = new URL(urlOf(input)).origin;
    calls.push(origin);
    if (origin === "https://b.test") throw new Error("down");
    return new Response("ok");
  });
  await edge(new Request(path), null);
  expect(calls.at(-1)).toBe("https://a.test");
});

it("does not retry a POST", async () => {
  const fetchMock = vi.fn(async () => {
    throw new Error("down");
  });
  vi.stubGlobal("fetch", fetchMock);

  const edge = createEdge({
    upstreams: ["https://a.test", "https://b.test"],
    assets: assets({}),
  });
  const res = await edge(
    new Request("https://edge.test/xrpc/place.stream.media.createUpload", {
      method: "POST",
      body: "{}",
    }),
    null,
  );

  expect(res.status).toBe(502);
  expect(fetchMock).toHaveBeenCalledTimes(1);
});

it("does not time out a streaming ingest request", async () => {
  vi.useFakeTimers();
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      const { promise, resolve, reject } = Promise.withResolvers<Response>();
      const timer = setTimeout(() => resolve(new Response("done")), 50);
      init?.signal?.addEventListener("abort", () => {
        clearTimeout(timer);
        reject(new Error("aborted"));
      });
      return promise;
    }),
  );
  const edge = createEdge({
    upstreams: ["https://node.test"],
    assets: assets({}),
    responseTimeoutMs: 5,
  });

  const ingest = edge(
    new Request("https://edge.test/api/ingest/fmp4/key", {
      method: "POST",
      body: "x",
    }),
    null,
  );
  await vi.advanceTimersByTimeAsync(50);
  expect((await ingest).status).toBe(200);

  // The same node, now slow on an ordinary request.
  const stuck = edge(
    new Request("https://edge.test/xrpc/place.stream.config.getEnv"),
    null,
  );
  await vi.advanceTimersByTimeAsync(10);
  expect((await stuck).status).toBe(502);
});

it("redirects playback to a node", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => new Response("ok")),
  );
  const edge = createEdge({
    upstreams: ["https://node.test"],
    assets: assets({}),
  });
  const res = await edge(
    new Request(
      "https://edge.test/xrpc/place.stream.playback.getLivePlaylist?streamer=a&rendition=source",
    ),
    null,
  );

  expect(res.status).toBe(307);
  expect(res.headers.get("location")).toBe(
    "https://node.test/xrpc/place.stream.playback.getLivePlaylist?streamer=a&rendition=source",
  );
  expect(res.headers.get("access-control-allow-origin")).toBe("*");
  expect(res.headers.get("cache-control")).toBe("no-store");
});

it("redirects only to a node that answers", async () => {
  vi.spyOn(Math, "random").mockReturnValue(0.999);
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL) => {
      if (new URL(urlOf(input)).origin === "https://stopped.test") {
        throw new Error("down");
      }
      return new Response("ok");
    }),
  );
  const edge = createEdge({
    upstreams: ["https://stopped.test", "https://live.test"],
    assets: assets({}),
  });
  const res = await edge(
    new Request(
      "https://edge.test/xrpc/place.stream.playback.getLivePlaylist?streamer=a",
    ),
    null,
  );

  expect(res.status).toBe(307);
  expect(res.headers.get("location")).toBe(
    "https://live.test/xrpc/place.stream.playback.getLivePlaylist?streamer=a",
  );
});

it("404s a missing asset instead of serving the shell", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => {
      throw new Error("no node");
    }),
  );
  const edge = createEdge({
    upstreams: ["https://node.test"],
    assets: assets({}),
  });
  const res = await edge(
    new Request("https://edge.test/assets/missing.js"),
    null,
  );

  expect(res.status).toBe(404);
});

it("serves a handle that looks like an asset as a page", async () => {
  const edge = createEdge({
    upstreams: [],
    assets: assets({ "/index.html": TEMPLATE }),
  });
  const res = await edge(new Request("https://edge.test/alice.zip"), null);

  expect(res.status).toBe(200);
  expect(await res.text()).toContain("<title>Default</title>");
});

it("overwrites client-supplied proxy headers", async () => {
  let seen: Headers | undefined;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL) => {
      if (!(input instanceof Request)) throw new Error("expected a Request");
      seen = input.headers;
      return new Response("ok");
    }),
  );

  const edge = createEdge({
    upstreams: ["https://node.test"],
    proxySecret: "s3cret",
    assets: assets({}),
  });
  const request = new Request(
    "https://edge.test/xrpc/place.stream.config.getEnv",
    {
      headers: {
        "x-forwarded-for": "1.2.3.4",
        "x-forwarded-host": "evil.test",
        "x-forwarded-proto": "http",
        "x-proxy-secret": "attacker",
      },
    },
  );
  await edge(request, "203.0.113.7");

  expect(seen?.get("x-forwarded-for")).toBe("203.0.113.7");
  expect(seen?.get("x-forwarded-host")).toBe("edge.test");
  expect(seen?.get("x-forwarded-proto")).toBe("https");
  expect(seen?.get("x-proxy-secret")).toBe("s3cret");
});
