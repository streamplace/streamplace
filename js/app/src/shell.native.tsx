import { createBottomTabNavigator } from "@react-navigation/bottom-tabs";
import { useLinkTo, useNavigation } from "@react-navigation/native";
import { useToast } from "@streamplace/components";
import LoginModal from "components/login/login-modal";
import { useBlueskyNotifications } from "hooks/useBlueskyNotifications";
import { useLiveUser } from "hooks/useLiveUser";
import { useEffect, useState } from "react";
import { StatusBar } from "react-native";
import { useStore } from "store";
import {
  useHydrated,
  useNotificationDestination,
  useNotificationToken,
} from "store/hooks";
import { MainTab, SettingsStack } from "./navigator";
import MobileGoLive from "./screens/mobile-go-live";

const Tab = createBottomTabNavigator();

export default function NativeShell() {
  const hydrate = useStore((state) => state.hydrate);
  const initPushNotifications = useStore(
    (state) => state.initPushNotifications,
  );
  const registerNotificationToken = useStore(
    (state) => state.registerNotificationToken,
  );
  const clearNotification = useStore((state) => state.clearNotification);
  const pollMySegments = useStore((state) => state.pollMySegments);
  const showLoginModal = useStore((state) => state.showLoginModal);
  const closeLoginModal = useStore((state) => state.closeLoginModal);

  const toast = useToast();
  const navigation = useNavigation();

  // Top-level hydration and push notification registration
  useEffect(() => {
    hydrate();
    initPushNotifications();
  }, []);

  const notificationToken = useNotificationToken();
  const hydrated = useHydrated();

  useEffect(() => {
    if (notificationToken) {
      registerNotificationToken();
    }
  }, [notificationToken]);

  // Handle incoming push notification routing
  const notificationDestination = useNotificationDestination();
  const linkTo = useLinkTo();

  useEffect(() => {
    if (notificationDestination) {
      linkTo(notificationDestination);
      clearNotification();
    }
  }, [notificationDestination]);

  // Poll for live streamers
  useEffect(() => {
    let handle: NodeJS.Timeout;
    handle = setInterval(() => {
      pollMySegments();
    }, 2500);
    pollMySegments();
    return () => clearInterval(handle);
  }, []);

  const userIsLive = useLiveUser();
  useBlueskyNotifications();

  // Show "You are live!" toast
  const [isLiveDashboard, setIsLiveDashboard] = useState(false);
  useEffect(() => {
    if (!isLiveDashboard && userIsLive) {
      toast.show("You are live!", "Do you want to go to your Live Dashboard?", {
        actionLabel: "Go",
        onAction: () => {
          navigation.navigate("LiveDashboard" as never);
        },
        onClose: () => {},
        variant: "error",
        duration: 8,
      });
    }
  }, [userIsLive]);

  if (!hydrated) {
    return null;
  }

  return (
    <>
      <StatusBar barStyle="light-content" />
      <Tab.Navigator
        initialRouteName="HomeTab"
        backBehavior="initialRoute"
        screenOptions={{
          lazy: true,
        }}
      >
        <Tab.Screen
          name="HomeTab"
          component={MainTab}
          options={{
            // tabBarIcon: {
            //   type: "sfSymbol",
            //   name: "house",
            // },
            title: "Home",
          }}
        />
        <Tab.Screen
          name="MobileGoLive"
          component={MobileGoLive}
          options={{
            // tabBarIcon: {
            //   type: "sfSymbol",
            //   name: "dot.radiowaves.left.and.right",
            // },
            title: "Go Live",
          }}
        />
        <Tab.Screen
          name="SettingsTab"
          component={SettingsStack}
          options={{
            // tabBarIcon: {
            //   type: "sfSymbol",
            //   name: "gearshape",
            // },
            title: "Settings",
          }}
        />
      </Tab.Navigator>
      <LoginModal visible={showLoginModal} onClose={closeLoginModal} />
    </>
  );
}
