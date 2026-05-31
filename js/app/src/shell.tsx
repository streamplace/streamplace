import {
  BottomTabIcon,
  createBottomTabNavigator,
} from "@react-navigation/bottom-tabs";
import { useLinkTo, useNavigation } from "@react-navigation/native";
import {
  createNativeStackNavigator,
  NativeStackHeaderBackProps,
} from "@react-navigation/native-stack";
import {
  Text,
  useAccentColor,
  useDID,
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
import { MobileAppBanner } from "components/mobile-app-banner";
import { AboutCategorySettings } from "components/settings/about-category-settings";
import { AccountCategorySettings } from "components/settings/account-category-settings";
import { AdvancedCategorySettings } from "components/settings/advanced-category-settings";
import { BackupSettings } from "components/settings/backup-settings";
import { BadgeIssuerPanel } from "components/settings/badge-issuer-panel";
import { BadgeSelectionManager } from "components/settings/badge-selection-manager";
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
import { useIsLargeScreen, useSidebarControl } from "hooks/useSidebarControl";
import { Clapperboard, Cog, Home, Video } from "lucide-react-native";
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
import PopoutInfoWidget from "src/screens/popout-info-widget";
import PopoutLivestream from "src/screens/popout-livestream";
import PopoutMultistream from "src/screens/popout-multistream";
import PopoutStreamMonitor from "src/screens/popout-stream-monitor";
import SupportScreen from "src/screens/support";
import UploadScreen from "src/screens/upload";
import VideoScreen from "src/screens/video";
import VideoListScreen from "src/screens/video-list";
import VodScreen from "src/screens/vod";
import { useStore } from "store";
import {
  useHydrated,
  useNotificationDestination,
  useNotificationToken,
} from "store/hooks";
import {
  AvatarButton,
  LGAvatarButton,
  NavigationButton,
  UploadButton,
} from "./router";

const Tab = createBottomTabNavigator();
const RootStack = createNativeStackNavigator();
const HomeStack = createNativeStackNavigator();
const VideosStack = createNativeStackNavigator();
const SettingsStack = createNativeStackNavigator();

function useBaseScreenOptions() {
  const z = useTheme();
  return {
    headerShown: true,
    headerTransparent: Platform.OS === "ios",
    headerBackButtonDisplayMode: "minimal" as const,
    headerTitleStyle: {
      fontFamily: z.theme.typography.universal["2xl"].fontFamily,
    },
    headerStyle: {
      backgroundColor: z.theme.colors.background,
      borderBottomColor: z.theme.colors.border,
      borderBottomWidth: 1,
    },
  };
}

// Home navigator (contains home + all general navigation screens)
function HomeNavigator() {
  const title = useSiteTitle() || "Streamplace Station";
  const baseScreenOptions = useBaseScreenOptions();
  const isNative = Platform.OS !== "web";
  const z = useTheme();
  const did = useDID();

  const headerScreenOptions = {
    headerShown: !isNative,
    headerLeft: isNative
      ? undefined
      : ({ canGoBack }: NativeStackHeaderBackProps) => (
          <NavigationButton canGoBack={canGoBack} />
        ),
    headerRight: () => (
      <View style={{ flexDirection: "row", alignItems: "center" }}>
        <UploadButton />
        <LGAvatarButton />
      </View>
    ),
    ...(isNative && {
      headerTransparent: true,
    }),
    headerTitleStyle: {
      fontFamily: z.theme.typography.universal.base.fontFamily,
    },
  };

  return (
    <HomeStack.Navigator screenOptions={baseScreenOptions}>
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
          headerRight: () => (
            <View style={{ flexDirection: "row", alignItems: "center" }}>
              <UploadButton />
              <AvatarButton />
            </View>
          ),
          ...(Platform.OS === "ios" && {
            unstable_headerRightItems: () => [
              {
                type: "custom",
                hidesSharedBackground: true,
                element: (
                  <View style={{ flexDirection: "row", alignItems: "center" }}>
                    <UploadButton />
                    <LGAvatarButton />
                  </View>
                ),
              },
            ],
          }),
        }}
      />
      <HomeStack.Screen
        name="About"
        component={AboutScreen}
        options={{
          title: "What's Streamplace?",
          ...headerScreenOptions,
        }}
      />
      <HomeStack.Screen
        name="Download"
        component={DownloadScreen}
        options={{ title: "Download", ...headerScreenOptions }}
      />
      <HomeStack.Screen
        name="LiveDashboard"
        component={LiveDashboard}
        options={{ title: "Live Dashboard", ...headerScreenOptions }}
      />
      <HomeStack.Screen
        name="Login"
        component={Login}
        options={{ title: did ? "Account" : "Login", ...headerScreenOptions }}
      />
      <HomeStack.Screen
        name="Multi"
        component={MultiScreen}
        options={{ title: "Multi-stream", ...headerScreenOptions }}
      />
      <HomeStack.Screen
        name="Support"
        component={SupportScreen}
        options={{ title: "Support", ...headerScreenOptions }}
      />
      {!isNative && (
        <HomeStack.Screen
          name="Upload"
          component={UploadScreen}
          options={{ title: "Upload Video", ...headerScreenOptions }}
        />
      )}
    </HomeStack.Navigator>
  );
}

// Videos stack navigator (global + per-user VOD listings). Unlike the pushed
// Home screens, these are tab roots, so they always carry their own header.
function VideosNavigator() {
  const baseScreenOptions = useBaseScreenOptions();
  const isNative = Platform.OS !== "web";
  const z = useTheme();

  return (
    <VideosStack.Navigator
      screenOptions={{
        ...baseScreenOptions,
        headerLeft: isNative
          ? undefined
          : ({ canGoBack }: NativeStackHeaderBackProps) => (
              <NavigationButton canGoBack={canGoBack} />
            ),
        headerRight: () => (
          <View style={{ flexDirection: "row", alignItems: "center" }}>
            <UploadButton />
            <LGAvatarButton />
          </View>
        ),
        headerTitleStyle: {
          fontFamily: z.theme.typography.universal.base.fontFamily,
        },
      }}
    >
      <VideosStack.Screen
        name="VideoList"
        component={VideoListScreen}
        options={{ title: "Videos" }}
      />
      <VideosStack.Screen
        name="UserVideoList"
        component={VideoListScreen}
        options={{ title: "Videos" }}
      />
    </VideosStack.Navigator>
  );
}

// Settings stack navigator
function SettingsNavigator() {
  const baseScreenOptions = useBaseScreenOptions();
  const z = useTheme();
  const isNative = Platform.OS !== "web";
  const headerScreenOptions = {
    ...baseScreenOptions,
    headerTransparent: Platform.OS === "ios",
    headerBackButtonDisplayMode: "minimal" as const,
    headerShown: true,
    // headerLeft: isNative
    //   ? undefined
    //   : ({ canGoBack }: NativeStackHeaderBackProps) => (
    //       <NavigationButton canGoBack={canGoBack} />
    //     ),
    // headerRight: () => <LGAvatarButton />,
    // ...(isNative && {
    //   headerTransparent: true,
    // }),
    // headerTitleStyle: {
    //   fontFamily: z.theme.typography.universal.base.fontFamily,
    //   marginBottom: 100,
    // },
  };
  return (
    <SettingsStack.Navigator
      initialRouteName="MainSettings"
      screenOptions={{
        ...headerScreenOptions,
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
        name="BackupSettings"
        component={BackupSettings}
        options={{ title: "Backup" }}
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
      <SettingsStack.Screen
        name="BadgeSelection"
        component={BadgeSelectionManager}
        options={{ title: "Badges" }}
      />
      <SettingsStack.Screen
        name="BadgeIssuer"
        component={BadgeIssuerPanel}
        options={{ title: "Issue Badges" }}
      />
    </SettingsStack.Navigator>
  );
}

const IOS_ICONS: Record<string, SFSymbols7_0> = {
  Home: "house.fill",
  Videos: "play.rectangle.fill",
  GoLive: "video.fill",
  Settings: "gearshape.fill",
};
const ANDROID_ICONS = {
  Home: "home",
  Videos: "video_library",
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
  } else {
    return {
      type: "materialSymbol",
      name: ANDROID_ICONS[name],
    };
  }
};

// Tab navigator (main app sections, navigation on web is handled in sidebar)
function TabNavigator() {
  const { isNative, isBrowser } = usePlatform();
  const accentColor = useAccentColor();
  const primaryColor = usePrimaryColor();
  const isLargeScreen = useIsLargeScreen();
  const z = useTheme();

  return (
    <Tab.Navigator
      screenOptions={{
        lazy: true,
        headerShown: false,
        // Hide tab bar on web and < 800px
        tabBarStyle: isNative
          ? undefined
          : !isLargeScreen
            ? undefined
            : { display: "none" },
        tabBarActiveTintColor: accentColor || primaryColor || "#06f",
        headerTitleStyle: {
          fontFamily: z.theme.typography.universal["2xl"].fontFamily,
        },
        headerStyle: {
          backgroundColor: z.theme.colors.background,
        },
      }}
    >
      <Tab.Screen
        name="HomeTab"
        component={HomeNavigator}
        options={{
          title: "Home",
          ...(isNative
            ? {
                tabBarIcon: getIcon("Home"),
              }
            : {
                tabBarIcon: ({ color, size }) => (
                  <Home size={size} color={color} />
                ),
              }),
        }}
      />
      <Tab.Screen
        name="VideosTab"
        component={VideosNavigator}
        options={{
          title: "Videos",
          ...(isNative
            ? {
                tabBarIcon: getIcon("Videos"),
              }
            : {
                tabBarIcon: ({ color, size }) => (
                  <Clapperboard size={size} color={color} />
                ),
              }),
        }}
      />
      <Tab.Screen
        name="GoLiveTab"
        component={LaunchGoLive}
        options={{
          title: "Go Live",
          ...(isNative
            ? {
                tabBarIcon: getIcon("GoLive"),
              }
            : {
                tabBarIcon: ({ color, size }) => (
                  <Video size={size} color={color} />
                ),
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
          ...(isNative
            ? {
                tabBarIcon: getIcon("Settings"),
              }
            : {
                tabBarIcon: ({ color, size }) => (
                  <Cog size={size} color={color} />
                ),
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
  const did = useStore((state) => state.oauthSession?.did);
  const hydrated = useHydrated();

  // Re-register when the token changes OR once the logged-in DID resolves, so a
  // token acquired before the OAuth session finishes restoring still gets its
  // repoDID association registered (otherwise the user is excluded from
  // follower livestream notifications).
  useEffect(() => {
    if (notificationToken) {
      registerNotificationToken();
    }
  }, [notificationToken, did]);

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
        console.log("setCurrentRouteName", currentRoute?.name);
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
    <View style={{ flex: 1 }}>
      <StatusBar barStyle="light-content" />
      {!isNative && <SidebarOverlay />}
      {!isNative && <MobileAppBanner />}
      <Animated.View style={[{ flex: 1 }, animatedContentStyle]}>
        <RootStack.Navigator
          screenOptions={{
            headerShown: !isNative,
            headerLeft: ({ canGoBack }) => (
              <NavigationButton canGoBack={canGoBack} />
            ),
            headerRight: () => (
              <View style={{ flexDirection: "row", alignItems: "center" }}>
                <UploadButton />
                <LGAvatarButton />
              </View>
            ),
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
          <RootStack.Screen
            name="VodPlayerDemo"
            component={VideoScreen}
            options={{ headerShown: false }}
          />
          <RootStack.Screen
            name="Vod"
            component={VodScreen}
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
          <RootStack.Screen
            name="PopoutStreamMonitor"
            component={PopoutStreamMonitor}
            options={{ headerShown: false }}
          />
          <RootStack.Screen
            name="PopoutInfoWidget"
            component={PopoutInfoWidget}
            options={{ headerShown: false }}
          />
          <RootStack.Screen
            name="PopoutMultistream"
            component={PopoutMultistream}
            options={{ headerShown: false }}
          />
          <RootStack.Screen
            name="PopoutLivestream"
            component={PopoutLivestream}
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
    </View>
  );
}
