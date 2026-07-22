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
    // Register the service worker that receives push events. This must
    // happen early (on app mount) so that pushes delivered while the tab is
    // backgrounded still surface as system notifications. The actual
    // subscription + permission request is deferred to the settings toggle
    // (enableWebNotifications) because browsers require a user gesture for
    // the permission prompt.
    if (!("serviceWorker" in navigator)) {
      return;
    }
    try {
      await navigator.serviceWorker.register("/sw.js");
    } catch (e) {
      console.log("service worker registration failed", e);
    }
  },
  registerNotificationToken: async () => {
    // On web, token registration is driven by enableWebNotifications (which
    // subscribes and posts the subscription). This no-op keeps the shared
    // shell effect happy without double-registering.
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
      const subJSON = JSON.stringify(subscription);
      set({ notificationToken: subJSON });

      // Register the subscription with the backend.
      const { oauthSession } = get();
      const body: { token: string; type: string; repoDID?: string } = {
        token: subJSON,
        type: "web",
      };
      if (oauthSession?.did) {
        body.repoDID = oauthSession.did;
      }
      const res = await fetch(`${url}/api/notification`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(body),
      });
      console.log("web notification registration status:", res.status);
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
