// Platform-specific state: notification tokens, login-link handoff, etc.
// On web, push notifications are a no-op; the slice exists to mirror
// js/app/store/slices/platformSlice.ts (the web variant, not .native.ts).
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
}

export const createPlatformSlice: StateCreator<PlatformSlice> = (set) => ({
  status: "idle",
  notificationToken: null,
  notificationDestination: null,
  handleNotification: (payload) => {
    // web: no notification handler today. payload is preserved as a
    // destination for future deep-link routing.
    if (payload && typeof (payload as { path?: unknown }).path === "string") {
      set({ notificationDestination: (payload as { path: string }).path });
    }
  },
  clearNotification: () => {
    set({ notificationDestination: null });
  },
  openLoginLink: async (url: string) => {
    set({ status: "loading" });
    try {
      window.location.href = url;
      set({ status: "idle" });
    } catch {
      set({ status: "failed" });
    }
  },
  initPushNotifications: async () => {
    // mobile-only, web notifications someday
  },
  registerNotificationToken: async () => {
    // notification token registration (no-op on web)
  },
});
