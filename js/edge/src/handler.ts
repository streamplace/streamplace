// Platform-neutral edge worker: serves the app shell from static assets and
// sends everything else to a healthy Streamplace node. The Cloudflare entry
// point is src/cloudflare.ts; nothing here uses platform-specific APIs.

// Keep in step with HeadStartMarker/HeadEndMarker in pkg/linking/linking.go.
export const HEAD_START = "<!-- sp-head -->";
export const HEAD_END = "<!-- /sp-head -->";

const PROXY_PREFIXES = ["/xrpc/", "/api/", "/oauth/", "/.well-known/", "/dl/"];
// Routes the node answers ahead of its static handler (see Handler in
// pkg/api/api.go), so the bundled copies must not win.
const PROXY_PATHS: Record<string, true> = {
  "/app-updates": true,
  "/linkbanner.png": true,
  "/favicon.ico": true,
  "/favicon.png": true,
  "/notification-icon": true,
};
// Playlists carry root-relative segment URLs, so one redirect keeps every
// later playlist poll and segment on that node.
const PLAYBACK_PATHS: Record<string, true> = {
  "/xrpc/place.stream.playback.getLivePlaylist": true,
  "/xrpc/place.stream.playback.getLiveSegment": true,
  "/xrpc/place.stream.playback.getVideoPlaylist": true,
  "/xrpc/place.stream.playback.getVideoBlob": true,
};
// Keep in step with assetExtensions in pkg/api/api.go.
const ASSET_EXTENSIONS: Record<string, true> = {
  ".mp4": true,
  ".m4s": true,
  ".m3u8": true,
  ".ts": true,
  ".webm": true,
  ".mp3": true,
  ".aac": true,
  ".ogg": true,
  ".json": true,
  ".js": true,
  ".mjs": true,
  ".css": true,
  ".map": true,
  ".wasm": true,
  ".png": true,
  ".jpg": true,
  ".jpeg": true,
  ".gif": true,
  ".webp": true,
  ".svg": true,
  ".ico": true,
  ".avif": true,
  ".woff": true,
  ".woff2": true,
  ".ttf": true,
  ".otf": true,
  ".txt": true,
  ".xml": true,
  ".pdf": true,
  ".zip": true,
};
const FORWARDED_HEADERS = [
  "x-forwarded-host",
  "x-forwarded-proto",
  "x-forwarded-for",
  "x-proxy-secret",
];
const RETRY_STATUSES = new Set([502, 503, 504]);
const DOWN_MS = 10_000;
const HEAD_TIMEOUT_MS = 1500;

export interface EdgeOptions {
  upstreams: string[];
  proxySecret?: string;
  // Fetches a static asset by request path; 404 when absent.
  assets: (request: Request) => Promise<Response>;
  now?: () => number;
  headTimeoutMs?: number;
}

interface Upstream {
  origin: string;
  downUntil: number;
}

export type EdgeHandler = (
  request: Request,
  clientIP?: string | null,
) => Promise<Response>;

export function createEdge(opts: EdgeOptions): EdgeHandler {
  const now = opts.now ?? Date.now;
  const headTimeoutMs = opts.headTimeoutMs ?? HEAD_TIMEOUT_MS;
  const upstreams: Upstream[] = opts.upstreams.map((origin) => ({
    origin: origin.replace(/\/+$/, ""),
    downUntil: 0,
  }));
  let template: string | null = null;

  function order(): Upstream[] {
    const healthy = shuffle(upstreams.filter((u) => u.downUntil <= now()));
    const down = shuffle(upstreams.filter((u) => u.downUntil > now()));
    return [...healthy, ...down];
  }

  function markDown(upstream: Upstream): void {
    upstream.downUntil = now() + DOWN_MS;
  }

  function upstreamRequest(
    upstream: Upstream,
    request: Request,
    url: URL,
    clientIP?: string | null,
  ): Request {
    const headers = new Headers(request.headers);
    for (const name of FORWARDED_HEADERS) headers.delete(name);
    headers.set("x-forwarded-host", url.host);
    headers.set("x-forwarded-proto", url.protocol.slice(0, -1));
    if (clientIP) headers.set("x-forwarded-for", clientIP);
    if (opts.proxySecret) headers.set("x-proxy-secret", opts.proxySecret);

    const init: RequestInit & { duplex?: "half" } = {
      method: request.method,
      headers,
      redirect: "manual",
    };
    if (request.method !== "GET" && request.method !== "HEAD") {
      init.body = request.body;
      // Node's fetch requires this for a streamed request body.
      init.duplex = "half";
    }
    return new Request(upstream.origin + url.pathname + url.search, init);
  }

  async function proxy(
    request: Request,
    url: URL,
    clientIP?: string | null,
  ): Promise<Response> {
    const retry = request.method === "GET" || request.method === "HEAD";
    let last: Response | null = null;
    for (const upstream of order()) {
      let res: Response;
      try {
        res = await fetch(upstreamRequest(upstream, request, url, clientIP));
      } catch {
        markDown(upstream);
        if (retry) continue;
        return new Response("Bad Gateway", { status: 502 });
      }
      if (RETRY_STATUSES.has(res.status)) {
        markDown(upstream);
        if (retry) {
          last = res;
          continue;
        }
      }
      return res;
    }
    return last ?? new Response("Bad Gateway", { status: 502 });
  }

  function redirectToNode(url: URL): Response {
    const upstream = order()[0];
    if (!upstream) {
      return new Response("Bad Gateway", { status: 502 });
    }
    return new Response(null, {
      status: 307,
      headers: {
        location: new URL(
          url.pathname + url.search,
          upstream.origin,
        ).toString(),
        "access-control-allow-origin": "*",
        "cache-control": "no-store",
      },
    });
  }

  async function loadTemplate(): Promise<string | null> {
    if (template !== null) return template;
    const res = await opts
      .assets(new Request("https://edge.internal/index.html"))
      .catch(() => null);
    if (!res?.ok) return null;
    template = await res.text();
    return template;
  }

  async function fetchHead(url: URL): Promise<string | null> {
    const path = "/api/page-head?url=" + encodeURIComponent(url.toString());
    for (const upstream of order()) {
      const controller = new AbortController();
      const timer = setTimeout(() => controller.abort(), headTimeoutMs);
      try {
        const res = await fetch(upstream.origin + path, {
          signal: controller.signal,
        });
        if (res.status !== 200) continue;
        return headOf(await res.json());
      } catch {
        markDown(upstream);
      } finally {
        clearTimeout(timer);
      }
    }
    return null;
  }

  async function serveShell(url: URL): Promise<Response> {
    const html = await loadTemplate();
    if (html === null) {
      return new Response("frontend unavailable", { status: 500 });
    }
    const head = await fetchHead(url);
    return new Response(head ? spliceHead(html, head) : html, {
      headers: {
        "content-type": "text/html; charset=utf-8",
        "cache-control": "no-cache",
      },
    });
  }

  async function serveAsset(
    request: Request,
    url: URL,
  ): Promise<Response | null> {
    if (url.pathname === "/" || url.pathname === "/index.html") return null;
    const res = await opts.assets(request).catch(() => null);
    if (res && res.status !== 404) return withCachePolicy(res, url.pathname);
    // Astro writes directory indexes; mirror the node's FileServer, which
    // redirects a directory without a trailing slash and serves its index.
    if (url.pathname.startsWith("/docs")) {
      const dir = url.pathname.endsWith("/")
        ? url.pathname
        : url.pathname + "/";
      const index = await opts
        .assets(new Request(url.origin + dir + "index.html"))
        .catch(() => null);
      if (index && index.status !== 404) {
        return dir === url.pathname
          ? withCachePolicy(index, dir + "index.html")
          : Response.redirect(url.origin + dir, 301);
      }
      return null;
    }
    return null;
  }

  return async function handle(request, clientIP) {
    const url = new URL(request.url);
    const readOnly = request.method === "GET" || request.method === "HEAD";

    if (readOnly && Object.hasOwn(PLAYBACK_PATHS, url.pathname)) {
      return redirectToNode(url);
    }
    if (!readOnly || isProxiedPath(url.pathname)) {
      return proxy(request, url, clientIP);
    }

    const asset = await serveAsset(request, url);
    if (asset) return asset;

    const hashed = url.pathname.startsWith("/_expo/static/");
    if (hashed || looksLikeAsset(url.pathname)) {
      // Missing assets must not become cacheable app HTML.
      return new Response("Not Found", {
        status: 404,
        headers: hashed ? { "cache-control": "no-store" } : {},
      });
    }
    return serveShell(url);
  };
}

// Keep in step with spliceHead in pkg/linking/linking.go.
export function spliceHead(template: string, head: string): string {
  const start = template.indexOf(HEAD_START);
  const end = template.indexOf(HEAD_END);
  if (start < 0 || end < start) return template;
  return (
    template.slice(0, start) + head + template.slice(end + HEAD_END.length)
  );
}

// Keep in step with looksLikeAsset in pkg/api/api.go.
export function looksLikeAsset(pathname: string): boolean {
  const dot = pathname.lastIndexOf(".");
  if (dot < 0) return false;
  return Object.hasOwn(ASSET_EXTENSIONS, pathname.slice(dot).toLowerCase());
}

function isProxiedPath(pathname: string): boolean {
  return (
    PROXY_PREFIXES.some((prefix) => pathname.startsWith(prefix)) ||
    Object.hasOwn(PROXY_PATHS, pathname)
  );
}

// Keep in step with notFoundLinkingHandler in pkg/api/api.go.
function withCachePolicy(res: Response, pathname: string): Response {
  const headers = new Headers(res.headers);
  if (pathname.startsWith("/_expo/static/")) {
    headers.set("cache-control", "public, max-age=31536000, immutable");
  } else if (!looksLikeAsset(pathname) || pathname === "/sw.js") {
    headers.set("cache-control", "no-cache");
  }
  return new Response(res.body, {
    status: res.status,
    statusText: res.statusText,
    headers,
  });
}

function headOf(body: unknown): string | null {
  if (body && typeof body === "object" && "head" in body) {
    return typeof body.head === "string" ? body.head : null;
  }
  return null;
}

function shuffle<T>(items: T[]): T[] {
  const result = [...items];
  for (let i = result.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [result[i], result[j]] = [result[j], result[i]];
  }
  return result;
}
