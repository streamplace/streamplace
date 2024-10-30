import messaging from "@react-native-firebase/messaging";
import { PermissionsAndroid, Platform } from "react-native";

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

export async function initPushNotifications() {
  try {
    const x = messaging();
    messaging().setBackgroundMessageHandler(async (remoteMessage) => {
      console.log("Message handled in the background!", remoteMessage);
    });
    await checkApplicationPermission();
    const authorizationStatus = await x.requestPermission();

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

    (async () => {
      if (typeof process.env.EXPO_PUBLIC_AQUAREUM_URL !== "string") {
        console.log("process.env.EXPO_PUBLIC_AQUAREUM_URL undefined!");
        return;
      }
      try {
        const token = await x.getToken();
        const res = await fetch(
          `${process.env.EXPO_PUBLIC_AQUAREUM_URL}/api/notification`,
          {
            method: "POST",
            headers: {
              "content-type": "application/json",
            },
            body: JSON.stringify({ token }),
          },
        );
        console.log({ status: res.status });
      } catch (e) {
        console.log(e);
      }
    })();
    // Register background handler

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
      // Handle notification interaction when the app is in the foreground
    });
    messaging()
      .getInitialNotification()
      .then((remoteMessage) => {
        console.log(
          "App opened by notification from closed state:",
          remoteMessage,
        );
        // Handle notification interaction when the app is opened from a closed state
      });
  } catch (e) {
    console.log(e);
  }
}
