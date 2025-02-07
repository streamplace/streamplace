import { openAuthSessionAsync } from "expo-web-browser";
import { createAppSlice } from "../../hooks/createSlice";
import { oauthCallback } from "features/bluesky/blueskySlice";
import messaging from "@react-native-firebase/messaging";
import { Platform, PermissionsAndroid } from "react-native";
import {
  initialState,
  RegisterNotificationTokenBody,
  PlatformState,
} from "./shared";
import { BlueskyState } from "features/bluesky/blueskyTypes";

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

export const platformSlice = createAppSlice({
  name: "platform",
  initialState,
  reducers: (create) => ({
    handleNotification: create.reducer(
      (
        state,
        action: { payload: { [key: string]: string | object } | undefined },
      ) => {
        if (!action.payload) {
          return state;
        }
        if (typeof action.payload.path !== "string") {
          return state;
        }
        return {
          ...state,
          notificationDestination: action.payload.path,
        };
      },
    ),
    clearNotification: create.reducer((state) => {
      return {
        ...state,
        notificationDestination: null,
      };
    }),
    openLoginLink: create.asyncThunk(
      async (url: string, thunkAPI) => {
        console.log("openLoginLink", url);
        const res = await openAuthSessionAsync(url);
        if (res.type === "success") {
          thunkAPI.dispatch(oauthCallback(res.url));
        }
      },
      {
        pending: (state) => {
          state.status = "loading";
        },
        fulfilled: (state) => {
          state.status = "idle";
        },
        rejected: (state, { error }) => {
          state.status = "failed";
          console.error(error);
        },
      },
    ),

    initPushNotifications: create.asyncThunk(
      async (_, thunkAPI) => {
        const msg = messaging();
        messaging().setBackgroundMessageHandler(async (remoteMessage) => {
          console.log("Message handled in the background!", remoteMessage);
        });
        await checkApplicationPermission();
        const authorizationStatus = await msg.requestPermission();

        let perms = "";

        if (authorizationStatus === messaging.AuthorizationStatus.AUTHORIZED) {
          console.log("User has notification permissions enabled.");
          perms += "authorized";
        } else if (
          authorizationStatus === messaging.AuthorizationStatus.PROVISIONAL
        ) {
          console.log("User has provisional notification permissions.");
          perms += "provisional";
        } else {
          console.log("User has notification permissions disabled");
          perms += "disabled";
        }

        const token = await msg.getToken();

        messaging()
          .subscribeToTopic("live")
          .then(() => console.log("Subscribed to live!"));

        messaging().onMessage((remoteMessage) => {
          console.log("Foreground message:", remoteMessage);
          // Display the notification to the user
        });
        messaging().onNotificationOpenedApp((remoteMessage) => {
          console.log(
            "App opened by notification while in foreground:",
            remoteMessage,
          );
          thunkAPI.dispatch(handleNotification(remoteMessage.data));
          // Handle notification interaction when the app is in the foreground
        });
        messaging()
          .getInitialNotification()
          .then((remoteMessage) => {
            if (!remoteMessage) {
              return;
            }
            console.log(
              "App opened by notification from closed state:",
              remoteMessage,
            );
            thunkAPI.dispatch(handleNotification(remoteMessage.data));
          });

        return { token };
      },
      {
        pending: (state) => {},
        fulfilled: (state, { payload }) => {
          return {
            ...state,
            notificationToken: payload.token,
          };
        },
        rejected: (state) => {},
      },
    ),

    registerNotificationToken: create.asyncThunk(
      async (_, thunkAPI) => {
        if (typeof process.env.EXPO_PUBLIC_STREAMPLACE_URL !== "string") {
          console.log("process.env.EXPO_PUBLIC_STREAMPLACE_URL undefined!");
          return;
        }
        const { platform, bluesky } = thunkAPI.getState() as {
          platform: PlatformState;
          bluesky: BlueskyState;
        };
        if (!platform.notificationToken) {
          throw new Error("No notification token");
        }
        const body: RegisterNotificationTokenBody = {
          token: platform.notificationToken,
        };
        const did = bluesky.oauthSession?.did;
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
          console.log({ status: res.status });
        } catch (e) {
          console.log(e);
        }
      },
      {
        pending: (state) => {},
        fulfilled: (state) => {},
        rejected: (state) => {},
      },
    ),
  }),

  selectors: {
    selectNotificationToken: (platform) => platform.notificationToken,
    selectNotificationDestination: (platform) =>
      platform.notificationDestination,
  },
});

export const {
  openLoginLink,
  initPushNotifications,
  registerNotificationToken,
  handleNotification,
  clearNotification,
} = platformSlice.actions;

export const { selectNotificationToken, selectNotificationDestination } =
  platformSlice.selectors;
