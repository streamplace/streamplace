import { LiquidGlassView } from "@callstack/liquid-glass";
import "@expo/metro-runtime";
import {
  DrawerActions,
  LinkingOptions,
  useNavigation,
} from "@react-navigation/native";
import { createNativeStackNavigator } from "@react-navigation/native-stack";
import {
  Button,
  Text,
  useDefaultStreamer,
  useSiteTitle,
  useTheme,
  useToast,
  zero,
} from "@streamplace/components";
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
import { Provider } from "components";
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
  useWindowDimensions,
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
  Menu,
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
import {
  configureReanimatedLogger,
  ReanimatedLogLevel,
} from "react-native-reanimated";
import "src/navigation-types";
import Shell from "src/shell";
import { useStore } from "store";
import { useUserProfile } from "store/hooks";

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
  config: {
    screens: {
      StreamList: "",
      Stream: {
        path: ":user",
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
          MultistreamCategory: "settings/streaming/multistream",
          KeyManagement: "settings/streaming/key-management",
          LanguagesCategory: "settings/languages",
          BrandingAdmin: "settings/branding",
        },
      },
      KeyManagement: "key-management",
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
  const openLoginModal = useStore((state) => state.openLoginModal);
  const loginAction = useStore((state) => state.login);
  const openLoginLink = useStore((state) => state.openLoginLink);
  const { theme } = useTheme();
  let source: ImageSourcePropType | undefined = undefined;

  const windowWidth = useWindowDimensions().width;

  const isCompact = windowWidth <= 800;

  if (userProfile) {
    source = { uri: userProfile.avatar };
    return (
      <AQLink
        to={{ screen: "Settings", params: { screen: "AccountCategory" } }}
      >
        <ImageBackground
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
  }

  const handleSignup = () => {
    // TODO: remove requirement for oauth-protected-resource in oatproxy
    loginAction("https://bsky.social", openLoginLink);
  };

  if (isCompact) {
    return (
      <Button
        onPress={() => openLoginModal()}
        variant="ghost"
        size="icon"
        width="min"
        style={{ marginRight: 10, marginLeft: "auto" }}
      >
        <LogIn size={20} color={theme.colors.text} />
      </Button>
    );
  }

  return (
    <View
      style={{
        flexDirection: "row",
        alignItems: "center",
        gap: 8,
        marginRight: 10,
      }}
    >
      <Button
        onPress={() => openLoginModal()}
        variant="secondary"
        width="min"
        style={[zero.r.full]}
      >
        <Text style={{ color: theme.colors.text }}>Log In</Text>
      </Button>
      <Button
        onPress={handleSignup}
        variant="primary"
        width="min"
        style={[zero.r.full]}
      >
        <Text style={{ color: theme.colors.text }}>Sign Up</Text>
      </Button>
      <Button
        width="min"
        size="icon"
        variant="secondary"
        style={[zero.r.full]}
        onPress={() => openLoginModal()}
      >
        <User size={24} color="white" />
      </Button>
    </View>
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
