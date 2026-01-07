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

export const createPlatformSlice: StateCreator<PlatformSlice> = (set, get) => ({
  status: "idle",
  notificationToken: null,
  notificationDestination: null,
  handleNotification: (payload) => {
    // notification handling logic
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
    // mobile-only, web notifications someday
  },
  registerNotificationToken: async () => {
    // notification token registration
  },
});
