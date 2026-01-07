import messaging from "@react-native-firebase/messaging";
import { openAuthSessionAsync } from "expo-web-browser";
import { PermissionsAndroid, Platform } from "react-native";
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
}

const checkApplicationPermission = async () => {
  if (Platform.OS === "android") {
    try {
      await PermissionsAndroid.request(
        PermissionsAndroid.PERMISSIONS.POST_NOTIFICATIONS,
      );
    } catch (error) {
      console.log("error getting notifications ", error);
    }
  }
};

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
      const res = await openAuthSessionAsync(url);
      set({ status: "idle" });
      if (res.type === "success") {
        // do oauth callback
        get().oauthCallback(res.url);
      }
    } catch (error) {
      set({ status: "failed" });
    }
  },
  initPushNotifications: async () => {
    try {
      const msg = messaging();

      // Handle background messages (when app is backgrounded)
      if (msg && typeof msg.setBackgroundMessageHandler === "function") {
        msg.setBackgroundMessageHandler(async (remoteMessage) => {
          console.log("Message handled in the background!", remoteMessage);
        });
      }

      // Request runtime permission on Android (POST_NOTIFICATIONS)
      await checkApplicationPermission();

      // Request notification permission from the OS
      const authorizationStatus = await msg.requestPermission();

      if (authorizationStatus === messaging.AuthorizationStatus.AUTHORIZED) {
        console.log("User has notification permissions enabled.");
      } else if (
        authorizationStatus === messaging.AuthorizationStatus.PROVISIONAL
      ) {
        console.log("User has provisional notification permissions.");
      } else {
        console.log("User has notification permissions disabled");
      }

      // Fetch device token
      const token = await msg.getToken();
      if (token) {
        set({ notificationToken: token });
        console.log("Notification token acquired:", token);
      } else {
        console.warn("Failed to acquire notification token");
      }

      // Subscribe to topic(s) if desired
      msg
        .subscribeToTopic("live")
        .then(() => console.log("Subscribed to live!"))
        .catch((e) => console.warn("Failed to subscribe to live topic", e));

      // Foreground message handler
      msg.onMessage((remoteMessage) => {
        console.log("Foreground message:", remoteMessage);
        // Could surface an in-app notification or toast here
      });

      // User tapped a notification while app was in background/foreground
      msg.onNotificationOpenedApp((remoteMessage) => {
        console.log(
          "App opened by notification while in background/foreground:",
          remoteMessage,
        );
        try {
          // route to the destination encoded in notification payload
          const data = remoteMessage?.data;
          if (data) {
            get().handleNotification(data as any);
          }
        } catch (e) {
          console.error("handleNotification error", e);
        }
      });

      // App opened from quit state by a notification
      try {
        const initial = await msg.getInitialNotification();
        if (initial) {
          console.log("App opened by notification from closed state:", initial);
          const data = initial?.data;
          if (data) {
            get().handleNotification(data as any);
          }
        }
      } catch (e) {
        console.warn("getInitialNotification error", e);
      }

      return;
    } catch (e) {
      console.error("initPushNotifications error", e);
    }
  },
  registerNotificationToken: async () => {
    try {
      // Ensure configured base URL is present
      if (typeof process.env.EXPO_PUBLIC_STREAMPLACE_URL !== "string") {
        console.log("process.env.EXPO_PUBLIC_STREAMPLACE_URL undefined!");
        return;
      }

      const { platform, bluesky } = get() as AppStore & {
        platform: { notificationToken: string | null };
        bluesky: { oauthSession?: { did?: string } };
      };

      const token = platform?.notificationToken;
      if (!token) {
        throw new Error("No notification token to register");
      }

      const body: any = {
        token,
      };

      const did = bluesky?.oauthSession?.did;
      if (did) {
        body.repoDID = did;
      }

      try {
        const res = await fetch(
          `${process.env.EXPO_PUBLIC_STREAMPLACE_URL}/api/notification`,
          {
            method: "POST",
            headers: {
              "content-type": "application/json",
            },
            body: JSON.stringify(body),
          },
        );
        console.log("registerNotificationToken status:", res.status);
      } catch (e) {
        console.log("Error registering notification token:", e);
      }
    } catch (e) {
      console.error("registerNotificationToken error", e);
    }
  },
});
