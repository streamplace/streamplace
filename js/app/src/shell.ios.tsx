import { createBottomTabNavigator } from "@react-navigation/bottom-tabs";
import { useLinkTo, useNavigation } from "@react-navigation/native";
import { createNativeStackNavigator } from "@react-navigation/native-stack";
import { useTheme, useToast } from "@streamplace/components";
import { Settings } from "components";
import Login from "components/login/login";
import LoginModal from "components/login/login-modal";
import { AboutCategorySettings } from "components/settings/about-category-settings";
import { AccountCategorySettings } from "components/settings/account-category-settings";
import { AdvancedCategorySettings } from "components/settings/advanced-category-settings";
import { DanmuCategorySettings } from "components/settings/danmu-category-settings";
import KeyManager from "components/settings/key-manager";
import { LanguagesCategorySettings } from "components/settings/languages-category-settings";
import { PrivacyCategorySettings } from "components/settings/privacy-category-settings";
import { StreamingCategorySettings } from "components/settings/streaming-category-settings";
import WebhookManager from "components/settings/webhook-manager";
import { useBlueskyNotifications } from "hooks/useBlueskyNotifications";
import { useLiveUser } from "hooks/useLiveUser";
import { useEffect, useState } from "react";
import { StatusBar } from "react-native";
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
import { LGAvatarButton } from "./router";

const Tab = createBottomTabNavigator();
const RootStack = createNativeStackNavigator();
const HomeStack = createNativeStackNavigator();
const SettingsStack = createNativeStackNavigator();

// Home navigator with just our home screen so we get that nice blur
function HomeNavigator() {
  let theme = useTheme();
  return (
    <HomeStack.Navigator
      screenOptions={{
        headerShown: true,
        headerTransparent: true,
      }}
    >
      <HomeStack.Screen
        name="HomeMain"
        component={HomeScreen}
        options={{
          title: "Home",
          unstable_headerRightItems: ({}) => [
            {
              type: "custom",
              hidesSharedBackground: true,
              element: <LGAvatarButton />,
            },
          ],
        }}
      />
    </HomeStack.Navigator>
  );
}

// Settings stack navigator
function SettingsNavigator() {
  return (
    <SettingsStack.Navigator
      screenOptions={{
        headerShown: true,
        headerTransparent: true,
        headerBackButtonDisplayMode: "minimal",
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

// Tab navigator with three main tabs
function TabNavigator() {
  return (
    <Tab.Navigator
      backBehavior="initialRoute"
      screenOptions={{
        lazy: true,
        headerShown: false,
      }}
    >
      <Tab.Screen
        name="HomeTab"
        component={HomeNavigator}
        options={{
          title: "Home",
          tabBarIcon: {
            type: "sfSymbol",
            name: "house",
          },
        }}
      />
      <Tab.Screen
        name="GoLiveTab"
        component={LaunchGoLive}
        options={{
          title: "Go Live",
          tabBarIcon: {
            type: "sfSymbol",
            name: "video",
          },
          headerShown: true,
          headerTransparent: true,
        }}
      />
      <Tab.Screen
        name="SettingsTab"
        component={SettingsNavigator}
        options={{
          title: "Settings",
          tabBarIcon: {
            type: "sfSymbol",
            name: "gearshape",
          },
          headerShown: false,
        }}
      />
    </Tab.Navigator>
  );
}

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
      <RootStack.Navigator
        screenOptions={{
          headerShown: true,
          headerTransparent: true,
        }}
      >
        {/* Tab navigator as the initial screen */}
        <RootStack.Screen
          name="MainTabs"
          component={TabNavigator}
          options={{ headerShown: false }}
        />

        {/* Full-screen screens outside tabs */}
        <RootStack.Screen
          name="Stream"
          component={MobileStream}
          options={{ headerShown: false }}
        />
        <RootStack.Screen
          name="MobileGoLive"
          component={MobileGoLive}
          options={{ headerShown: false }}
        />

        {/* Other screens */}
        <RootStack.Screen name="Login" component={Login} />
        <RootStack.Screen name="Multi" component={MultiScreen} />
        <RootStack.Screen name="Support" component={SupportScreen} />
        <RootStack.Screen name="LiveDashboard" component={LiveDashboard} />
        <RootStack.Screen name="AppReturn" component={AppReturnScreen} />
        <RootStack.Screen name="About" component={AboutScreen} />
        <RootStack.Screen name="Download" component={DownloadScreen} />
        <RootStack.Screen name="PopoutChat" component={PopoutChat} />
        <RootStack.Screen name="Embed" component={EmbedScreen} />
        <RootStack.Screen name="InfoWidgetEmbed" component={InfoWidgetEmbed} />
        <RootStack.Screen name="DanmuOBS" component={DanmuOBSScreen} />
      </RootStack.Navigator>
      <LoginModal visible={showLoginModal} onClose={closeLoginModal} />
    </>
  );
}
