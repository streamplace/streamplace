import {
  createDrawerNavigator,
  DrawerContentScrollView,
  DrawerItem,
  DrawerItemList,
} from "@react-navigation/drawer";
import {
  CommonActions,
  useLinkTo,
  useNavigation,
} from "@react-navigation/native";
import { Text, useTheme, useToast, useUrl } from "@streamplace/components";
import LoginModal from "components/login/login-modal";
import Sidebar, { ExternalDrawerItem } from "components/sidebar/sidebar";
import { useBlueskyNotifications } from "hooks/useBlueskyNotifications";
import { useLiveUser } from "hooks/useLiveUser";
import usePlatform from "hooks/usePlatform";
import { useSidebarControl } from "hooks/useSidebarControl";
import {
  Book,
  Download,
  ExternalLink,
  Home,
  LogIn,
  Settings as SettingsIcon,
  ShieldQuestion,
  Video,
} from "lucide-react-native";
import React, { Fragment, useEffect, useState } from "react";
import { Linking, StatusBar, View } from "react-native";
import { runOnJS, useAnimatedReaction } from "react-native-reanimated";
import "src/navigation-types";
import { RootNavigator } from "src/root-navigator";
import { useStore } from "store";
import {
  useHydrated,
  useNotificationDestination,
  useNotificationToken,
} from "store/hooks";
import { AvatarButton, NavigationButton } from "./router";

const Drawer = createDrawerNavigator();

const useExternalItems = (): ExternalDrawerItem[] => {
  const streamplaceUrl = useUrl();
  const { theme } = useTheme();
  return [
    {
      item: React.memo(() => <Book size={24} color={theme.colors.text} />),
      label: (
        <Text variant="h5" style={{ alignSelf: "flex-start" }}>
          Documentation{" "}
          <ExternalLink
            size={16}
            color={theme.colors.mutedForeground}
            style={{
              position: "relative",
              top: 2,
            }}
          />
        </Text>
      ) as any,
      onPress: () => {
        const u = new URL(streamplaceUrl);
        u.pathname = "/docs";
        Linking.openURL(u.toString());
      },
    },
  ];
};

function CustomDrawerContent(props) {
  let { theme } = useTheme();
  return (
    <DrawerContentScrollView {...props}>
      <DrawerItemList {...props} />
      <DrawerItem
        icon={() => <Book size={24} color={theme.colors.text} />}
        label={() => (
          <Text style={{ alignSelf: "flex-start" }}>
            Documentation{" "}
            <ExternalLink
              size={16}
              color="#666"
              style={{
                position: "relative",
                top: 2,
              }}
            />
          </Text>
        )}
        onPress={() => {
          const u = new URL(window.location.href);
          u.pathname = "/docs";
          Linking.openURL(u.toString());
        }}
      />
    </DrawerContentScrollView>
  );
}

export default function WebShell() {
  const theme = useTheme();
  const { isWeb, isElectron, isNative, isBrowser } = usePlatform();
  const navigation = useNavigation();
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
  const [livePopup, setLivePopup] = useState(false);

  const sidebar = useSidebarControl();
  const [drawerWidth, setDrawerWidth] = useState(sidebar.animatedWidth.value);

  useAnimatedReaction(
    () => sidebar.animatedWidth.value,
    (current) => {
      runOnJS(setDrawerWidth)(current);
    },
    [sidebar.animatedWidth],
  );

  const toast = useToast();

  // Top-level stuff to handle push notification registration
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

  // Stuff to handle incoming push notification routing
  const notificationDestination = useNotificationDestination();
  const linkTo = useLinkTo();

  useEffect(() => {
    if (notificationDestination) {
      linkTo(notificationDestination);
      clearNotification();
    }
  }, [notificationDestination]);

  // Top-level stuff to handle polling for live streamers
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

  let foregroundColor = theme.theme.colors.text || "#fff";

  // Track if we're on LiveDashboard
  const [currentRouteName, setCurrentRouteName] = useState<
    string | undefined
  >();

  useEffect(() => {
    const unsubscribe = navigation.addListener("state", () => {
      const state = navigation.getState();
      if (state?.routes) {
        const currentRoute = state.routes[state.index];
        setCurrentRouteName(currentRoute?.name);
      }
    });
    return unsubscribe;
  }, [navigation]);

  const isLiveDashboard = currentRouteName === "LiveDashboard";

  useEffect(() => {
    if (!isLiveDashboard && userIsLive && !livePopup) {
      setLivePopup(true);
      toast.show("You are live!", "Do you want to go to your Live Dashboard?", {
        actionLabel: "Go",
        onAction: () => {
          navigation.navigate("LiveDashboard" as never);
          setLivePopup(false);
        },
        onClose: () => setLivePopup(false),
        variant: "error",
        duration: 8,
      });
    }
  }, [userIsLive, isLiveDashboard, livePopup]);
  const externalItems = useExternalItems();

  if (!hydrated) {
    return <View />;
  }

  return (
    <>
      <StatusBar barStyle="light-content" />
      <Drawer.Navigator
        screenOptions={{
          // for the custom sidebar
          drawerType: sidebar.isActive ? "permanent" : "front",
          swipeEnabled: !sidebar.isActive,
          drawerStyle: {
            zIndex: 128000,
            width: sidebar.isActive ? drawerWidth : undefined,
          },
          headerLeft: ({ canGoBack }) => (
            <NavigationButton canGoBack={canGoBack} />
          ),
          headerRight: () => <AvatarButton />,
          headerShown: true,
          drawerActiveTintColor: "#a0287c33",
        }}
        drawerContent={
          sidebar.isActive
            ? (props) => (
                <Sidebar
                  {...props}
                  collapsed={sidebar.isCollapsed}
                  hidden={sidebar.isHidden}
                  widthAnim={sidebar.animatedWidth}
                  externalItems={externalItems}
                />
              )
            : CustomDrawerContent
        }
      >
        <Drawer.Screen
          name="HomeDrawer"
          component={RootNavigator}
          options={{
            drawerIcon: () => <Home color={foregroundColor} size={24} />,
            drawerLabel: () => <Text variant="h5">Home</Text>,
            headerShown: true,
            title: "Streamplace",
          }}
          listeners={{
            drawerItemPress: (e) => {
              e.preventDefault();
              navigation.dispatch(
                CommonActions.navigate({
                  name: "StreamList",
                }),
              );
            },
          }}
        />
        <Drawer.Screen
          name="AboutDrawer"
          component={Fragment}
          options={{
            drawerLabel: () => <Text variant="h5">What's Streamplace?</Text>,
            drawerIcon: () => (
              <ShieldQuestion color={foregroundColor} size={24} />
            ),
            drawerItemStyle: isNative ? { display: "none" } : undefined,
          }}
          listeners={{
            drawerItemPress: (e) => {
              e.preventDefault();
              navigation.dispatch(
                CommonActions.navigate({
                  name: "About",
                }),
              );
            },
          }}
        />
        <Drawer.Screen
          name="DownloadDrawer"
          component={Fragment}
          options={{
            drawerLabel: () => <Text variant="h5">Download</Text>,
            drawerIcon: () => <Download color={foregroundColor} size={24} />,
            drawerItemStyle: isBrowser ? undefined : { display: "none" },
          }}
          listeners={{
            drawerItemPress: (e) => {
              e.preventDefault();
              navigation.dispatch(
                CommonActions.navigate({
                  name: "Download",
                }),
              );
            },
          }}
        />
        <Drawer.Screen
          name="SettingsDrawer"
          component={Fragment}
          options={{
            drawerIcon: () => (
              <SettingsIcon color={foregroundColor} size={24} />
            ),
            drawerLabel: () => <Text variant="h5">Settings</Text>,
          }}
          listeners={{
            drawerItemPress: (e) => {
              e.preventDefault();
              navigation.dispatch(
                CommonActions.navigate({
                  name: "MainSettings",
                }),
              );
            },
          }}
        />
        <Drawer.Screen
          name="SupportDrawer"
          component={Fragment}
          options={{
            drawerLabel: () => <Text variant="h5">Support</Text>,
            drawerItemStyle: { display: "none" },
          }}
          listeners={{
            drawerItemPress: (e) => {
              e.preventDefault();
              navigation.dispatch(
                CommonActions.navigate({
                  name: "Support",
                }),
              );
            },
          }}
        />
        <Drawer.Screen
          name="LiveDashboardDrawer"
          component={Fragment}
          options={{
            drawerLabel: () => <Text variant="h5">Live Dashboard</Text>,
            drawerIcon: () => <Video color={foregroundColor} size={24} />,
            drawerItemStyle: isNative ? { display: "none" } : undefined,
          }}
          listeners={{
            drawerItemPress: (e) => {
              e.preventDefault();
              navigation.dispatch(
                CommonActions.navigate({
                  name: "LiveDashboard",
                }),
              );
            },
          }}
        />
        <Drawer.Screen
          name="LoginDrawer"
          component={Fragment}
          options={{
            drawerIcon: () => <LogIn color={foregroundColor} size={24} />,
            drawerLabel: () => <Text variant="h5">Login</Text>,
          }}
          listeners={{
            drawerItemPress: (e) => {
              e.preventDefault();
              navigation.dispatch(
                CommonActions.navigate({
                  name: "Login",
                }),
              );
            },
          }}
        />
      </Drawer.Navigator>
      <LoginModal visible={showLoginModal} onClose={closeLoginModal} />
    </>
  );
}
