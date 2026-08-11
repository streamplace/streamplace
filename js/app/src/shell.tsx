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
  useTheme,
  zero,
} from "@streamplace/components";
import { colors, spacing } from "@streamplace/components/src/lib/theme/tokens";
import { Settings } from "components";
import { SiteTitleLockup, useNodeTitle } from "components/brand/logo";
import { LogoBrandMenu } from "components/brand/logo-brand-menu";
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
import { BrandingAdmin } from "components/settings/branding-admin";
import { DanmuCategorySettings } from "components/settings/danmu-category-settings";
import KeyManager from "components/settings/key-manager";
import { LanguagesCategorySettings } from "components/settings/languages-category-settings";
import MultistreamManager from "components/settings/multistream-manager";
import { NotificationsCategorySettings } from "components/settings/notifications-category-settings";
import { PrivacyCategorySettings } from "components/settings/privacy-category-settings";
import RecommendationsManager from "components/settings/recommendations-manager";
import { StreamingCategorySettings } from "components/settings/streaming-category-settings";
import WebhookManager from "components/settings/webhook-manager";
import {
  SidebarOverlay,
  SidebarToggle,
} from "components/sidebar/sidebar-overlay";
import UploadProgressIndicator from "components/upload/upload-progress-indicator";
import { useBlueskyNotifications } from "hooks/useBlueskyNotifications";
import usePlatform from "hooks/usePlatform";
import { useIsLargeScreen, useSidebarControl } from "hooks/useSidebarControl";
import { Clapperboard, Cog, Home, Video } from "lucide-react-native";
import { useEffect, useState } from "react";
import { Platform, Pressable, StatusBar, View } from "react-native";
import Animated, { useAnimatedStyle } from "react-native-reanimated";
import { SFSymbols7_0 } from "sf-symbols-typescript";
import "src/navigation-types";
import AboutScreen from "src/screens/about";
import AppReturnScreen from "src/screens/app-return";
import BrandScreen from "src/screens/brand";
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
import UploadScreen, {
  UploadDraftsScreen,
  UploadLivestreamsScreen,
  UploadVideoScreen,
  UploadVideosScreen,
} from "src/screens/upload";
import VideoScreen from "src/screens/video";
import VideoListScreen from "src/screens/video-list";
import VodScreen from "src/screens/vod";
import VodEmbedScreen from "src/screens/vod-embed";
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

const AnimatedPressable = Animated.createAnimatedComponent(Pressable);

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
    // Slim, quiet chrome: 15px medium title on surface0 with a hairline
    headerTitleStyle: {
      fontFamily: z.theme.typography.universal.xl.fontFamily,
      fontSize: 15,
      fontWeight: "500" as const,
      color: z.theme.colors.text1,
    },
    headerStyle: {
      backgroundColor: z.theme.colors.surface0,
      borderBottomColor: z.theme.colors.borderSubtle,
      borderBottomWidth: 1,
      // Slimmer bar, aligned with the 56px sidebar brand row.
      height: 56,
    },
  };
}

// Home navigator (contains home + all general navigation screens)
function HomeNavigator() {
  const title = useNodeTitle();
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
      fontFamily: z.theme.typography.universal.xl.fontFamily,
      fontSize: 15,
      fontWeight: "500" as const,
      color: z.theme.colors.text1,
    },
  };

  return (
    <HomeStack.Navigator screenOptions={baseScreenOptions}>
      <HomeStack.Screen
        name="HomeMain"
        component={HomeScreen}
        options={{
          // `title` drives the browser tab: the node's title (runtime
          // branding, else the brand default). The visible web header shows
          // the contextual "Home", not a repeat of it.
          title,
          headerTitle:
            Platform.OS === "ios"
              ? (props) => (
                  <View style={{ flex: 1, alignItems: "flex-start" }}>
                    <Text size="3xl" style={[zero.ml[4]]}>
                      {title}
                    </Text>
                  </View>
                )
              : "Home",
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
        name="Brand"
        component={BrandScreen}
        options={{ title: "Brand Guidelines", ...headerScreenOptions }}
      />
      <HomeStack.Screen
        name="Download"
        component={DownloadScreen}
        options={{ title: "Download", ...headerScreenOptions }}
      />
      <HomeStack.Screen
        name="LiveDashboard"
        component={LiveDashboard}
        options={{ title: "Live streaming", ...headerScreenOptions }}
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
      {!isNative && (
        <HomeStack.Screen
          name="UploadVideo"
          component={UploadVideoScreen}
          options={{ title: "Edit Video", ...headerScreenOptions }}
        />
      )}
      {!isNative && (
        <HomeStack.Screen
          name="UploadDrafts"
          component={UploadDraftsScreen}
          options={{ title: "Drafts", ...headerScreenOptions }}
        />
      )}
      {!isNative && (
        <HomeStack.Screen
          name="UploadLivestreams"
          component={UploadLivestreamsScreen}
          options={{ title: "Livestreams", ...headerScreenOptions }}
        />
      )}
      {!isNative && (
        <HomeStack.Screen
          name="UploadVideos"
          component={UploadVideosScreen}
          options={{ title: "My Videos", ...headerScreenOptions }}
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
  const headerScreenOptions = {
    ...baseScreenOptions,
    headerTransparent: Platform.OS === "ios",
    headerBackButtonDisplayMode: "minimal" as const,
    headerShown: true,
    // Same Create + avatar cluster as the other navigators, so the header
    // controls don't vanish on Settings screens.
    headerRight: () => (
      <View style={{ flexDirection: "row", alignItems: "center" }}>
        <UploadButton />
        <LGAvatarButton />
      </View>
    ),
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
        name="NotificationsCategory"
        component={NotificationsCategorySettings}
        options={{ title: "Notifications" }}
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
      <SettingsStack.Screen
        name="BrandingAdmin"
        component={BrandingAdmin}
        options={{ title: "Branding" }}
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
          ? {
              backgroundColor: z.theme.colors.surface1,
              borderTopColor: z.theme.colors.borderSubtle,
            }
          : !isLargeScreen
            ? {
                backgroundColor: z.theme.colors.surface1,
                borderTopColor: z.theme.colors.borderSubtle,
              }
            : { display: "none" },
        tabBarActiveTintColor:
          accentColor || primaryColor || z.theme.colors.primary,
        tabBarInactiveTintColor: z.theme.colors.text3,
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

// Walk the (possibly nested) navigation state to the deepest active route, so
// checks against leaf screen names like "LiveDashboard" resolve correctly —
// getState().routes[index].name alone only yields the top-level route
// (e.g. "MainTabs"), so nested screens never match.
function getActiveLeafRouteName(state: any): string | undefined {
  if (!state?.routes) return undefined;
  const route = state.routes[state.index ?? 0];
  if (route?.state) return getActiveLeafRouteName(route.state) ?? route.name;
  return route?.name;
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
  const z = useTheme();
  const baseScreenOptions = useBaseScreenOptions();

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

  useBlueskyNotifications();

  // Track current route
  const [currentRouteName, setCurrentRouteName] = useState<string | undefined>(
    () => {
      try {
        return getActiveLeafRouteName(navigation.getState());
      } catch {
        return undefined;
      }
    },
  );

  useEffect(() => {
    const update = () => {
      const name = getActiveLeafRouteName(navigation.getState());
      if (name) setCurrentRouteName(name);
    };
    update(); // seed on mount so direct loads (e.g. a stream URL) resolve now
    const unsubscribe = navigation.addListener("state", update);
    return unsubscribe;
  }, [navigation]);

  // Detail views (watching a stream or video) take the sidebar out of the flow
  // so it opens as an overlay drawer over dimmed content instead of pushing.
  const setOverlay = useStore((state) => state.setOverlay);
  const closeDrawer = useStore((state) => state.closeDrawer);
  const isDetailView =
    currentRouteName === "Stream" ||
    currentRouteName === "Video" ||
    currentRouteName === "Vod";
  // Video pages get a YouTube-style sticky translucent header the content
  // scrolls under; the livestream keeps its own solid header.
  const isVodDetail =
    currentRouteName === "Video" || currentRouteName === "Vod";
  useEffect(() => {
    setOverlay(isDetailView);
  }, [isDetailView, setOverlay]);

  // Animate content margin when sidebar is active (web only). In overlay mode
  // the content stays full width (margin 0) and the drawer floats over it.
  const animatedContentStyle = useAnimatedStyle(() => {
    if (isNative || !sidebar.isActive) {
      return { marginLeft: 0 };
    }
    return {
      marginLeft: sidebar.animatedContentMargin.value,
    };
  });

  // Scrim that dims content behind the overlay drawer.
  const scrimStyle = useAnimatedStyle(() => ({
    opacity: sidebar.animatedScrim.value,
  }));

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
            // Reuse the shared chrome (surface0 + hairline, 15px title) so
            // pushed screens like Stream don't render a differently-colored
            // header than the HomeStack.
            ...baseScreenOptions,
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
            name="Video"
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
            name="VodEmbed"
            component={VodEmbedScreen}
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
      {/* Scrim behind the overlay drawer (detail views only) */}
      {!isNative && (
        <AnimatedPressable
          accessibilityLabel="Close menu"
          pointerEvents={
            sidebar.overlay && sidebar.drawerOpen ? "auto" : "none"
          }
          onPress={closeDrawer}
          style={[
            scrimStyle,
            {
              position: "absolute",
              top: 0,
              left: 0,
              right: 0,
              bottom: 0,
              backgroundColor: "#000", // token-ok: overlay scrim
              zIndex: 127000,
            },
          ]}
        />
      )}
      {/* Header cluster on detail views: hamburger + logo, sitting exactly where
          the drawer's brand row will appear so it stays put as the drawer opens. */}
      {!isNative &&
        sidebar.isActive &&
        sidebar.overlay &&
        !sidebar.drawerOpen && (
          <View
            style={[
              {
                position: "absolute",
                top: 0,
                height: 56,
                flexDirection: "row",
                alignItems: "center",
                // Match the sidebar brand row (gap 0) so the logo sits in the
                // same spot when navigating between the sidebar and detail-view
                // headers.
                gap: 0,
                zIndex: 128001,
              },
              isVodDetail
                ? // Full-width liquid-glass bar the video scrolls under.
                  ({
                    left: 0,
                    right: 0,
                    paddingHorizontal: spacing[2],
                    backgroundColor: "rgba(10,10,11,0.55)", // token-ok: glass tint
                    backdropFilter: "blur(18px)",
                    WebkitBackdropFilter: "blur(18px)",
                    borderBottomWidth: 1,
                    borderBottomColor: z.theme.colors.borderSubtle,
                  } as any)
                : { left: spacing[2] },
            ]}
          >
            <SidebarToggle label="Open menu" onPress={sidebar.toggle} />
            <LogoBrandMenu>
              <Pressable
                // @ts-ignore renders as <a> on web
                href="/"
                style={{ flexShrink: 1, minWidth: 0 }}
                onPress={(e: any) => {
                  e?.preventDefault?.();
                  navigation.navigate("MainTabs" as any, {
                    screen: "HomeTab",
                    params: { screen: "HomeMain" },
                  });
                }}
              >
                <SiteTitleLockup
                  size={19}
                  weight="semibold"
                  letterSpacing={0}
                  markColor={colors.white}
                  color={colors.white}
                />
              </Pressable>
            </LogoBrandMenu>
          </View>
        )}
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
      <UploadProgressIndicator />
    </View>
  );
}
