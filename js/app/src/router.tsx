import { LiquidGlassView } from "@callstack/liquid-glass";
import "@expo/metro-runtime";
import { useNavigation } from "@react-navigation/native";
import * as DropdownMenuPrimitive from "@rn-primitives/dropdown-menu";
import {
  Button,
  Text,
  useBetaStatus,
  useTheme,
  zero,
} from "@streamplace/components";
import { Provider } from "components";
import { ImageBackground } from "expo-image";
import { useLiveUser } from "hooks/useLiveUser";
import { useSidebarControl } from "hooks/useSidebarControl";
import {
  ArrowLeft,
  Clapperboard,
  LogIn,
  PanelLeftClose,
  PanelLeftOpen,
  Upload,
  User,
} from "lucide-react-native";
import { ReactNode } from "react";
import {
  ImageSourcePropType,
  Platform,
  Pressable,
  StyleSheet,
  useWindowDimensions,
  View,
} from "react-native";
import AQLink from "../components/aqlink";

import Constants from "expo-constants";

import {
  configureReanimatedLogger,
  ReanimatedLogLevel,
} from "react-native-reanimated";
import { convertNavigationParams } from "src/navigation-helper";
import "src/navigation-types";
import Shell from "src/shell";
import { useStore } from "store";
import { useUserProfile } from "store/hooks";

import { streamplaceLinkingOptions } from "./linking-config";
// Initialize sidebar state on app load
useStore.getState().loadStateFromStorage();

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
          marginRight: 12,
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

// Red ring shown around the avatar while the user is live
const LiveRing = ({ children }: { children: ReactNode }) => {
  const { theme } = useTheme();
  return (
    <View
      style={{
        borderWidth: 2,
        borderColor: theme.colors.destructive,
        borderRadius: 26,
        padding: 2,
      }}
    >
      {children}
    </View>
  );
};

// Anchored popover menu with a link to the live dashboard. Built on
// rn-primitives directly so it stays a regular dropdown on native too,
// where the shared DropdownMenuContent renders as a bottom sheet.
const LiveAvatarDropdown = ({ children }: { children: ReactNode }) => {
  const { theme } = useTheme();
  const navigation = useNavigation();
  return (
    <DropdownMenuPrimitive.Root>
      <DropdownMenuPrimitive.Trigger>{children}</DropdownMenuPrimitive.Trigger>
      <DropdownMenuPrimitive.Portal>
        <DropdownMenuPrimitive.Overlay
          style={Platform.OS !== "web" ? StyleSheet.absoluteFill : undefined}
        >
          <DropdownMenuPrimitive.Content
            align="end"
            sideOffset={8}
            style={{
              zIndex: 50,
              minWidth: 160,
              borderRadius: 8,
              borderWidth: 1,
              borderColor: theme.colors.border,
              backgroundColor: theme.colors.popover,
              padding: 4,
            }}
          >
            <DropdownMenuPrimitive.Item
              onPress={() => {
                const to = convertNavigationParams({
                  screen: "LiveDashboard",
                });
                // @ts-expect-error - dynamic navigation with LinkParams
                navigation.navigate(to.screen, to.params);
              }}
            >
              <View
                style={{
                  flexDirection: "row",
                  alignItems: "center",
                  gap: 8,
                  paddingVertical: 6,
                  paddingHorizontal: 8,
                }}
              >
                <View
                  style={{
                    width: 8,
                    height: 8,
                    borderRadius: 4,
                    backgroundColor: theme.colors.destructive,
                  }}
                />
                <Text style={{ color: theme.colors.popoverForeground }}>
                  Go to Live Dashboard
                </Text>
              </View>
            </DropdownMenuPrimitive.Item>
          </DropdownMenuPrimitive.Content>
        </DropdownMenuPrimitive.Overlay>
      </DropdownMenuPrimitive.Portal>
    </DropdownMenuPrimitive.Root>
  );
};

export const LGAvatarButton = () => {
  const userProfile = useUserProfile();
  const userIsLive = useLiveUser();

  if (Platform.OS !== "ios") return <AvatarButton />;

  let source: ImageSourcePropType | undefined = undefined;
  let opacity = 1;
  const targetScreen: any = userProfile
    ? { screen: "AccountCategory", params: {} }
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
        }}
      >
        <User size={24} color="white" style={{ zIndex: -2 }} />
      </ImageBackground>
    </LiquidGlassView>
  );

  if (userProfile && userIsLive) {
    return (
      <View style={{ marginRight: 10 }}>
        <LiveAvatarDropdown>
          <LiveRing>{glassAvatar}</LiveRing>
        </LiveAvatarDropdown>
      </View>
    );
  }

  return (
    <AQLink to={targetScreen} style={{ marginRight: 10 }}>
      {glassAvatar}
    </AQLink>
  );
};

export const AvatarButton = () => {
  const userProfile = useUserProfile();
  const userIsLive = useLiveUser();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const openPDSModal = useStore((state) => state.openPdsModal);
  const { theme } = useTheme();
  let source: ImageSourcePropType | undefined = undefined;

  const windowWidth = useWindowDimensions().width;

  const isCompact = windowWidth <= 800;

  if (userProfile) {
    source = { uri: userProfile.avatar };
    const avatar = (
      <ImageBackground
        key={source?.uri ?? "default"}
        source={source}
        style={{
          width: 40,
          height: 40,
          borderRadius: 24,
          overflow: "hidden",
          backgroundColor: "black",
          justifyContent: "center",
          alignItems: "center",
        }}
      >
        <User size={24} color="white" style={{ zIndex: -2 }} />
      </ImageBackground>
    );
    if (userIsLive) {
      return (
        <View style={{ marginRight: 10 }}>
          <LiveAvatarDropdown>
            <LiveRing>{avatar}</LiveRing>
          </LiveAvatarDropdown>
        </View>
      );
    }
    return (
      <AQLink to={{ screen: "AccountCategory" }} style={{ marginRight: 10 }}>
        {avatar}
      </AQLink>
    );
  }

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
        onPress={() => openPDSModal()}
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

export const UploadButton = () => {
  const did = useStore((state) => state.oauthSession?.did);
  const { theme } = useTheme();
  const windowWidth = useWindowDimensions().width;
  const isCompact = windowWidth <= 800;

  const { status: betaStatus, loading: betaLoading } = useBetaStatus("vod");
  const navigation = useNavigation();

  if (!did) return null;
  if (betaLoading) return null;
  if (betaStatus === "none") {
    if (isCompact) {
      return (
        <AQLink
          to={{ screen: "HomeTab", params: { screen: "Upload" } }}
          style={{ marginRight: 10, padding: 8 }}
        >
          <Upload size={20} color={theme.colors.text} />
        </AQLink>
      );
    }
    return (
      <AQLink
        to={{ screen: "HomeTab", params: { screen: "Upload" } }}
        style={{ marginRight: 10 }}
      >
        <Button
          variant="secondary"
          style={[zero.r.full]}
          onPress={() =>
            navigation.navigate("HomeTab" as any, { screen: "Upload" })
          }
        >
          <Clapperboard size={16} color={theme.colors.textMuted} />
          <Text style={{ color: theme.colors.textMuted }}>VOD Beta</Text>
        </Button>
      </AQLink>
    );
  } else if (betaStatus === "granted") {
    if (isCompact) {
      return (
        <AQLink
          to={{ screen: "HomeTab", params: { screen: "Upload" } }}
          style={{ marginRight: 10, padding: 8 }}
        >
          <Clapperboard size={20} color={theme.colors.text} />
        </AQLink>
      );
    }

    return (
      <AQLink
        to={{ screen: "HomeTab", params: { screen: "Upload" } }}
        style={{ marginRight: 10 }}
      >
        <Button
          variant="secondary"
          style={[zero.r.full]}
          onPress={() =>
            navigation.navigate("HomeTab" as any, { screen: "Upload" })
          }
        >
          <Upload size={16} color={theme.colors.textMuted} />
          <Text style={{ color: theme.colors.textMuted }}>Upload</Text>
        </Button>
      </AQLink>
    );
  } else if (betaStatus === "requested") {
    return null;
  }
};
