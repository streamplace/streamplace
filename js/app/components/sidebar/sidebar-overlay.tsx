import {
  CommonActions,
  getPathFromState,
  useNavigation,
} from "@react-navigation/native";
import {
  Text,
  useDID,
  useMainLogo,
  useSidebarBackgroundImage,
  useSiteTitle,
  useTheme,
  useUrl,
  zero,
} from "@streamplace/components";
import { Image } from "expo-image";
import usePlatform from "hooks/usePlatform";
import { useSidebarControl } from "hooks/useSidebarControl";
import {
  Book,
  Clapperboard,
  Download,
  ExternalLink,
  Home,
  LogIn,
  Settings as SettingsIcon,
  ShieldQuestion,
  Video,
} from "lucide-react-native";
import React, { useEffect, useState } from "react";
import { Linking, Platform, Pressable } from "react-native";
import Animated, { useAnimatedStyle } from "react-native-reanimated";
import {
  getStreamplaceStateFromPath,
  streamplaceLinkingOptions,
} from "src/linking-config";
import SidebarItem from "./sidebar-item";

function getActiveTabAndScreen(state: any): {
  tab: string | undefined;
  screen: string | undefined;
} {
  if (!state) return { tab: undefined, screen: undefined };
  const mainTabsRoute = state.routes?.[state.index ?? 0];
  if (mainTabsRoute?.name !== "MainTabs" || !mainTabsRoute.state) {
    return { tab: undefined, screen: mainTabsRoute?.name };
  }
  const tabState = mainTabsRoute.state;
  const activeTab = tabState.routes?.[tabState.index ?? 0];
  if (!activeTab) return { tab: undefined, screen: undefined };
  let screen = activeTab.name;
  let nested = activeTab.state;
  while (nested) {
    const r = nested.routes?.[nested.index ?? 0];
    if (!r) break;
    screen = r.name;
    nested = r.state;
  }
  return { tab: activeTab.name, screen };
}

function getTargetTabAndScreen(href: string): {
  tab: string | undefined;
  screen: string | undefined;
} {
  const state = getStreamplaceStateFromPath(href);
  const first = (state.routes as any[])?.[state.index ?? 0];
  if (first?.name !== "MainTabs" || !first.state) {
    return { tab: undefined, screen: first?.name };
  }
  const tabState = first.state;
  const activeTab = tabState.routes?.[tabState.index ?? 0];
  if (!activeTab) return { tab: undefined, screen: undefined };
  let screen = activeTab.name;
  let nested = activeTab.state;
  while (nested) {
    const r = nested.routes?.[nested.index ?? 0];
    if (!r) break;
    screen = r.name;
    nested = r.state;
  }
  return { tab: activeTab.name, screen };
}

export interface SidebarNavItem {
  icon:
    | React.ComponentType<any>
    | React.ReactElement
    | (() => React.ReactElement);
  label: React.ReactNode;
  href: string;
  hidden?: boolean;
  matchPrefix?: string;
}

export interface ExternalSidebarItem {
  icon:
    | React.ComponentType<any>
    | React.ReactElement
    | (() => React.ReactElement);
  label: React.ReactNode;
  onPress: () => void;
  href: string;
  hidden?: boolean;
}

export function SidebarOverlay() {
  const sidebar = useSidebarControl();
  const navigation = useNavigation();
  const { theme } = useTheme();
  const { isNative, isBrowser } = usePlatform();
  const streamplaceUrl = useUrl();
  const nodeName = useSiteTitle() || "My Streamplace Station";
  const mainLogo = useMainLogo();
  const sidebarBackgroundImageAsset = useSidebarBackgroundImage();
  const did = useDID();

  const [navState, setNavState] = useState(() => navigation.getState());
  useEffect(() => {
    return navigation.addListener("state", () => {
      setNavState(navigation.getState());
    });
  }, [navigation]);
  const { tab: currentTab, screen: currentScreen } =
    getActiveTabAndScreen(navState);
  const currentPath = navState
    ? getPathFromState(navState, streamplaceLinkingOptions.config)
    : undefined;

  function isItemActive(href: string, matchPrefix?: string): boolean {
    if (matchPrefix !== undefined) {
      return currentPath?.startsWith(matchPrefix) ?? false;
    }
    const target = getTargetTabAndScreen(href);
    if (!target.tab || !currentTab) return false;
    if (target.tab !== currentTab) return false;
    return target.screen === currentScreen;
  }

  const animatedSidebarStyle = useAnimatedStyle(() => {
    return {
      minWidth: sidebar.animatedWidth.value,
      maxWidth: sidebar.animatedWidth.value,
    };
  });

  // Don't render if sidebar is not active (small screen) or hidden
  if (!sidebar.isActive || sidebar.isHidden) {
    return null;
  }

  const foregroundColor = theme.colors.text || "#fff";

  const navItems: SidebarNavItem[] = [
    {
      icon: () => <Home color={foregroundColor} size={24} />,
      label: <Text variant="h5">Home</Text>,
      href: "/",
    },
    {
      icon: () => <Clapperboard color={foregroundColor} size={24} />,
      label: <Text variant="h5">Videos</Text>,
      href: "/video",
      matchPrefix: "/video",
    },
    {
      icon: () => <ShieldQuestion color={foregroundColor} size={24} />,
      label: <Text variant="h5">What's Streamplace?</Text>,
      href: "/about",
      hidden: isNative,
    },
    {
      icon: () => <Download color={foregroundColor} size={24} />,
      label: <Text variant="h5">Download</Text>,
      href: "/download",
      hidden: !isBrowser,
    },
    {
      icon: () => <SettingsIcon color={foregroundColor} size={24} />,
      label: <Text variant="h5">Settings</Text>,
      href: "/settings",
      matchPrefix: "/settings",
    },
    {
      icon: () => <Video color={foregroundColor} size={24} />,
      label: <Text variant="h5">Live Dashboard</Text>,
      href: "/live",
      hidden: isNative,
    },
    {
      icon: () => <LogIn color={foregroundColor} size={24} />,
      label: <Text variant="h5">{did ? "Account" : "Login"}</Text>,
      href: "/login",
    },
  ];

  const u = new URL(streamplaceUrl);
  u.pathname = "/docs";

  const externalItems: ExternalSidebarItem[] = [
    {
      icon: React.memo(() => <Book size={24} color={theme.colors.text} />),
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
      ),
      onPress: () => {
        Linking.openURL(u.toString());
      },
      href: u.toString(),
      hidden: !isBrowser,
    },
  ];

  return (
    <Animated.View
      style={[
        animatedSidebarStyle,
        zero.p[2],
        zero.gap.all[2],
        zero.layout.flex.column,
        {
          position: "absolute",
          top: 0,
          left: 0,
          bottom: 0,
          zIndex: 128000,
          backgroundColor: theme.colors.background,
          borderRightColor: theme.colors.border,
          borderRightWidth: 1,
        },
      ]}
    >
      {sidebarBackgroundImageAsset?.data && (
        <Image
          source={{ uri: sidebarBackgroundImageAsset.data }}
          contentFit="contain"
          style={{
            //opacity: 0.3,
            position: "absolute",
            bottom: 0,
            left: 0,
            width: "100%",
            height: "auto",
            aspectRatio:
              sidebarBackgroundImageAsset.width &&
              sidebarBackgroundImageAsset.height
                ? sidebarBackgroundImageAsset.width /
                  sidebarBackgroundImageAsset.height
                : undefined,
          }}
        />
      )}
      <Pressable
        // @ts-ignore renders as <a> on web
        href="/"
        style={[
          zero.layout.flex.row,
          zero.layout.flex.alignCenter,
          zero.gap.all[3],
          {
            marginTop: Platform.OS === "ios" ? 29 : 8,
            marginBottom: 20,
            paddingLeft: 11,
          },
        ]}
        onPress={(e) => {
          e.preventDefault();
          navigation.navigate("MainTabs", {
            screen: "HomeTab",
            params: { screen: "HomeMain" },
          });
          return;
        }}
      >
        {mainLogo ? (
          <Image
            source={{ uri: mainLogo }}
            contentFit="contain"
            style={{ width: 28, height: 30 }}
          />
        ) : (
          <Image
            source={require("../../assets/images/cube.png")}
            contentFit="contain"
            style={{ width: 28, height: 30 }}
          />
        )}
        {!sidebar.isCollapsed && <Text size="2xl">{nodeName}</Text>}
      </Pressable>

      {navItems.map((item, index) => {
        if (item.hidden) return null;
        return (
          <SidebarItem
            key={index}
            icon={item.icon}
            href={item.href}
            label={item.label}
            active={isItemActive(item.href, item.matchPrefix)}
            collapsed={sidebar.isCollapsed}
            onPress={(e) => {
              e.preventDefault();
              const state = getStreamplaceStateFromPath(item.href);
              navigation.dispatch(
                CommonActions.reset({
                  index: 0,
                  routes: state.routes,
                }),
              );
            }}
            tint="rgba(189, 110, 134)"
          />
        );
      })}

      {externalItems.map((item, index) => {
        if (item.hidden) return null;
        return (
          <SidebarItem
            key={`external-${index}`}
            icon={item.icon}
            label={item.label}
            href={item.href}
            active={false}
            collapsed={sidebar.isCollapsed}
            onPress={(e) => {
              e.preventDefault();
              item.onPress();
            }}
            tint="rgba(189, 110, 134)"
          />
        );
      })}
    </Animated.View>
  );
}
