import {
  BottomTabIcon,
  createBottomTabNavigator,
} from "@react-navigation/bottom-tabs";
import { useLinkTo, useNavigation } from "@react-navigation/native";
import { createNativeStackNavigator } from "@react-navigation/native-stack";
import {
  Text,
  useAccentColor,
  usePrimaryColor,
  useSiteTitle,
  useTheme,
  useToast,
  zero,
} from "@streamplace/components";
import { Settings } from "components";
import Login from "components/login/login";
import LoginModal from "components/login/login-modal";
import PdsHostSelectorModal from "components/login/pds-host-selector-modal";
import { AboutCategorySettings } from "components/settings/about-category-settings";
import { AccountCategorySettings } from "components/settings/account-category-settings";
import { AdvancedCategorySettings } from "components/settings/advanced-category-settings";
import { DanmuCategorySettings } from "components/settings/danmu-category-settings";
import KeyManager from "components/settings/key-manager";
import { LanguagesCategorySettings } from "components/settings/languages-category-settings";
import MultistreamManager from "components/settings/multistream-manager";
import { PrivacyCategorySettings } from "components/settings/privacy-category-settings";
import RecommendationsManager from "components/settings/recommendations-manager";
import { StreamingCategorySettings } from "components/settings/streaming-category-settings";
import WebhookManager from "components/settings/webhook-manager";
import { SidebarOverlay } from "components/sidebar/sidebar-overlay";
import { useBlueskyNotifications } from "hooks/useBlueskyNotifications";
import { useLiveUser } from "hooks/useLiveUser";
import usePlatform from "hooks/usePlatform";
import { useSidebarControl } from "hooks/useSidebarControl";
import { useEffect, useRef, useState } from "react";
import { Platform, StatusBar, View } from "react-native";
import Animated, { useAnimatedStyle } from "react-native-reanimated";
import { SFSymbols7_0 } from "sf-symbols-typescript";
import "src/navigation-types";
import AboutScreen from "src/screens/about";
import AppReturnScreen from "src/screens/app-return";
import PopoutChat from "src/screens/chat-popout";
import DanmuOBSScreen from "src/screens/danmu-obs";
import DownloadScreen from "src/screens/download";
import EmbedScreen from "src/screens/embed";
import HomeScreen from "src/screens/home";
import InfoWidgetEmbed from "src/screens/info-widget-embed";
import LaunchGoLive from "src/screens/launch-go-live";
import LiveDashboard from "src/screens/live-dashboard";
import MobileGoLive from "src/screens/mobile-go-live";
import MobileStream from "src/screens/mobile-stream";
import MultiScreen from "src/screens/multi";
import SupportScreen from "src/screens/support";
import { useStore } from "store";
import {
  useHydrated,
  useNotificationDestination,
  useNotificationToken,
} from "store/hooks";
import { AvatarButton, LGAvatarButton, NavigationButton } from "./router";

const Tab = createBottomTabNavigator();
const RootStack = createNativeStackNavigator();
const HomeStack = createNativeStackNavigator();
const SettingsStack = createNativeStackNavigator();

// Home navigator (contains home + all general navigation screens)
function HomeNavigator() {
  const title = useSiteTitle() || "Streamplace Station";
  return (
    <HomeStack.Navigator
      screenOptions={{
        headerShown: true,
        headerTransparent: Platform.OS === "ios",
        headerBackButtonDisplayMode: "minimal",
      }}
    >
      <HomeStack.Screen
        name="HomeMain"
        component={HomeScreen}
        options={{
          title: "Streamplace",
          headerTitle:
            Platform.OS === "ios"
              ? (props) => (
                  <View style={{ flex: 1, alignItems: "flex-start" }}>
                    <Text size="3xl" style={[zero.ml[4]]}>
                      {title}
                    </Text>
                  </View>
                )
              : undefined,
          headerLeft:
            Platform.OS !== "ios"
              ? ({ canGoBack }) => <NavigationButton canGoBack={canGoBack} />
              : undefined,
          headerRight: () => <AvatarButton />,
          ...(Platform.OS === "ios" && {
            unstable_headerRightItems: () => [
              {
                type: "custom",
                hidesSharedBackground: true,
                element: <LGAvatarButton />,
              },
            ],
          }),
        }}
      />
      <HomeStack.Screen
        name="About"
        component={AboutScreen}
        options={{ title: "What's Streamplace?" }}
      />
      <HomeStack.Screen
        name="Download"
        component={DownloadScreen}
        options={{ title: "Download" }}
      />
      <HomeStack.Screen
        name="LiveDashboard"
        component={LiveDashboard}
        options={{ title: "Live Dashboard" }}
      />
      <HomeStack.Screen
        name="Login"
        component={Login}
        options={{ title: "Login" }}
      />
      <HomeStack.Screen
        name="Multi"
        component={MultiScreen}
        options={{ title: "Multi-stream" }}
      />
      <HomeStack.Screen
        name="Support"
        component={SupportScreen}
        options={{ title: "Support" }}
      />
    </HomeStack.Navigator>
  );
}

// Settings stack navigator
function SettingsNavigator() {
  const z = useTheme();
  return (
    <SettingsStack.Navigator
      initialRouteName="MainSettings"
      screenOptions={{
        headerShown: true,
        headerTransparent: Platform.OS === "ios",
        headerBackButtonDisplayMode: "minimal",
        headerTitleStyle: {
          fontFamily: z.theme.typography.universal["2xl"].fontFamily,
        },
      }}
    >
      <SettingsStack.Screen
        name="MainSettings"
        component={Settings}
        options={{ title: "Settings" }}
      />
      <SettingsStack.Screen
        name="AboutCategory"
        component={AboutCategorySettings}
        options={{ title: "About" }}
      />
      <SettingsStack.Screen
        name="AccountCategory"
        component={AccountCategorySettings}
        options={{ title: "Account" }}
      />
      <SettingsStack.Screen
        name="StreamingCategory"
        component={StreamingCategorySettings}
        options={{ title: "Streaming" }}
      />
      <SettingsStack.Screen
        name="WebhooksSettings"
        component={WebhookManager}
        options={{ title: "Webhooks" }}
      />
      <SettingsStack.Screen
        name="RecommendationsSettings"
        component={RecommendationsManager}
        options={{ title: "Recommendations" }}
      />
      <SettingsStack.Screen
        name="PrivacyCategory"
        component={PrivacyCategorySettings}
        options={{ title: "Privacy & Security" }}
      />
      <SettingsStack.Screen
        name="DanmuCategory"
        component={DanmuCategorySettings}
        options={{ title: "Danmu" }}
      />
      <SettingsStack.Screen
        name="AdvancedCategory"
        component={AdvancedCategorySettings}
        options={{ title: "Advanced" }}
      />
      <SettingsStack.Screen
        name="MultistreamCategory"
        component={MultistreamManager}
        options={{ title: "Multistream" }}
      />
      <SettingsStack.Screen
        name="LanguagesCategory"
        component={LanguagesCategorySettings}
        options={{ title: "Languages" }}
      />
      <SettingsStack.Screen
        name="KeyManagement"
        component={KeyManager}
        options={{ title: "Key Manager" }}
      />
    </SettingsStack.Navigator>
  );
}

const IOS_ICONS: Record<string, SFSymbols7_0> = {
  Home: "house.fill",
  GoLive: "video.fill",
  Settings: "gearshape.fill",
};
const ANDROID_ICONS = {
  Home: "home",
  GoLive: "videocam",
  Settings: "settings",
};

const getIcon = (
  name: keyof typeof IOS_ICONS | keyof typeof ANDROID_ICONS,
): BottomTabIcon => {
  if (Platform.OS === "ios") {
    return {
      type: "sfSymbol",
      name: IOS_ICONS[name],
    };
  }
  return {
    type: "materialSymbol",
    name: ANDROID_ICONS[name],
  };
};

// Tab navigator (main app sections, navigation on web is handled in sidebar)
function TabNavigator() {
  const { isNative, isBrowser } = usePlatform();
  const accentColor = useAccentColor();
  const primaryColor = usePrimaryColor();
  const z = useTheme();

  return (
    <Tab.Navigator
      screenOptions={{
        lazy: true,
        headerShown: false,
        // Hide tab bar on web
        tabBarStyle: isNative ? undefined : { display: "none" },
        // doesn't seem to work on iOS?
        // tabBarInactiveTintColor: primaryColor || accentColor || "#f0f",
        // tabBarActiveTintColor: accentColor || primaryColor || "#06f",
        headerTitleStyle: {
          fontFamily: z.theme.typography.universal["2xl"].fontFamily,
        },
      }}
    >
      <Tab.Screen
        name="HomeTab"
        component={HomeNavigator}
        options={{
          title: "Home",
          ...(isNative && {
            tabBarIcon: getIcon("Home"),
          }),
        }}
      />
      <Tab.Screen
        name="GoLiveTab"
        component={LaunchGoLive}
        options={{
          title: "Go Live",
          ...(isNative && {
            tabBarIcon: getIcon("GoLive"),
          }),
          headerShown: true,
          headerTransparent: true,
        }}
      />
      <Tab.Screen
        name="SettingsTab"
        component={SettingsNavigator}
        options={{
          title: "Settings",
          ...(isNative && {
            tabBarIcon: getIcon("Settings"),
          }),
          headerShown: false,
        }}
      />
    </Tab.Navigator>
  );
}

export default function Shell() {
  const { isNative } = usePlatform();
  const sidebar = useSidebarControl();
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
  const showPdsModal = useStore((state) => state.showPdsModal);
  const openPdsModal = useStore((state) => state.openPdsModal);
  const closePdsModal = useStore((state) => state.closePdsModal);
  const loginAction = useStore((state) => state.login);
  const openLoginLink = useStore((state) => state.openLoginLink);
  const livePopupShown = useRef(false);
  const z = useTheme();

  const toast = useToast();

  // Top-level hydration and initialization
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

  // Track current route
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

  const noLivePopupRoutes =
    currentRouteName === "LiveDashboard" ||
    currentRouteName === "GoLiveTab" ||
    currentRouteName === "MobileGoLive";

  // Show "You are live!" toast once per live session
  useEffect(() => {
    if (!userIsLive) {
      livePopupShown.current = false;
      return;
    }
    if (!noLivePopupRoutes && !livePopupShown.current) {
      livePopupShown.current = true;
      toast.show("You are live!", "Do you want to go to your Live Dashboard?", {
        actionLabel: "Go",
        onAction: () => {
          navigation.navigate("MainTabs" as any, {
            screen: "HomeTab",
            params: { screen: "LiveDashboard" },
          });
        },
        variant: "error",
        duration: 8,
      });
    }
  }, [userIsLive, noLivePopupRoutes]);

  // Animate content margin when sidebar is active (web only)
  const animatedContentStyle = useAnimatedStyle(() => {
    if (isNative || !sidebar.isActive) {
      return { marginLeft: 0 };
    }
    return {
      marginLeft: sidebar.animatedWidth.value,
    };
  });

  if (!hydrated) {
    return <View />;
  }

  return (
    <>
      <StatusBar barStyle="light-content" />
      {!isNative && <SidebarOverlay />}
      <Animated.View style={[{ flex: 1 }, animatedContentStyle]}>
        <RootStack.Navigator
          screenOptions={{
            headerShown: !isNative,
            headerLeft: ({ canGoBack }) => (
              <NavigationButton canGoBack={canGoBack} />
            ),
            headerRight: () => <LGAvatarButton />,
            ...(isNative && {
              headerTransparent: true,
            }),
            headerTitleStyle: {
              fontFamily: z.theme.typography.universal.base.fontFamily,
            },
          }}
        >
          {/* Main tabs (initial screen for all platforms) */}
          <RootStack.Screen
            name="MainTabs"
            component={TabNavigator}
            options={{ headerShown: false }}
          />

          {/* Full-screen screens that should NOT have tab bar accessible on mobile */}
          <RootStack.Screen
            name="Stream"
            component={MobileStream}
            options={{
              headerShown: Platform.OS === "web",
              headerTitle: "",
            }}
          />
          <RootStack.Screen
            name="MobileGoLive"
            component={MobileGoLive}
            options={{ headerShown: false }}
          />

          {/* Utility/embed screens */}
          <RootStack.Screen
            name="AppReturn"
            component={AppReturnScreen}
            options={{ title: "Returning to app..." }}
          />
          <RootStack.Screen
            name="PopoutChat"
            component={PopoutChat}
            options={{ headerShown: false }}
          />
          <RootStack.Screen
            name="Embed"
            component={EmbedScreen}
            options={{ headerShown: false }}
          />
          <RootStack.Screen
            name="InfoWidgetEmbed"
            component={InfoWidgetEmbed}
            options={{ headerShown: false }}
          />
          <RootStack.Screen
            name="DanmuOBS"
            component={DanmuOBSScreen}
            options={{ headerShown: false }}
          />
        </RootStack.Navigator>
      </Animated.View>
      <LoginModal
        visible={showLoginModal}
        onClose={closeLoginModal}
        onOpenPdsModal={openPdsModal}
      />
      <PdsHostSelectorModal
        open={showPdsModal}
        onOpenChange={closePdsModal}
        onSubmit={(pdsHost) => {
          closePdsModal();
          loginAction(pdsHost, openLoginLink);
        }}
      />
    </>
  );
}
