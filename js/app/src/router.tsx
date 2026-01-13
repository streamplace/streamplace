import { LiquidGlassView } from "@callstack/liquid-glass";
import "@expo/metro-runtime";
import { getStateFromPath } from "@react-navigation/core";
import { LinkingOptions, useNavigation } from "@react-navigation/native";
import { createNativeStackNavigator } from "@react-navigation/native-stack";
import {
  Text,
  useDefaultStreamer,
  useSiteTitle,
  useTheme,
  useToast,
} from "@streamplace/components";
import { Provider, Settings } from "components";
import AQLink from "components/aqlink";
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
import Constants from "expo-constants";
import { useSidebarControl } from "hooks/useSidebarControl";
import {
  ArrowLeft,
  PanelLeftClose,
  PanelLeftOpen,
  User,
} from "lucide-react-native";
import {
  ImageBackground,
  ImageSourcePropType,
  Platform,
  Pressable,
  View,
} from "react-native";

import Constants from "expo-constants";
import {
  configureReanimatedLogger,
  ReanimatedLogLevel,
} from "react-native-reanimated";
import "src/navigation-types";
import Shell from "src/shell";
import { useStore } from "store";
import { useUserProfile } from "store/hooks";
import { SCREEN_PATHS } from "./linking-config";

// Initialize sidebar state on app load
useStore.getState().loadStateFromStorage();

// disabled strict b/c chat swipeable triggers it a LOT and the resulting logging
// slows down the whole app
configureReanimatedLogger({
  level: ReanimatedLogLevel.warn,
  strict: false,
});

const linking: LinkingOptions<ReactNavigation.RootParamList> = {
  prefixes: [ExpoLinking.createURL("")],
  getStateFromPath: (path, config) => {
    // Use default parsing
    const state = getStateFromPath(path, config);

    if (state) {
      // Check if we're navigating to a settings detail screen
      const routes = state.routes;
      const mainTabsRoute = routes.find((r: any) => r.name === "MainTabs");

      if (mainTabsRoute?.state) {
        const settingsTabRoute = mainTabsRoute.state.routes?.find(
          (r: any) => r.name === "SettingsTab",
        );

        // if we're going to a settings detail screen, but MainSettings is not in the stack,
        // we need to insert it at the bottom of the stack so we can escape to MainSettings
        if (settingsTabRoute?.state) {
          const settingsStack = settingsTabRoute.state.routes;
          const firstRoute = settingsStack?.[0];
          if (
            firstRoute?.name !== "MainSettings" &&
            settingsStack?.length === 1
          ) {
            return {
              ...state,
              routes: state.routes.map((r: any) => {
                if (r.name === "MainTabs" && r.state) {
                  return {
                    ...r,
                    state: {
                      ...r.state,
                      routes: r.state.routes.map((tabRoute: any) => {
                        if (tabRoute.name === "SettingsTab" && tabRoute.state) {
                          return {
                            ...tabRoute,
                            state: {
                              ...tabRoute.state,
                              routes: [
                                { name: "MainSettings" },
                                ...tabRoute.state.routes,
                              ],
                            },
                          };
                        }
                        return tabRoute;
                      }),
                    },
                  };
                }
                return r;
              }),
            };
          }
        }
      }
    }

    return state;
  },
  config: {
    screens: {
      // Main tabs (used on all platforms, tab bar hidden on web)
      MainTabs: {
        screens: {
          HomeTab: {
            screens: {
              HomeMain: SCREEN_PATHS.HomeMain,
              About: SCREEN_PATHS.About,
              Download: SCREEN_PATHS.Download,
              LiveDashboard: SCREEN_PATHS.LiveDashboard,
              Login: SCREEN_PATHS.Login,
              Multi: SCREEN_PATHS.Multi,
              Support: SCREEN_PATHS.Support,
            },
          },
          GoLiveTab: SCREEN_PATHS.GoLiveTab,
          SettingsTab: {
            screens: {
              Settings: {
                screens: {
                  MainSettings: SCREEN_PATHS.MainSettings,
                  AboutCategory: SCREEN_PATHS.AboutCategory,
                  AccountCategory: SCREEN_PATHS.AccountCategory,
                  StreamingCategory: SCREEN_PATHS.StreamingCategory,
                  WebhooksSettings: SCREEN_PATHS.WebhooksSettings,
                  RecommendationsSettings: SCREEN_PATHS.RecommendationsSettings,
                  PrivacyCategory: SCREEN_PATHS.PrivacyCategory,
                  DanmuCategory: SCREEN_PATHS.DanmuCategory,
                  AdvancedCategory: SCREEN_PATHS.AdvancedCategory,
                  DeveloperSettings: SCREEN_PATHS.DeveloperSettings,
                  MultistreamCategory: SCREEN_PATHS.MultistreamCategory,
                  KeyManagement: SCREEN_PATHS.KeyManagement,
                  LanguagesCategory: SCREEN_PATHS.LanguagesCategory,
                  BrandingAdmin: SCREEN_PATHS.BrandingAdmin,
                },
              },
              KeyManagement: "key-management",
            },
          },
        },
      },
      // Root stack screens (outside tabs - full-screen experiences)
      Stream: {
        path: SCREEN_PATHS.Stream,
      },
      MobileGoLive: SCREEN_PATHS.MobileGoLive,
      AVSync: SCREEN_PATHS.AVSync,
      AppReturn: SCREEN_PATHS.AppReturn,
      PopoutChat: SCREEN_PATHS.PopoutChat,
      Embed: SCREEN_PATHS.Embed,
      InfoWidgetEmbed: SCREEN_PATHS.InfoWidgetEmbed,
      LegacyStream: SCREEN_PATHS.LegacyStream,
      DanmuOBS: SCREEN_PATHS.DanmuOBS,
    },
  },
};

const associatedDomain = Constants.expoConfig?.ios?.associatedDomains?.[0];
if (associatedDomain && associatedDomain.startsWith("applinks:")) {
  const domain = associatedDomain.slice("applinks:".length);
  linking.prefixes?.push(`https://${domain}`);
}

// https://github.com/streamplace/streamplace/issues/377
const hasDevDomain = linking.prefixes?.some((prefix) =>
  prefix.includes("tv.aquareum.dev"),
);
if (hasDevDomain) {
  linking.prefixes?.push("tv.aquareum://");
  linking.prefixes?.push("https://stream.place");
}

console.log("Linking prefixes", linking.prefixes);

export default function Router() {
  return (
    <Provider linking={linking}>
      <Shell />
    </Provider>
  );
}

export const NavigationButton = ({ canGoBack }: { canGoBack?: boolean }) => {
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
        canGoBack && (
          <Pressable style={{ padding: 5 }} onPress={handleGoBackPress}>
            <ArrowLeft size={24} color={theme.colors.accentForeground} />
          </Pressable>
        )
      )}
    </View>
  );
};

export const LGAvatarButton = () => {
  const userProfile = useUserProfile();
  let source: ImageSourcePropType | undefined = undefined;
  let opacity = 1;
  const targetScreen: any = userProfile
    ? { screen: "AccountCategory", params: {} }
    : { screen: "Login", params: {} };

  if (userProfile) {
    source = { uri: userProfile.avatar };
    opacity = 0;
  }
  return (
    <AQLink to={targetScreen}>
      <LiquidGlassView
        interactive
        style={{
          borderRadius: 24,
          width: 40,
          height: 40,
          justifyContent: "center",
          alignItems: "center",
        }}
      >
        <ImageBackground
          // defeat cursed-ass caching on ios; image sticks around when source is undefined
          key={source?.uri ?? "default"}
          source={source}
          style={{
            width: 38,
            height: 38,
            borderRadius: 24,
            overflow: "hidden",
            backgroundColor: "black",
            opacity: 0.9,
          }}
        >
          <User size={24} color="white" style={{ zIndex: -2 }} />
        </ImageBackground>
      </LiquidGlassView>
    </AQLink>
  );
};

export const AvatarButton = () => {
  const userProfile = useUserProfile();
  let source: ImageSourcePropType | undefined = undefined;
  let opacity = 1;
  const targetScreen: any = userProfile
    ? { screen: "AccountCategory", params: {} }
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
