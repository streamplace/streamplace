import "@expo/metro-runtime";
import {
  createDrawerNavigator,
  DrawerContentScrollView,
  DrawerItem,
  DrawerItemList,
} from "@react-navigation/drawer";
import {
  CommonActions,
  DrawerActions,
  LinkingOptions,
  NavigatorScreenParams,
  useLinkTo,
  useNavigation,
  useRoute,
} from "@react-navigation/native";
import { createNativeStackNavigator } from "@react-navigation/native-stack";
import { Text, useTheme, useToast } from "@streamplace/components";
import { Provider, Settings } from "components";
import AQLink from "components/aqlink";
import Login from "components/login/login";
import LoginModal from "components/login/login-modal";
import { AboutCategorySettings } from "components/settings/about-category-settings";
import { AccountCategorySettings } from "components/settings/account-category-settings";
import { AdvancedCategorySettings } from "components/settings/advanced-category-settings";
import { DanmuCategorySettings } from "components/settings/danmu-category-settings";
import { PrivacyCategorySettings } from "components/settings/privacy-category-settings";
import { StreamingCategorySettings } from "components/settings/streaming-category-settings";
import WebhookManager from "components/settings/webhook-manager";
import Sidebar, { ExternalDrawerItem } from "components/sidebar/sidebar";
import * as ExpoLinking from "expo-linking";
import { useLiveUser } from "hooks/useLiveUser";
import usePlatform from "hooks/usePlatform";
import { useSidebarControl } from "hooks/useSidebarControl";
import {
  ArrowLeft,
  Book,
  Download,
  ExternalLink,
  Home,
  LogIn,
  Menu,
  PanelLeftClose,
  PanelLeftOpen,
  Settings as SettingsIcon,
  ShieldQuestion,
  User,
  Video,
} from "lucide-react-native";
import React, { Fragment, useEffect, useState } from "react";
import {
  ImageBackground,
  ImageSourcePropType,
  Linking,
  Platform,
  Pressable,
  StatusBar,
  View,
} from "react-native";
import AboutScreen from "./screens/about";
import AppReturnScreen from "./screens/app-return";
import PopoutChat from "./screens/chat-popout";
import DownloadScreen from "./screens/download";
import EmbedScreen from "./screens/embed";
import InfoWidgetEmbed from "./screens/info-widget-embed";
import LiveDashboard from "./screens/live-dashboard";
import MultiScreen from "./screens/multi";
import SupportScreen from "./screens/support";

import KeyManager from "components/settings/key-manager";

import HomeScreen from "./screens/home";

import { useUrl } from "@streamplace/components";
import { LanguagesCategorySettings } from "components/settings/languages-category-settings";
import RecommendationsManager from "components/settings/recommendations-manager";
import Constants from "expo-constants";
import { useBlueskyNotifications } from "hooks/useBlueskyNotifications";
import { SystemBars } from "react-native-edge-to-edge";
import {
  configureReanimatedLogger,
  ReanimatedLogLevel,
  useAnimatedStyle,
} from "react-native-reanimated";
import { useStore } from "store";
import {
  useHydrated,
  useNotificationDestination,
  useNotificationToken,
  useUserProfile,
} from "store/hooks";
import DanmuOBSScreen from "./screens/danmu-obs";
import MobileGoLive from "./screens/mobile-go-live";
import MobileStream from "./screens/mobile-stream";

// Initialize sidebar state on app load
useStore.getState().loadStateFromStorage();

const Stack = createNativeStackNavigator();

// disabled strict b/c chat swipeable triggers it a LOT and the resulting logging
// slows down the whole app
configureReanimatedLogger({
  level: ReanimatedLogLevel.warn,
  strict: false,
});

type HomeStackParamList = {
  StreamList: undefined;
  Stream: { user: string };
};

type SettingsStackParamList = {
  MainSettings: undefined;
  AboutCategory: undefined;
  AccountCategory: undefined;
  StreamingCategory: undefined;
  WebhooksSettings: undefined;
  RecommendationsSettings: undefined;
  PrivacyCategory: undefined;
  DanmuCategory: undefined;
  AdvancedCategory: undefined;
  LanguagesCategory: undefined;
  DeveloperSettings: undefined;
  KeyManagement: undefined;
};

type RootStackParamList = {
  Home: NavigatorScreenParams<HomeStackParamList>;
  Multi: { config: string };
  Support: undefined;
  Settings: NavigatorScreenParams<SettingsStackParamList>;
  KeyManagement: undefined;
  GoLive: undefined;
  LiveDashboard: undefined;
  Login: undefined;
  AVSync: undefined;
  AppReturn: { scheme: string };
  About: undefined;
  Download: undefined;
  PopoutChat: { user: string };
  Embed: { user: string };
  InfoWidgetEmbed: undefined;
  LegacyStream: { user: string };
  DanmuOBS: { user: string };
  MobileGoLive: undefined;
};

declare global {
  namespace ReactNavigation {
    interface RootParamList extends RootStackParamList {}
  }
}

const linking: LinkingOptions<ReactNavigation.RootParamList> = {
  prefixes: [ExpoLinking.createURL("")],
  config: {
    screens: {
      Home: {
        screens: {
          StreamList: "",
          Stream: {
            path: ":user",
          },
        },
      },
      Multi: "multi/:config",
      Support: "support",
      Settings: {
        screens: {
          MainSettings: "settings",
          AboutCategory: "settings/about",
          AccountCategory: "settings/account",
          StreamingCategory: "settings/streaming",
          WebhooksSettings: "settings/streaming/webhooks",
          RecommendationsSettings: "settings/streaming/recommendations",
          PrivacyCategory: "settings/privacy",
          DanmuCategory: "settings/danmu",
          AdvancedCategory: "settings/advanced",
          DeveloperSettings: "settings/developer",
        },
      },
      KeyManagement: "key-management",
      GoLive: "golive",
      LiveDashboard: "live",
      Login: "login",
      AVSync: "sync-test",
      AppReturn: "app-return/:scheme",
      About: "about",
      Download: "download",
      PopoutChat: "chat-popout/:user",
      Embed: "embed/:user",
      InfoWidgetEmbed: "info-widget",
      LegacyStream: "legacy/:user",
      DanmuOBS: "widgets/:user/danmu",
      MobileGoLive: "mobile-golive",
    },
  },
};

const associatedDomain = Constants.expoConfig?.ios?.associatedDomains?.[0];
if (associatedDomain && associatedDomain.startsWith("applinks:")) {
  const domain = associatedDomain.slice("applinks:".length);
  linking.prefixes.push(`https://${domain}`);
}

// https://github.com/streamplace/streamplace/issues/377
const hasDevDomain = linking.prefixes.some((prefix) =>
  prefix.includes("tv.aquareum.dev"),
);
if (hasDevDomain) {
  linking.prefixes.push("tv.aquareum://");
  linking.prefixes.push("https://stream.place");
}

console.log("Linking prefixes", linking.prefixes);

const Drawer = createDrawerNavigator();

const NavigationButton = ({ canGoBack }: { canGoBack?: boolean }) => {
  const sidebar = useSidebarControl();
  const navigation = useNavigation();
  const { theme } = useTheme();

  const handlePress = () => {
    if (sidebar?.isActive) {
      sidebar.toggle();
    }
  };

  const handleGoBackPress = () => {
    if (canGoBack) {
      navigation.goBack();
    } else {
      navigation.dispatch(DrawerActions.toggleDrawer());
    }
  };

  return (
    <View
      style={[
        { flexDirection: "row" },
        {
          marginLeft: Platform.OS === "android" ? 0 : 12,
          marginRight: Platform.OS === "android" ? 12 : 0,
        },
      ]}
    >
      {sidebar?.isActive ? (
        <>
          <Pressable style={{ padding: 5 }} onPress={handlePress}>
            {sidebar.isCollapsed ? (
              <PanelLeftOpen size={24} color={theme.colors.accentForeground} />
            ) : (
              <PanelLeftClose size={24} color={theme.colors.accentForeground} />
            )}
          </Pressable>
          {canGoBack && (
            <Pressable
              style={{ marginLeft: 10, paddingVertical: 5 }}
              onPress={handleGoBackPress}
            >
              <ArrowLeft size={24} color={theme.colors.accentForeground} />
            </Pressable>
          )}
        </>
      ) : (
        <Pressable style={{ padding: 5 }} onPress={handleGoBackPress}>
          {canGoBack ? (
            <ArrowLeft size={24} color={theme.colors.accentForeground} />
          ) : (
            <Menu size={24} color={theme.colors.accentForeground} />
          )}
        </Pressable>
      )}
    </View>
  );
};

const AvatarButton = () => {
  const userProfile = useUserProfile();
  let source: ImageSourcePropType | undefined = undefined;
  let opacity = 1;
  const targetScreen: any = userProfile
    ? { screen: "Settings", params: { screen: "AccountCategory" } }
    : { screen: "Login", params: {} };

  if (userProfile) {
    source = { uri: userProfile.avatar };
    opacity = 0;
  }
  return (
    <AQLink to={targetScreen}>
      <ImageBackground
        // defeat cursed-ass caching on ios; image sticks around when source is undefined
        key={source?.uri ?? "default"}
        source={source}
        style={{
          width: 40,
          height: 40,
          borderRadius: 24,
          overflow: "hidden",
          marginRight: 10,
          backgroundColor: "black",
          justifyContent: "center",
          alignItems: "center",
        }}
      >
        <User size={24} color="white" style={{ zIndex: -2 }} />
      </ImageBackground>
    </AQLink>
  );
};

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

// TODO: merge in ^
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

export default function Router() {
  return (
    <Provider linking={linking}>
      <StreamplaceDrawer />
    </Provider>
  );
}

export function StreamplaceDrawer() {
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

  const toast = useToast();

  SystemBars.setStyle("dark");

  // Top-level stuff to handle push notification registration
  useEffect(() => {
    hydrate();
    initPushNotifications();
  }, []);
  const notificationToken = useNotificationToken();
  const userProfile = useUserProfile();
  const hydrated = useHydrated();
  useEffect(() => {
    if (notificationToken) {
      registerNotificationToken();
    }
  }, [notificationToken, userProfile]);

  // Stuff to handle incoming push notification routing
  const notificationDestination = useNotificationDestination();
  const linkTo = useLinkTo();

  const animatedDrawerStyle = useAnimatedStyle(() => {
    return {
      width: sidebar.isActive ? sidebar.animatedWidth.value : undefined,
    };
  });

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

  // are we in the live dashboard?
  const [isLiveDashboard, setIsLiveDashboard] = useState(false);
  useEffect(() => {
    if (!isLiveDashboard && userIsLive) {
      toast.show("You are live!", "Do you want to go to your Live Dashboard?", {
        actionLabel: "Go",
        onAction: () => {
          navigation.navigate("LiveDashboard");
          setLivePopup(false);
        },
        onClose: () => setLivePopup(false),
        variant: "error",
        duration: 8,
      });
    }
  }, [userIsLive]);
  const externalItems = useExternalItems();

  if (!hydrated) {
    return <View />;
  }

  return (
    <>
      <StatusBar barStyle="light-content" />
      <Drawer.Navigator
        initialRouteName="Home"
        screenOptions={{
          // for the custom sidebar
          drawerType: sidebar.isActive ? "permanent" : "front",
          swipeEnabled: !sidebar.isActive,
          drawerStyle: [
            {
              zIndex: 128000,
            },
            sidebar.isActive ? animatedDrawerStyle : [],
          ],
          // rest
          headerLeft: () => (
            <>
              {/* this is a hack to give the popup the navigator context */}
              <PopupChecker setIsLiveDashboard={setIsLiveDashboard} />
              <NavigationButton />
            </>
          ),
          headerRight: () => <AvatarButton />,
          drawerActiveTintColor: "#a0287c33",
          unmountOnBlur: true,
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
          name="Home"
          component={MainTab}
          options={{
            drawerIcon: () => <Home color={foregroundColor} size={24} />,
            drawerLabel: () => <Text variant="h5">Home</Text>,
            headerTitle: isWeb ? "Home" : "Streamplace",
            headerShown: isWeb,
            title: "Streamplace",
          }}
          listeners={{
            drawerItemPress: (e) => {
              e.preventDefault();
              navigation.dispatch(
                CommonActions.reset({
                  index: 0,
                  routes: [
                    {
                      name: "Home",
                      state: {
                        routes: [{ name: "StreamList" }],
                      },
                    },
                  ],
                }),
              );
            },
          }}
        />
        <Drawer.Screen
          name="About"
          component={AboutScreen}
          options={{
            drawerLabel: () => <Text variant="h5">What's Streamplace?</Text>,
            drawerIcon: () => (
              <ShieldQuestion color={foregroundColor} size={24} />
            ),
            drawerItemStyle: isNative ? { display: "none" } : undefined,
          }}
        />
        <Drawer.Screen
          name="Download"
          component={DownloadScreen}
          options={{
            drawerLabel: () => <Text variant="h5">Download</Text>,
            drawerIcon: () => <Download color={foregroundColor} size={24} />,
            drawerItemStyle: isBrowser ? undefined : { display: "none" },
          }}
        />
        <Drawer.Screen
          name="Settings"
          component={SettingsStack}
          options={{
            drawerIcon: () => (
              <SettingsIcon color={foregroundColor} size={24} />
            ),
            drawerLabel: () => <Text variant="h5">Settings</Text>,
            headerShown: false,
          }}
          listeners={{
            drawerItemPress: (e) => {
              e.preventDefault();
              navigation.dispatch(
                CommonActions.reset({
                  index: 0,
                  routes: [
                    {
                      name: "Settings",
                    },
                  ],
                }),
              );
            },
          }}
        />

        <Drawer.Screen
          name="Support"
          component={SupportScreen}
          options={{
            drawerLabel: () => <Text variant="h5">Support</Text>,
            drawerItemStyle: { display: "none" },
          }}
        />
        <Drawer.Screen
          name="LiveDashboard"
          component={LiveDashboard}
          options={{
            drawerLabel: () => <Text variant="h5">Live Dashboard</Text>,
            drawerIcon: () => <Video color={foregroundColor} size={24} />,
            drawerItemStyle: isNative ? { display: "none" } : undefined,
          }}
        />
        <Drawer.Screen
          name="AppReturn"
          component={AppReturnScreen}
          options={{
            drawerLabel: () => null,
            drawerItemStyle: { display: "none" },
            headerShown: false,
          }}
        />
        <Drawer.Screen
          name="Multi"
          component={MultiScreen}
          options={{
            drawerLabel: () => null,
            drawerItemStyle: { display: "none" },
          }}
        />
        <Drawer.Screen
          name="Login"
          component={Login}
          options={{
            drawerIcon: () => <LogIn color={foregroundColor} size={24} />,
            drawerLabel: () => <Text variant="h5">Login</Text>,
          }}
        />
        <Drawer.Screen
          name="PopoutChat"
          component={PopoutChat}
          options={{
            drawerLabel: () => null,
            drawerItemStyle: { display: "none" },
            headerShown: false,
            drawerStyle: { display: "none" },
          }}
        />
        <Drawer.Screen
          name="Embed"
          component={EmbedScreen}
          options={{
            drawerLabel: () => null,
            drawerItemStyle: { display: "none" },
            headerShown: false,
          }}
        />
        <Drawer.Screen
          name="InfoWidgetEmbed"
          component={InfoWidgetEmbed}
          options={{
            drawerLabel: () => null,
            drawerItemStyle: { display: "none" },
            headerShown: false,
          }}
        />
        <Drawer.Screen
          name="DanmuOBS"
          component={DanmuOBSScreen}
          options={{
            drawerLabel: () => null,
            drawerItemStyle: { display: "none" },
            headerShown: false,
          }}
        />
        <Drawer.Screen
          name="MobileGoLive"
          component={MobileGoLive}
          options={{
            headerTitle: "Go Live",
            drawerItemStyle: isNative ? undefined : { display: "none" },
            drawerLabel: () => <Text variant="h5">Go Live</Text>,
            title: "Go live",
            drawerIcon: () => <Video color={foregroundColor} size={24} />,
            headerShown: false,
          }}
        />
      </Drawer.Navigator>
      <LoginModal visible={showLoginModal} onClose={closeLoginModal} />
    </>
  );
}

export const PopupChecker = ({
  setIsLiveDashboard,
}: {
  setIsLiveDashboard: (isLiveDashboard: boolean) => void;
}) => {
  const route = useRoute();
  useEffect(() => {
    if (route.name === "LiveDashboard") {
      setIsLiveDashboard(true);
    } else {
      setIsLiveDashboard(false);
    }
  }, [route.name]);
  return <Fragment />;
};

const MainTab = () => {
  const theme = useTheme();
  const { isWeb } = usePlatform();
  return (
    <Stack.Navigator
      initialRouteName="StreamList"
      screenOptions={{
        headerLeft: ({ canGoBack }) => (
          <NavigationButton canGoBack={canGoBack} />
        ),
        headerRight: () => <AvatarButton />,
        headerShown: !isWeb,
      }}
    >
      <Stack.Screen
        name="StreamList"
        component={HomeScreen}
        options={{ headerTitle: "Streamplace", title: "Streamplace" }}
      />
      <Stack.Screen
        name="Stream"
        component={MobileStream}
        options={{
          headerTitle: "Stream",
          title: "Streamplace Stream",
          headerShown: false,
        }}
      />
    </Stack.Navigator>
  );
};

const SettingsStack = () => {
  const { isWeb } = usePlatform();
  return (
    <Stack.Navigator
      initialRouteName="MainSettings"
      screenOptions={{
        headerLeft: ({ canGoBack }) => (
          <NavigationButton canGoBack={canGoBack} />
        ),
        headerRight: () => <AvatarButton />,
      }}
    >
      <Stack.Screen
        name="MainSettings"
        component={Settings}
        options={{ headerTitle: "Settings", title: "Settings" }}
      />
      <Stack.Screen
        name="AboutCategory"
        component={AboutCategorySettings}
        options={{ headerTitle: "About", title: "About" }}
      />
      <Stack.Screen
        name="AccountCategory"
        component={AccountCategorySettings}
        options={{ headerTitle: "Account", title: "Account" }}
      />
      <Stack.Screen
        name="StreamingCategory"
        component={StreamingCategorySettings}
        options={{ headerTitle: "Streaming", title: "Streaming" }}
      />
      <Stack.Screen
        name="WebhooksSettings"
        component={WebhookManager}
        options={{ headerTitle: "Webhooks", title: "Webhooks" }}
      />
      <Stack.Screen
        name="RecommendationsSettings"
        component={RecommendationsManager}
        options={{ headerTitle: "Recommendations", title: "Recommendations" }}
      />
      <Stack.Screen
        name="PrivacyCategory"
        component={PrivacyCategorySettings}
        options={{
          headerTitle: "Privacy & Security",
          title: "Privacy & Security",
        }}
      />
      <Stack.Screen
        name="DanmuCategory"
        component={DanmuCategorySettings}
        options={{ headerTitle: "Danmu", title: "Danmu" }}
      />
      <Stack.Screen
        name="AdvancedCategory"
        component={AdvancedCategorySettings}
        options={{ headerTitle: "Advanced", title: "Advanced" }}
      />
      <Stack.Screen
        name="LanguagesCategory"
        component={LanguagesCategorySettings}
        options={{ headerTitle: "Languages", title: "Languages" }}
      />
      <Stack.Screen
        name="KeyManagement"
        component={KeyManager}
        options={{ headerTitle: "Key Manager", title: "Key Manager" }}
      />
    </Stack.Navigator>
  );
};
