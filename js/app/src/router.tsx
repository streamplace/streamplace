import { LiquidGlassView } from "@callstack/liquid-glass";
import "@expo/metro-runtime";
import { useNavigation } from "@react-navigation/native";
import {
  Button,
  DropdownMenu,
  DropdownMenuItem,
  DropdownMenuTrigger,
  IconButton,
  ResponsiveDropdownMenuContent,
  Text,
  useTheme,
} from "@streamplace/components";
import { statusColors } from "@streamplace/components/src/lib/theme/tokens";
import { Provider } from "components";
import { ImageBackground } from "expo-image";
import { useLiveUser } from "hooks/useLiveUser";
import {
  CircleUser,
  LogIn,
  LogOut,
  Plus,
  Radio,
  Settings,
  SquarePlay,
  User,
} from "lucide-react-native";
import { useState } from "react";
import {
  ImageSourcePropType,
  Platform,
  useWindowDimensions,
  View,
} from "react-native";
import AQLink from "../components/aqlink";
import { convertNavigationParams, ROOT_SCREENS } from "./navigation-helper";

import Constants from "expo-constants";

import {
  configureReanimatedLogger,
  ReanimatedLogLevel,
} from "react-native-reanimated";
import "src/navigation-types";
import Shell from "src/shell";
import { useStore } from "store";
import { useUserProfile } from "store/hooks";

import {
  getStreamplaceStateFromPath,
  streamplaceLinkingOptions,
} from "./linking-config";
// Initialize sidebar state on app load
useStore.getState().loadStateFromStorage();

// On web, seed overlay mode from the initial URL so a direct load of a detail
// view (stream/video) renders full-width immediately — no docked-then-snap flash.
const DETAIL_ROUTES = ["Stream", "Video", "Vod"];
// getStateFromPath nests routes, so walk the active chain to find the leaf.
function pathHasDetailRoute(state: any): boolean {
  const routes = state?.routes;
  if (!routes?.length) return false;
  const active = routes[state.index ?? routes.length - 1];
  if (!active) return false;
  if (DETAIL_ROUTES.includes(active.name)) return true;
  return pathHasDetailRoute(active.state);
}
if (Platform.OS === "web" && typeof window !== "undefined") {
  try {
    const path =
      window.location.pathname + window.location.search + window.location.hash;
    if (pathHasDetailRoute(getStreamplaceStateFromPath(path))) {
      useStore.getState().setOverlay(true);
    }
  } catch {
    // fall back to the default (docked) — the Shell effect will correct it
  }
}

// disabled strict b/c chat swipeable triggers it a LOT and the resulting logging
// slows down the whole app
configureReanimatedLogger({
  level: ReanimatedLogLevel.warn,
  strict: false,
});

const associatedDomain = Constants.expoConfig?.ios?.associatedDomains?.[0];
if (associatedDomain && associatedDomain.startsWith("applinks:")) {
  const domain = associatedDomain.slice("applinks:".length);
  streamplaceLinkingOptions.prefixes?.push(`https://${domain}`);
}

// https://github.com/streamplace/streamplace/issues/377
const hasDevDomain = streamplaceLinkingOptions.prefixes?.some((prefix) =>
  prefix.includes("tv.aquareum.dev"),
);
if (hasDevDomain) {
  streamplaceLinkingOptions.prefixes?.push("tv.aquareum://");
  streamplaceLinkingOptions.prefixes?.push("https://stream.place");
}

console.log("Linking prefixes", streamplaceLinkingOptions.prefixes);

export default function Router() {
  return (
    <Provider linking={streamplaceLinkingOptions}>
      <Shell />
    </Provider>
  );
}

// Just a small left inset so the header title clears the sidebar. Back
// navigation is handled by the sidebar and the browser's own back button, so
// there's no in-header back arrow.
export const NavigationButton = (_props: { canGoBack?: boolean }) => {
  return <View style={{ width: 12 }} />;
};

export const LGAvatarButton = () => {
  const userProfile = useUserProfile();
  const userIsLive = useLiveUser();

  if (Platform.OS !== "ios") return <AvatarButton />;

  let source: ImageSourcePropType | undefined = undefined;
  let opacity = 1;
  // On-air, the avatar links to the live dashboard (with a red ring for the
  // state); otherwise it goes to the account screen as usual.
  const targetScreen: any = userProfile
    ? userIsLive
      ? { screen: "LiveDashboard", params: {} }
      : { screen: "AccountCategory", params: {} }
    : { screen: "Login", params: {} };

  if (userProfile) {
    source = { uri: userProfile.avatar };
    opacity = 0;
  }
  const glassAvatar = (
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
          justifyContent: "center",
          alignItems: "center",
        }}
      >
        <User size={24} color="white" style={{ zIndex: -2 }} />
      </ImageBackground>
    </LiquidGlassView>
  );
  return (
    <AQLink to={targetScreen} style={{ marginRight: 10 }}>
      {userProfile && userIsLive ? (
        <View
          style={{
            borderWidth: 2,
            borderColor: statusColors.live,
            borderRadius: 26,
            padding: 2,
          }}
        >
          {glassAvatar}
        </View>
      ) : (
        glassAvatar
      )}
    </AQLink>
  );
};

// One row of the account menu: icon + label, quiet at rest and brightening on
// hover. `danger` renders it as red ink with a red-tinted hover (log out).
// Like the Create menu, the row's fill is also the keyboard focus cue — the
// app's global 2px offset :focus-visible ring floats a rectangle around the
// row, so it's suppressed here and Radix's item focus drives the same fill.
const AccountMenuItem = ({
  icon: Icon,
  label,
  onPress,
  danger,
  iconColor,
}: {
  icon: typeof User;
  label: string;
  onPress: () => void;
  danger?: boolean;
  /** Explicit icon tint (e.g. live red for the on-air row); label keeps the row color. */
  iconColor?: string;
}) => {
  const { theme } = useTheme();
  const c = theme.colors;
  const [hovered, setHovered] = useState(false);
  const [focused, setFocused] = useState(false);
  const active = hovered || focused;
  const fg = danger ? c.danger : active ? c.text1 : c.text2;
  return (
    <DropdownMenuItem
      onPress={onPress}
      noHighlight
      onFocus={() => setFocused(true)}
      onBlur={() => setFocused(false)}
      style={{ outlineStyle: "none" } as any}
    >
      <View
        onPointerEnter={() => setHovered(true)}
        onPointerLeave={() => setHovered(false)}
        style={{
          flexDirection: "row",
          alignItems: "center",
          gap: 12,
          width: "100%",
          paddingVertical: 8,
          paddingHorizontal: 8,
          borderRadius: 8,
          backgroundColor: active
            ? danger
              ? c.dangerSoft
              : c.surface3
            : "transparent",
        }}
      >
        <Icon size={18} color={iconColor ?? fg} />
        <Text
          style={{
            color: fg,
            fontSize: 14,
            fontWeight: danger ? "600" : "500",
          }}
        >
          {label}
        </Text>
      </View>
    </DropdownMenuItem>
  );
};

export const AvatarButton = () => {
  const userProfile = useUserProfile();
  const userIsLive = useLiveUser();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const openPDSModal = useStore((state) => state.openPdsModal);
  const logout = useStore((state) => state.logout);
  const navigation = useNavigation();
  const { theme } = useTheme();
  const c = theme.colors;
  const [menuOpen, setMenuOpen] = useState(false);
  let source: ImageSourcePropType | undefined = undefined;

  const windowWidth = useWindowDimensions().width;

  const isCompact = windowWidth <= 800;

  // Mirror AQLink's navigation: root-level screens (Stream) go through the root
  // navigator; the rest are nested into their tab (Settings → SettingsTab).
  const go = (screen: string, params?: any) => () => {
    if (ROOT_SCREENS.includes(screen)) {
      const rootNav = navigation.getParent()?.getParent() || navigation;
      (rootNav as any).navigate(screen, params);
    } else {
      const converted = convertNavigationParams({
        screen: screen as any,
        params,
      });
      (navigation as any).navigate(converted.screen, converted.params);
    }
  };

  if (userProfile) {
    source = userProfile.avatar ? { uri: userProfile.avatar } : undefined;
    const handle = userProfile.handle;
    const name = userProfile.displayName?.trim() || handle;
    const channelUser = handle || userProfile.did;
    const avatar = (
      <ImageBackground
        key={source?.uri ?? "default"}
        source={source}
        style={{
          width: 32,
          height: 32,
          borderRadius: 24,
          overflow: "hidden",
          borderWidth: 1,
          borderColor: menuOpen ? c.text3 : c.borderStrong,
          backgroundColor: c.surface3,
          justifyContent: "center",
          alignItems: "center",
        }}
      >
        <User
          size={18}
          color={c.text2}
          style={{
            zIndex: -2,
            paddingHorizontal: "auto",
            paddingVertical: "auto",
          }}
        />
      </ImageBackground>
    );
    return (
      <View style={{ marginRight: 12 }}>
        <DropdownMenu onOpenChange={setMenuOpen}>
          <DropdownMenuTrigger>
            {userIsLive ? (
              // On-air: the red ring replaces the old "You are live!" toast.
              <View
                style={{
                  borderWidth: 2,
                  borderColor: statusColors.live,
                  borderRadius: 20,
                  padding: 2,
                }}
              >
                {avatar}
              </View>
            ) : (
              avatar
            )}
          </DropdownMenuTrigger>
          <ResponsiveDropdownMenuContent
            align="end"
            sideOffset={8}
            style={{ minWidth: 248 }}
          >
            {/* Identity — which account you're signed in as */}
            <View
              style={{
                flexDirection: "row",
                alignItems: "center",
                gap: 10,
                paddingHorizontal: 8,
                paddingVertical: 6,
              }}
            >
              <ImageBackground
                key={`${source?.uri ?? "default"}-hdr`}
                source={source}
                style={{
                  width: 40,
                  height: 40,
                  borderRadius: 20,
                  overflow: "hidden",
                  backgroundColor: c.surface1,
                  borderWidth: 1,
                  borderColor: c.borderSubtle,
                  justifyContent: "center",
                  alignItems: "center",
                }}
              >
                <User size={20} color={c.text3} style={{ zIndex: -2 }} />
              </ImageBackground>
              <View style={{ flex: 1, minWidth: 0 }}>
                <Text
                  numberOfLines={1}
                  style={{ color: c.text1, fontSize: 14, fontWeight: "600" }}
                >
                  {name}
                </Text>
                {handle ? (
                  <Text
                    numberOfLines={1}
                    style={{ color: c.text3, fontSize: 12.5 }}
                  >
                    @{handle}
                  </Text>
                ) : null}
              </View>
            </View>

            <View
              style={{
                height: 1,
                backgroundColor: c.borderSubtle,
                marginVertical: 6,
              }}
            />

            <View style={{ gap: 2 }}>
              {userIsLive && (
                <AccountMenuItem
                  icon={Radio}
                  label="Go to Live Dashboard"
                  iconColor={statusColors.live}
                  onPress={go("LiveDashboard")}
                />
              )}
              <AccountMenuItem
                icon={CircleUser}
                label="Your channel"
                onPress={go("Stream", { user: channelUser })}
              />
              <AccountMenuItem
                icon={Settings}
                label="Settings"
                onPress={go("MainSettings")}
              />
            </View>

            <View
              style={{
                height: 1,
                backgroundColor: c.borderSubtle,
                marginVertical: 6,
              }}
            />

            <AccountMenuItem
              icon={LogOut}
              label="Log out"
              danger
              onPress={() => logout()}
            />
          </ResponsiveDropdownMenuContent>
        </DropdownMenu>
      </View>
    );
  }

  if (isCompact) {
    return (
      <IconButton
        onPress={() => openLoginModal()}
        size="sm"
        accessibilityLabel="Log in"
        style={{ marginRight: 10, marginLeft: "auto" }}
      >
        <LogIn size={18} color={theme.colors.text2} />
      </IconButton>
    );
  }

  return (
    <View
      style={{
        flexDirection: "row",
        alignItems: "center",
        gap: 8,
        marginRight: 12,
      }}
    >
      <Button
        onPress={() => openLoginModal()}
        variant="ghost"
        size="sm"
        width="min"
      >
        Log in
      </Button>
      <Button
        onPress={() => openPDSModal()}
        variant="primary"
        size="sm"
        width="min"
      >
        Sign up
      </Button>
    </View>
  );
};

// One row of the Create menu. A menu row is a list item, not a card: one
// highlight, one level of nesting. An icon in a bordered tile made three nested
// rounded rectangles (panel → row → tile), so on hover the wash landed *behind*
// a dark bordered square and the tile read as a hole instead of the row reading
// as one selected unit. Plain icon instead, and hover/focus moves the icon and
// the background together so the whole row responds as a unit.
//
// The highlight (surface3 on the surface2 panel — the token's documented
// "hovered overlay row") is also the keyboard focus cue: the app's global 2px
// offset focus ring floats a rectangle around the row, so it's suppressed here
// and Radix's item focus drives the same fill as pointer hover.
const CreateMenuItem = ({
  icon: Icon,
  title,
  subtitle,
  onPress,
}: {
  icon: typeof Plus;
  title: string;
  subtitle: string;
  onPress: () => void;
}) => {
  const { theme } = useTheme();
  const c = theme.colors;
  const [hovered, setHovered] = useState(false);
  const [focused, setFocused] = useState(false);
  const active = hovered || focused;
  return (
    <DropdownMenuItem
      onPress={onPress}
      noHighlight
      onFocus={() => setFocused(true)}
      onBlur={() => setFocused(false)}
      style={{ outlineStyle: "none" } as any}
    >
      <View
        onPointerEnter={() => setHovered(true)}
        onPointerLeave={() => setHovered(false)}
        style={{
          flexDirection: "row",
          alignItems: "center",
          gap: 12,
          // flex:1 — NOT width:"100%". A fixed 100% width plus the negative
          // margins below just shifts the box left (it gains 8px on the left and
          // loses 8px on the right); as a flex item the negative margins are
          // absorbed into the layout so it widens evenly on both sides.
          flex: 1,
          paddingVertical: 8,
          paddingHorizontal: 10,
          borderRadius: 8,
          // DropdownMenuItem wraps children in its own pl[2]/pr[2]/py[1], which
          // stacks on the panel's p[2] and floats the highlight ~16px in from
          // the panel edge. Cancel that inner padding so the gutter is just the
          // panel's own 8px.
          marginHorizontal: -8,
          marginVertical: -4,
          backgroundColor: active ? c.surface3 : "transparent",
        }}
      >
        <Icon size={20} color={active ? c.text1 : c.text3} />
        <View style={{ gap: 1 }}>
          <Text
            style={{
              color: c.text1,
              fontSize: 14,
              lineHeight: 18,
              fontWeight: "600",
            }}
          >
            {title}
          </Text>
          <Text style={{ color: c.text3, fontSize: 12, lineHeight: 16 }}>
            {subtitle}
          </Text>
        </View>
      </View>
    </DropdownMenuItem>
  );
};

// YouTube-style "Create" CTA: a pill that opens a menu to upload a VOD or go
// live. Replaces the old single-purpose Upload button; shown wherever the shell
// header renders it (logged-in users only).
export const UploadButton = () => {
  const did = useStore((state) => state.oauthSession?.did);
  const { theme } = useTheme();
  const c = theme.colors;
  const windowWidth = useWindowDimensions().width;
  const isCompact = windowWidth <= 800;
  const navigation = useNavigation();
  // Observed only for the trigger's active styling — the menu is uncontrolled
  // so DropdownMenuItem closes the portal itself on press (no orphaned menu).
  const [menuOpen, setMenuOpen] = useState(false);

  if (!did) return null;

  const go = (screen: string) => () =>
    navigation.navigate("HomeTab" as any, { screen });

  return (
    <View style={{ marginRight: 16 }}>
      <DropdownMenu onOpenChange={setMenuOpen}>
        <DropdownMenuTrigger asChild>
          {isCompact ? (
            <IconButton accessibilityLabel="Create">
              <Plus size={22} color={c.text} strokeWidth={2.5} />
            </IconButton>
          ) : (
            <Button
              variant="secondary"
              size="md"
              width="min"
              leftIcon={<Plus size={18} strokeWidth={2.5} />}
              // Keep the trigger visibly "active" while its menu is open.
              style={menuOpen ? { backgroundColor: c.surface3 } : undefined}
            >
              Create
            </Button>
          )}
        </DropdownMenuTrigger>
        <ResponsiveDropdownMenuContent
          align="end"
          sideOffset={8}
          style={{ minWidth: 244 }}
        >
          <View style={{ gap: 2 }}>
            <CreateMenuItem
              icon={SquarePlay}
              title="Upload video"
              subtitle="Share a recorded video"
              onPress={go("Upload")}
            />
            <CreateMenuItem
              icon={Radio}
              title="Go live"
              subtitle="Start streaming now"
              onPress={go("LiveDashboard")}
            />
          </View>
        </ResponsiveDropdownMenuContent>
      </DropdownMenu>
    </View>
  );
};
