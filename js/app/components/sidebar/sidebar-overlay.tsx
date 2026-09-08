import {
  CommonActions,
  getPathFromState,
  useNavigation,
} from "@react-navigation/native";
import {
  Button,
  Text,
  useBrandingAsset,
  useDID,
  useSidebarBackgroundImage,
  useSocialShell,
  useTheme,
  useUrl,
  zero,
} from "@streamplace/components";
import { BlueskyIcon } from "@streamplace/components/src/components/icons/bluesky-icon";
import { DiscordIcon } from "@streamplace/components/src/components/icons/discord-icon";
import { colors, spacing } from "@streamplace/components/src/lib/theme/tokens";
import { decodeDataUrlText, SiteTitleLockup } from "components/brand/logo";
import { LogoBrandMenu } from "components/brand/logo-brand-menu";
import { Image } from "expo-image";
import usePlatform from "hooks/usePlatform";
import { useSidebarControl } from "hooks/useSidebarControl";
import {
  Bell,
  Book,
  Bookmark,
  Clapperboard,
  Download,
  Hash,
  Home,
  Library,
  Link as LinkIcon,
  List,
  LogIn,
  Menu,
  MessageCircle,
  Play,
  Radio,
  Search,
  Settings as SettingsIcon,
  UserCircle,
  Video,
} from "lucide-react-native";
import React, { useEffect, useMemo, useState } from "react";
import {
  Linking,
  Platform,
  Pressable,
  useWindowDimensions,
  View,
} from "react-native";
import Animated, { useAnimatedStyle } from "react-native-reanimated";
import { SvgXml } from "react-native-svg";
import {
  getStreamplaceStateFromPath,
  streamplaceLinkingOptions,
} from "src/linking-config";
import { AvatarButton } from "src/router";
import { useStore } from "store";
import { streamColumnWidthFor } from "../stream-card/widths";
import SidebarItem from "./sidebar-item";
import { EditIcon, SOCIAL_NAV_ICONS } from "./social-icons";

/**
 * Branded navigation (branding keys navLinks / navCta): a node can replace
 * the browse and creator sections with its own links, typically into the
 * wider site a single-user node belongs to. navLinks is a JSON array of
 * { label, url, icon? } with icon one of NAV_ICONS; navCta is
 * { label, url } and renders as the pill button under the list. Internal
 * paths navigate in-app; anything else opens as a link.
 */
export const NAV_ICONS: Record<string, React.ComponentType<any>> = {
  home: Home,
  search: Search,
  play: Play,
  live: Radio,
  bell: Bell,
  message: MessageCircle,
  hash: Hash,
  list: List,
  bookmark: Bookmark,
  user: UserCircle,
  settings: SettingsIcon,
  video: Video,
  book: Book,
};

export interface BrandedNavLink {
  label: string;
  url: string;
  icon?: string;
}

export function parseNavLinks(raw: string | undefined): BrandedNavLink[] {
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed
      .filter(
        (l: any) =>
          l && typeof l.label === "string" && typeof l.url === "string",
      )
      .map((l: any) => ({ label: l.label, url: l.url, icon: l.icon }));
  } catch {
    return [];
  }
}

export function parseNavCta(
  raw: string | undefined,
): { label: string; url: string } | null {
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw);
    if (
      parsed &&
      typeof parsed.label === "string" &&
      typeof parsed.url === "string"
    ) {
      return { label: parsed.label, url: parsed.url };
    }
  } catch {
    // fall through
  }
  return null;
}

/**
 * Social links (branding keys socialHeading / socialLinks / socialIcon1-4):
 * the "Say Hello?" row at the bottom of the sidebar. socialLinks is a JSON
 * array of { label, url, icon } where icon is one of SOCIAL_ICONS, one of
 * NAV_ICONS, or socialIcon1..socialIcon4 for an uploaded image. Unset shows
 * the Streamplace defaults; an empty array hides the row.
 */
export const SOCIAL_ICONS: Record<string, React.ComponentType<any>> = {
  bluesky: BlueskyIcon,
  discord: DiscordIcon,
};

export const SOCIAL_ICON_SLOTS = [
  "socialIcon1",
  "socialIcon2",
  "socialIcon3",
  "socialIcon4",
];

export const DEFAULT_SOCIAL_LINKS: BrandedNavLink[] = [
  {
    label: "Bluesky",
    url: "https://bsky.app/profile/stream.place",
    icon: "bluesky",
  },
  { label: "Discord", url: "https://discord.stream.place", icon: "discord" },
];

export function parseSocialLinks(
  raw: string | undefined,
): BrandedNavLink[] | null {
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return null;
    return parsed
      .filter(
        (l: any) =>
          l && typeof l.label === "string" && typeof l.url === "string",
      )
      .map((l: any) => ({ label: l.label, url: l.url, icon: l.icon }));
  } catch {
    return null;
  }
}

const isExternal = (url: string) => /^[a-z]+:/i.test(url);

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

// An uploaded SVG icon is tinted like the built-in ones when it is authored
// with currentColor; anything else (multi-color SVG, raster) renders as
// drawn.
function UploadedSocialIcon({
  slot,
  size,
  color,
}: {
  slot: string;
  size: number;
  color: string;
}) {
  const asset = useBrandingAsset(slot);
  const data = asset?.data;
  const mime = asset?.mimeType ?? "";
  const svg = useMemo(() => {
    if (!data || !data.startsWith("data:")) return null;
    if (!mime.includes("svg") && !data.startsWith("data:image/svg")) {
      return null;
    }
    const text = decodeDataUrlText(data);
    return text && text.includes("<svg") ? text : null;
  }, [data, mime]);
  if (svg) {
    return (
      <SvgXml
        xml={svg.replaceAll("currentColor", color)}
        width={size}
        height={size}
      />
    );
  }
  if (data) {
    return (
      <Image
        source={{ uri: data }}
        style={{ width: size, height: size }}
        contentFit="contain"
      />
    );
  }
  return <LinkIcon size={size} color={color} />;
}

function SocialIconButton({
  icon,
  label,
  href,
}: {
  icon?: string;
  label: string;
  href: string;
}) {
  const { theme } = useTheme();
  const [hover, setHover] = useState(false);
  const color = hover ? theme.colors.text1 : theme.colors.text2;
  const slot = icon && SOCIAL_ICON_SLOTS.includes(icon) ? icon : null;
  const Icon = SOCIAL_ICONS[icon ?? ""] ?? NAV_ICONS[icon ?? ""] ?? LinkIcon;
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
        {slot ? (
          <UploadedSocialIcon slot={slot} size={24} color={color} />
        ) : (
          <Icon size={24} color={color} />
        )}
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
  activeIcon?: React.ComponentType<any>;
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

  // Branded navigation replaces the browse/creator sections when present;
  // the social row is branded the same way. Hooks stay above the early
  // return below so the count is stable when the sidebar deactivates on
  // resize.
  const brandedLinks = parseNavLinks(useBrandingAsset("navLinks")?.data);
  const brandedCta = parseNavCta(useBrandingAsset("navCta")?.data);
  const socialShell = useSocialShell();
  // Unset social links mean the Streamplace defaults in the classic shell
  // and nothing in the social shell (the app it imitates has no such row).
  const socialLinks =
    parseSocialLinks(useBrandingAsset("socialLinks")?.data) ??
    (socialShell ? [] : DEFAULT_SOCIAL_LINKS);
  const socialHeading =
    useBrandingAsset("socialHeading")?.data?.trim() || "Say Hello?";
  const downloadSetting = useBrandingAsset("showDownloadLink")?.data;
  const showDownload = downloadSetting
    ? downloadSetting === "on"
    : !socialShell;
  const brandedBottomLinks = parseSocialLinks(
    useBrandingAsset("bottomLinks")?.data,
  );
  const { width: windowWidth } = useWindowDimensions();

  // Don't render if sidebar is not active (small screen) or hidden
  if (!sidebar.isActive || sidebar.isHidden) {
    return null;
  }

  // Browse destinations — public, content-first, YouTube-style
  const brandedItems: SidebarNavItem[] = brandedLinks.map((l) => ({
    icon:
      (socialShell && SOCIAL_NAV_ICONS[l.icon ?? ""]?.inactive) ||
      NAV_ICONS[l.icon ?? ""] ||
      Hash,
    activeIcon: socialShell
      ? SOCIAL_NAV_ICONS[l.icon ?? ""]?.active
      : undefined,
    label: l.label,
    href: l.url,
  }));
  const openLink = (url: string) => {
    if (isExternal(url)) {
      closeDrawer();
      void Linking.openURL(url);
    } else {
      navigate(url);
    }
  };

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
  // A branded link that already targets a destination (e.g. /settings)
  // replaces the built-in row for it rather than duplicating it.
  const brandedHrefs = new Set(brandedLinks.map((l) => l.url));
  const secondaryItems: SidebarNavItem[] = [
    { icon: LogIn, label: "Log in", href: "/login", hidden: !!did },
    {
      icon: socialShell ? SOCIAL_NAV_ICONS.settings.inactive : SettingsIcon,
      activeIcon: socialShell ? SOCIAL_NAV_ICONS.settings.active : undefined,
      label: "Settings",
      href: "/settings",
      matchPrefix: "/settings",
      hidden: brandedHrefs.has("/settings"),
    },
    {
      icon: Download,
      label: "Download",
      href: "/download",
      hidden: !isBrowser || !showDownload,
    },
  ];

  const u = new URL(streamplaceUrl);
  u.pathname = "/docs";
  // Links pinned to the bottom (branding key bottomLinks). Unset keeps the
  // classic shell's Documentation link; the social shell shows none.
  const bottomLinks: BrandedNavLink[] =
    brandedBottomLinks ??
    (socialShell
      ? []
      : [{ label: "Documentation", url: u.toString(), icon: "book" }]);

  // The social shell centers the nav + feed + chat cluster (Figma: 240 +
  // 600 + 407 in a 1440 window) instead of pinning the rail to the left
  // edge. The shell adds the same offset to its content margin.
  // Tool screens leave the column (see the shell), so the rail returns to
  // the left edge with them.
  const fullWidthRoute =
    currentScreen === "LiveDashboard" || currentScreen === "MobileGoLive";
  const socialOffset =
    socialShell && sidebar.isActive && !sidebar.overlay && !fullWidthRoute
      ? Math.max(
          0,
          Math.floor(
            (windowWidth -
              (sidebar.contentMargin + streamColumnWidthFor(windowWidth))) /
              2,
          ),
        )
      : 0;

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
          activeIcon={item.activeIcon}
          href={item.href}
          label={item.label}
          active={isItemActive(item.href, item.matchPrefix)}
          collapsed={collapsed}
          variant={socialShell ? "social" : "classic"}
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
          left: socialOffset,
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

      {socialShell ? (
        // Social shell: the viewer's avatar heads the rail (48px, opens the
        // account menu; logged out it opens login), no lockup, no toggle.
        <View
          style={{
            height: 72,
            paddingVertical: spacing[3],
            paddingLeft: collapsed ? 0 : spacing[4],
            alignItems: collapsed ? "center" : "flex-start",
            justifyContent: "center",
            marginBottom: spacing[1],
          }}
        >
          <AvatarButton size={48} bare />
        </View>
      ) : (
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
                style={[
                  zero.layout.flex.row,
                  zero.layout.flex.alignCenter,
                  { flexShrink: 1, minWidth: 0 },
                ]}
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
      )}

      {brandedItems.length > 0 ? (
        <View style={{ gap: 2 }}>
          {brandedItems.map((item) => (
            <SidebarItem
              key={item.href}
              icon={item.icon}
              activeIcon={item.activeIcon}
              href={item.href}
              label={item.label}
              active={
                !isExternal(item.href) &&
                isItemActive(item.href, item.matchPrefix)
              }
              collapsed={collapsed}
              variant={socialShell ? "social" : "classic"}
              onPress={(e) => {
                e.preventDefault();
                openLink(item.href);
              }}
            />
          ))}
          {brandedCta && !collapsed && (
            <View
              style={{
                paddingHorizontal: spacing[3],
                paddingVertical: spacing[4],
              }}
            >
              <Button
                // The social shell's pill is the brand color (Figma: accent
                // fill, dark text); the classic one is the monochrome primary.
                variant={socialShell ? "accent" : "primary"}
                width="min"
                style={{ borderRadius: 999, height: 44, paddingHorizontal: 24 }}
                onPress={() => openLink(brandedCta.url)}
              >
                {socialShell ? (
                  <View
                    style={[
                      zero.layout.flex.row,
                      zero.layout.flex.alignCenter,
                      { gap: 6 },
                    ]}
                  >
                    <EditIcon
                      size={18}
                      color={theme.colors.primaryForeground}
                    />
                    <Text
                      weight="medium"
                      style={{
                        fontSize: 15,
                        lineHeight: 20,
                        color: theme.colors.primaryForeground,
                      }}
                    >
                      {brandedCta.label}
                    </Text>
                  </View>
                ) : (
                  brandedCta.label
                )}
              </Button>
            </View>
          )}
        </View>
      ) : (
        <>
          <View style={{ gap: 2 }}>{renderItems(browseItems)}</View>
          {creatorItems.some((item) => !item.hidden) && (
            <>
              {renderSectionHeader("Creator Dashboard")}
              <View style={{ gap: 2 }}>{renderItems(creatorItems)}</View>
            </>
          )}
        </>
      )}

      {/* Hairline section divider, only when there is a section under it */}
      {secondaryItems.some((item) => !item.hidden) && (
        <>
          <View
            style={{
              height: 1,
              backgroundColor: theme.colors.borderSubtle,
              marginVertical: spacing[3],
              marginHorizontal: spacing[3],
            }}
          />
          <View style={{ gap: 2 }}>{renderItems(secondaryItems)}</View>
        </>
      )}

      {/* Bottom links and the social row pinned to the bottom */}
      {isBrowser && (
        <View style={{ marginTop: "auto", gap: 2 }}>
          {bottomLinks.map((link) => (
            <SidebarItem
              key={link.url + link.label}
              icon={
                (socialShell && SOCIAL_NAV_ICONS[link.icon ?? ""]?.inactive) ||
                NAV_ICONS[link.icon ?? ""] ||
                Book
              }
              href={link.url}
              label={link.label}
              active={false}
              collapsed={sidebar.isCollapsed}
              variant={socialShell ? "social" : "classic"}
              onPress={(e) => {
                e.preventDefault();
                openLink(link.url);
              }}
            />
          ))}
          {socialLinks.length > 0 && (
            <>
              {renderSectionHeader(socialHeading)}
              <View
                style={
                  collapsed
                    ? [
                        zero.layout.flex.column,
                        zero.layout.flex.alignCenter,
                        { gap: 2 },
                      ]
                    : [
                        zero.layout.flex.row,
                        {
                          gap: 2,
                          paddingHorizontal: spacing[3],
                          flexWrap: "wrap",
                        },
                      ]
                }
              >
                {socialLinks.map((link) => (
                  <SocialIconButton
                    key={link.url + link.label}
                    icon={link.icon}
                    label={link.label}
                    href={link.url}
                  />
                ))}
              </View>
            </>
          )}
        </View>
      )}
    </Animated.View>
  );
}
