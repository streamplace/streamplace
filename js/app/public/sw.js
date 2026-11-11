// Streamplace service worker, served from /sw.js: Web Push notifications plus a
// cache-first store for hashed /_expo/static/ assets (production builds only).

// The web build sets a cache name; null (Metro dev) disables fetch interception.
const CACHE_NAME = null;
const MAX_CACHE_ENTRIES = 128;
const STATIC_PREFIX = "/_expo/static/";
const BYPASS_CACHE_MODES = ["no-store", "no-cache", "reload", "only-if-cached"];

self.addEventListener("message", (event) => {
  if (event.data && event.data.type === "SKIP_WAITING") {
    event.waitUntil(self.skipWaiting());
  }
});

// The cache is shared across deploys and never cleared, so old tabs keep assets.
self.addEventListener("activate", (event) => {
  event.waitUntil(self.clients.claim());
});

if (CACHE_NAME) {
  self.addEventListener("fetch", (event) => {
    if (isCacheableRequest(event.request)) {
      event.respondWith(cacheFirst(event));
    }
  });
}

function hasDirective(header, names) {
  return (header || "")
    .toLowerCase()
    .split(",")
    .some((d) => names.includes(d.trim().split("=")[0]));
}

function isCacheableRequest(request) {
  const url = new URL(request.url);
  return (
    request.method === "GET" &&
    request.mode !== "navigate" &&
    url.origin === self.location.origin &&
    url.pathname.startsWith(STATIC_PREFIX) &&
    !BYPASS_CACHE_MODES.includes(request.cache) &&
    !request.headers.has("range") &&
    !request.headers.has("authorization") &&
    !hasDirective(request.headers.get("cache-control"), [
      "no-store",
      "no-cache",
    ])
  );
}

function isImmutableResponse(response) {
  const cc = response.headers.get("cache-control");
  return (
    response.status === 200 &&
    response.type === "basic" &&
    !response.redirected &&
    hasDirective(cc, ["public"]) &&
    hasDirective(cc, ["immutable"]) &&
    !hasDirective(cc, ["no-store", "no-cache", "private"])
  );
}

async function cacheFirst(event) {
  const request = event.request;
  try {
    const cached = await (await caches.open(CACHE_NAME)).match(request);
    if (cached) {
      return cached;
    }
  } catch (e) {
    // Cache unavailable; fall through to the network.
  }
  const response = await fetch(request);
  if (isImmutableResponse(response)) {
    try {
      event.waitUntil(store(request, response.clone()).catch(() => {}));
    } catch (e) {
      // Never let caching fail a successful network response.
    }
  }
  return response;
}

async function store(request, response) {
  const cache = await caches.open(CACHE_NAME);
  await cache.put(request, response);
  // Best-effort FIFO bound: keys() is in insertion order.
  const keys = await cache.keys();
  const excess = Math.max(0, keys.length - MAX_CACHE_ENTRIES);
  for (const key of keys.slice(0, excess)) {
    await cache.delete(key);
  }
}

self.addEventListener("push", (event) => {
  let data = { title: "Streamplace", body: "", data: {} };
  try {
    if (event.data) {
      const parsed = event.data.json();
      data = {
        title: parsed.title || data.title,
        body: parsed.body || data.body,
        data: parsed.data || data.data,
      };
    }
  } catch (e) {
    // Payload wasn't JSON; fall back to raw text if present.
    if (event.data) {
      data.body = event.data.text();
    }
  }
  // Browsers require a visible notification for every push, even a silent one.
  event.waitUntil(
    self.registration.showNotification(data.title, {
      body: data.body,
      data: data.data,
      icon: "/notification-icon",
      badge: "/favicon.ico",
    }),
  );
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const path = event.notification.data && event.notification.data.path;
  const targetUrl = path ? path : "/";

  event.waitUntil(
    (async () => {
      const all = await self.clients.matchAll({
        type: "window",
        includeUncontrolled: true,
      });
      // Focus an existing tab if one is open.
      for (const client of all) {
        if ("focus" in client) {
          client.focus();
          if ("navigate" in client) {
            await client.navigate(targetUrl);
          }
          return;
        }
      }
      // Otherwise open a new one.
      if (self.clients.openWindow) {
        await self.clients.openWindow(targetUrl);
      }
    })(),
  );
});
