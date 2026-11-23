import { AppStore } from "store";
import { StateCreator } from "zustand";

export interface PlatformSlice {
  status: "idle" | "loading" | "failed";
  notificationToken: string | null;
  notificationDestination: string | null;
  // actions
  handleNotification: (payload?: { [key: string]: string | object }) => void;
  clearNotification: () => void;
  openLoginLink: (url: string) => Promise<void>;
  initPushNotifications: () => Promise<void>;
  registerNotificationToken: () => Promise<void>;
  // web-only: subscribe/unsubscribe the browser's PushManager. Returns the
  // permission state so the settings toggle can reflect reality.
  enableWebNotifications: () => Promise<NotificationPermission>;
  disableWebNotifications: () => Promise<void>;
  webNotificationPermission: () => NotificationPermission;
}

// VAPID public key must be converted from base64url to a Uint8Array for the
// PushManager.subscribe() applicationServerKey argument.
function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = "=".repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/");
  const rawData = atob(base64);
  const output = new Uint8Array(rawData.length);
  for (let i = 0; i < rawData.length; ++i) {
    output[i] = rawData.charCodeAt(i);
  }
  return output;
}

async function postWebSubscription(
  url: string,
  token: string,
  oauthSession: AppStore["oauthSession"],
  mode: "signed-only" | "allow-anonymous",
) {
  // Relative OAuth requests are resolved against the token's audience.
  const session =
    oauthSession &&
    new URL((await oauthSession.getTokenInfo()).aud).origin ===
      new URL(url).origin
      ? oauthSession
      : null;
  if (!session && mode === "signed-only") return;

  const path = "/api/notification";
  const init: RequestInit = {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ token, type: "web", repoDID: session?.did }),
  };
  const res = session
    ? await session.fetchHandler(path, init)
    : await fetch(`${url}${path}`, init);
  if (!res.ok) throw new Error(`server registration failed: ${res.status}`);
}

export const createPlatformSlice: StateCreator<
  AppStore,
  [],
  [],
  PlatformSlice
> = (set, get) => ({
  status: "idle",
  notificationToken: null,
  notificationDestination: null,
  handleNotification: (payload) => {
    if (!payload) return;
    if (typeof payload.path !== "string") return;
    set({ notificationDestination: payload.path });
  },
  clearNotification: () => {
    set({ notificationDestination: null });
  },
  openLoginLink: async (url: string) => {
    set({ status: "loading" });
    try {
      window.location.href = url;
      set({ status: "idle" });
    } catch (error) {
      set({ status: "failed" });
    }
  },
  initPushNotifications: async () => {
    // Production registers before Expo loads; Metro still needs registration.
    // Push permission remains deferred to the settings toggle's user gesture.
    if (!("serviceWorker" in navigator)) {
      return;
    }
    try {
      if (!document.getElementById("bootstrap-scripts")) {
        await navigator.serviceWorker.register("/sw.js", {
          updateViaCache: "none",
        });
      }
      // Restore an existing PushSubscription into the store. The browser
      // keeps subscriptions across reloads (and the server still has the
      // row), but the Zustand store isn't persisted — so without this the
      // toggle would show "off" after a reload even though the user is
      // still subscribed. This mirrors the native path, where the FCM token
      // is re-acquired on startup.
      const reg = await navigator.serviceWorker.ready;
      const existing = await reg.pushManager.getSubscription();
      if (existing && !get().notificationToken) {
        set({ notificationToken: JSON.stringify(existing) });
        await get().registerNotificationToken();
      }
    } catch (e) {
      console.log("service worker registration failed", e);
    }
  },
  registerNotificationToken: async () => {
    const { url, notificationToken, oauthSession } = get();
    if (!url || !notificationToken || !oauthSession) return;
    try {
      await postWebSubscription(
        url,
        notificationToken,
        oauthSession,
        "signed-only",
      );
    } catch (error) {
      console.error("registerNotificationToken error", error);
    }
  },
  enableWebNotifications: async () => {
    const url = get().url;
    if (!url) {
      console.log(
        "no streamplace url configured, cannot enable web notifications",
      );
      return "denied";
    }
    try {
      const permission = await Notification.requestPermission();
      if (permission !== "granted") {
        return permission;
      }

      // Make sure the service worker is active before subscribing.
      const reg = await navigator.serviceWorker.ready;

      // Fetch the server's VAPID public key.
      const vapidRes = await fetch(`${url}/api/notification/vapid-public-key`);
      if (!vapidRes.ok) {
        throw new Error(`failed to fetch vapid public key: ${vapidRes.status}`);
      }
      const { publicKey } = await vapidRes.json();

      // Subscribe the browser's PushManager.
      const subscription = await reg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(publicKey) as BufferSource,
      });
      const token = JSON.stringify(subscription);
      await postWebSubscription(
        url,
        token,
        get().oauthSession,
        "allow-anonymous",
      );
      set({ notificationToken: token });
      return permission;
    } catch (e) {
      console.error("enableWebNotifications error", e);
      return "denied";
    }
  },
  disableWebNotifications: async () => {
    const url = get().url;
    const { notificationToken } = get();
    try {
      if (notificationToken) {
        // Unsubscribe the browser side so it stops accepting pushes.
        const sub = JSON.parse(notificationToken);
        // We need the live PushSubscription object to call unsubscribe(); get
        // it from the service worker registration by matching endpoint.
        const reg = await navigator.serviceWorker.ready;
        const existing = await reg.pushManager.getSubscription();
        if (existing && existing.endpoint === sub.endpoint) {
          await existing.unsubscribe();
        }
        // Tell the server to drop the row. Only clear the local token after
        // the DELETE succeeds — otherwise the toggle shows "off" while the
        // server keeps pushing to a subscription the user thought they
        // disabled, with no way to retry.
        if (url) {
          const res = await fetch(`${url}/api/notification`, {
            method: "DELETE",
            headers: { "content-type": "application/json" },
            body: JSON.stringify({ token: notificationToken }),
          });
          if (!res.ok) {
            throw new Error(`server delete failed: ${res.status}`);
          }
        }
        set({ notificationToken: null });
      }
    } catch (e) {
      console.error("disableWebNotifications error", e);
      // Leave notificationToken set so the toggle stays "on" and the user
      // can retry. The browser-side unsubscribe may have already succeeded,
      // but the server row is what matters for stopping future pushes.
    }
  },
  webNotificationPermission: () => {
    if (typeof Notification === "undefined") {
      return "denied";
    }
    return Notification.permission;
  },
});
