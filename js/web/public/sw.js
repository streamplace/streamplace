// Streamplace service worker for Web Push.
//
// This file is served from /sw.js (see public/) and registered by the web
// platform slice on app mount. It does two things:
//
// 1. push — when a push message arrives, show it as a system notification.
//    The payload is the JSON-serialized NotificationBlast from the server
//    ({title, body, data}). If the payload is empty (a "silent" push), we
//    still show a notification with a generic title, since browsers require
//    a visible notification for every push.
//
// 2. notificationclick — focus an existing tab (or open a new one) and
//    navigate it to the path encoded in the notification's data, so tapping
//    a "🔴 @user is LIVE!" notification opens that stream.

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
  event.waitUntil(
    self.registration.showNotification(data.title, {
      body: data.body,
      data: data.data,
      icon: "/favicon.svg",
      badge: "/favicon.svg",
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
