import {
  CommonActions,
  getPathFromState,
  useNavigation,
} from "@react-navigation/native";
import {
  Text,
  useDID,
  useSidebarBackgroundImage,
  useTheme,
  useUrl,
  zero,
} from "@streamplace/components";
import { BlueskyIcon } from "@streamplace/components/src/components/icons/bluesky-icon";
import { DiscordIcon } from "@streamplace/components/src/components/icons/discord-icon";
import { colors, spacing } from "@streamplace/components/src/lib/theme/tokens";
import { SiteTitleLockup } from "components/brand/logo";
import { LogoBrandMenu } from "components/brand/logo-brand-menu";
import { Image } from "expo-image";
import usePlatform from "hooks/usePlatform";
import { useSidebarControl } from "hooks/useSidebarControl";
import {
  Book,
  Clapperboard,
  Download,
  Home,
  Library,
  LogIn,
  Menu,
  Radio,
  Settings as SettingsIcon,
} from "lucide-react-native";
import React, { useEffect, useState } from "react";
import { Linking, Platform, Pressable, View } from "react-native";
import Animated, { useAnimatedStyle } from "react-native-reanimated";
import {
  getStreamplaceStateFromPath,
  streamplaceLinkingOptions,
} from "src/linking-config";
import { useStore } from "store";
import SidebarItem from "./sidebar-item";

/**
 * Sidebar toggle — a hamburger/panel button styled to line its icon up with
 * the nav item icons below it (YouTube-style), sitting left of the logo.
 */
export function SidebarToggle({
  label,
  onPress,
}: {
  label: string;
  onPress: () => void;
}) {
  const { theme } = useTheme();
  const [hover, setHover] = useState(false);
  return (
    <Pressable
      onPress={onPress}
      onHoverIn={() => setHover(true)}
      onHoverOut={() => setHover(false)}
      accessibilityLabel={label}
    >
      <View
        style={{
          height: 36,
          paddingHorizontal: spacing[3],
          borderRadius: theme.borderRadius.md,
          backgroundColor: hover ? theme.colors.surface1 : "transparent",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <View
          style={{ width: 24, alignItems: "center", justifyContent: "center" }}
        >
          <Menu
            size={24}
            color={hover ? theme.colors.text1 : theme.colors.text2}
          />
        </View>
      </View>
    </Pressable>
  );
}

function SocialIconButton({
  icon,
  label,
  href,
}: {
  icon: React.ComponentType<any>;
  label: string;
  href: string;
}) {
  const { theme } = useTheme();
  const [hover, setHover] = useState(false);
  const Icon = icon;
  return (
    <Pressable
      onPress={(e) => {
        e.preventDefault();
        Linking.openURL(href);
      }}
      onHoverIn={() => setHover(true)}
      onHoverOut={() => setHover(false)}
      accessibilityLabel={label}
      role="link"
      // @ts-ignore renders as <a> on web
      href={href}
    >
      <View
        style={[
          zero.layout.flex.center,
          {
            width: 40,
            height: 40,
            borderRadius: theme.borderRadius.md,
            backgroundColor: hover ? theme.colors.surface1 : "transparent",
          },
        ]}
      >
        <Icon
          size={24}
          color={hover ? theme.colors.text1 : theme.colors.text2}
        />
      </View>
    </Pressable>
  );
}

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
  label: string;
  href: string;
  hidden?: boolean;
  matchPrefix?: string;
}

export function SidebarOverlay() {
  const sidebar = useSidebarControl();
  const closeDrawer = useStore((state) => state.closeDrawer);
  const navigation = useNavigation();
  const { theme } = useTheme();
  // The overlay drawer is always full-width; only the docked sidebar collapses.
  const collapsed = sidebar.isCollapsed && !sidebar.overlay;
  const { isNative, isBrowser } = usePlatform();
  const streamplaceUrl = useUrl();
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
      transform: [{ translateX: sidebar.animatedTranslateX.value }],
    };
  });

  // Don't render if sidebar is not active (small screen) or hidden
  if (!sidebar.isActive || sidebar.isHidden) {
    return null;
  }

  // Browse destinations — public, content-first, YouTube-style
  const browseItems: SidebarNavItem[] = [
    { icon: Home, label: "Home", href: "/" },
    {
      icon: Clapperboard,
      label: "Videos",
      href: "/video",
      matchPrefix: "/video",
    },
  ];

  // Creator Dashboard — the creator-side counterparts to the public feed, under
  // their own labeled section.
  const creatorItems: SidebarNavItem[] = [
    {
      // Your content hub (My Videos / Livestreams / Drafts). Logged-in only.
      icon: Library,
      label: "My Videos",
      href: "/upload/videos",
      matchPrefix: "/upload",
      hidden: !did,
    },
    {
      icon: Radio,
      label: "Live streaming",
      href: "/live",
      // Creator Dashboard is logged-in only; with both items hidden the whole
      // section (header included) collapses via the `some(!hidden)` guard below.
      hidden: isNative || !did,
    },
  ];

  // You / meta destinations. Account lives under Settings, so it's dropped from
  // the sidebar — this row is just the logged-out "Log in" entry.
  const secondaryItems: SidebarNavItem[] = [
    { icon: LogIn, label: "Log in", href: "/login", hidden: !!did },
    {
      icon: SettingsIcon,
      label: "Settings",
      href: "/settings",
      matchPrefix: "/settings",
    },
    {
      icon: Download,
      label: "Download",
      href: "/download",
      hidden: !isBrowser,
    },
  ];

  const u = new URL(streamplaceUrl);
  u.pathname = "/docs";

  const navigate = (href: string) => {
    closeDrawer();
    const state = getStreamplaceStateFromPath(href);
    navigation.dispatch(
      CommonActions.reset({
        index: 0,
        routes: state.routes,
      }),
    );
  };

  // Quiet section label, Linear-style: title case, same 14px size as its items
  // (the hierarchy comes from a muted color + medium weight, not a smaller
  // size), no letter-spacing, left-rail aligned with the icons. Airy above,
  // tight hug below. Collapsed to an icon rail, it degrades to a hairline.
  const renderSectionHeader = (label: string) =>
    collapsed ? (
      <View
        style={{
          height: 1,
          backgroundColor: theme.colors.borderSubtle,
          marginVertical: spacing[2],
          marginHorizontal: spacing[3],
        }}
      />
    ) : (
      <Text
        weight="medium"
        numberOfLines={1}
        style={{
          fontSize: 14,
          lineHeight: 20,
          color: theme.colors.text3,
          paddingHorizontal: spacing[3],
          marginTop: spacing[4],
          marginBottom: spacing[2],
        }}
      >
        {label}
      </Text>
    );

  const renderItems = (items: SidebarNavItem[]) =>
    items.map((item) => {
      if (item.hidden) return null;
      return (
        <SidebarItem
          key={item.href}
          icon={item.icon}
          href={item.href}
          label={item.label}
          active={isItemActive(item.href, item.matchPrefix)}
          collapsed={collapsed}
          onPress={(e) => {
            e.preventDefault();
            navigate(item.href);
          }}
        />
      );
    });

  return (
    <Animated.View
      style={[
        animatedSidebarStyle,
        zero.layout.flex.column,
        {
          position: "absolute",
          top: 0,
          left: 0,
          bottom: 0,
          zIndex: 128000,
          paddingHorizontal: spacing[2],
          paddingBottom: spacing[3],
          backgroundColor: theme.colors.surface0,
          borderRightColor: theme.colors.borderSubtle,
          borderRightWidth: 1,
        },
      ]}
    >
      {sidebarBackgroundImageAsset?.data && (
        <Image
          source={{ uri: sidebarBackgroundImageAsset.data }}
          contentFit="contain"
          style={{
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

      {/* Brand row — toggle left of the logo, its icon aligned with the nav
          icons below (YouTube-style). */}
      <View
        style={[
          zero.layout.flex.row,
          zero.layout.flex.alignCenter,
          {
            height: 56,
            marginTop: Platform.OS === "ios" ? spacing[6] : 0,
            marginBottom: spacing[2],
            // No gap: the toggle's width equals a nav row's icon cluster, so
            // butting the logo against it lands the mark on the nav-label
            // column while the toggle icon stays aligned with the nav icons.
            gap: 0,
          },
        ]}
      >
        <SidebarToggle
          label={
            sidebar.overlay
              ? "Close menu"
              : collapsed
                ? "Expand sidebar"
                : "Collapse sidebar"
          }
          onPress={sidebar.toggle}
        />
        {!collapsed && (
          <LogoBrandMenu>
            <Pressable
              // @ts-ignore renders as <a> on web
              href="/"
              style={[zero.layout.flex.row, zero.layout.flex.alignCenter]}
              onPress={(e) => {
                e.preventDefault();
                closeDrawer();
                navigation.navigate("MainTabs", {
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
        )}
      </View>

      <View style={{ gap: 2 }}>{renderItems(browseItems)}</View>

      {creatorItems.some((item) => !item.hidden) && (
        <>
          {renderSectionHeader("Creator Dashboard")}
          <View style={{ gap: 2 }}>{renderItems(creatorItems)}</View>
        </>
      )}

      {/* Hairline section divider */}
      <View
        style={{
          height: 1,
          backgroundColor: theme.colors.borderSubtle,
          marginVertical: spacing[3],
          marginHorizontal: spacing[3],
        }}
      />

      <View style={{ gap: 2 }}>{renderItems(secondaryItems)}</View>

      {/* Docs and social pinned to the bottom */}
      {isBrowser && (
        <View style={{ marginTop: "auto", gap: 2 }}>
          <SidebarItem
            icon={Book}
            href={u.toString()}
            label="Documentation"
            active={false}
            collapsed={sidebar.isCollapsed}
            onPress={(e) => {
              e.preventDefault();
              Linking.openURL(u.toString());
            }}
          />
          {renderSectionHeader("Say Hello?")}
          {collapsed ? (
            <View
              style={[
                zero.layout.flex.column,
                zero.layout.flex.alignCenter,
                { gap: 2 },
              ]}
            >
              <SocialIconButton
                icon={BlueskyIcon}
                label="Bluesky"
                href="https://bsky.app/profile/stream.place"
              />
              <SocialIconButton
                icon={DiscordIcon}
                label="Discord"
                href="https://discord.stream.place"
              />
            </View>
          ) : (
            <View
              style={[
                zero.layout.flex.row,
                { gap: 2, paddingHorizontal: spacing[3] },
              ]}
            >
              <SocialIconButton
                icon={BlueskyIcon}
                label="Bluesky"
                href="https://bsky.app/profile/stream.place"
              />
              <SocialIconButton
                icon={DiscordIcon}
                label="Discord"
                href="https://discord.stream.place"
              />
            </View>
          )}
        </View>
      )}
    </Animated.View>
  );
}
