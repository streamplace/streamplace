import { LiquidGlassView } from "@callstack/liquid-glass";
import "@expo/metro-runtime";
import { useNavigation } from "@react-navigation/native";
import { Button, Text, useTheme, zero } from "@streamplace/components";
import { Provider } from "components";
import { ImageBackground } from "expo-image";
import { useSidebarControl } from "hooks/useSidebarControl";
import {
  ArrowLeft,
  LogIn,
  PanelLeftClose,
  PanelLeftOpen,
  Upload,
  User,
} from "lucide-react-native";
import {
  ImageSourcePropType,
  Platform,
  Pressable,
  useWindowDimensions,
  View,
} from "react-native";
import AQLink from "../components/aqlink";

import Constants from "expo-constants";

import {
  configureReanimatedLogger,
  ReanimatedLogLevel,
} from "react-native-reanimated";
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

export const LGAvatarButton = () => {
  const userProfile = useUserProfile();

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
  return (
    <AQLink to={targetScreen} style={{ marginRight: 10 }}>
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
  const openLoginModal = useStore((state) => state.openLoginModal);
  const openPDSModal = useStore((state) => state.openPdsModal);
  const { theme } = useTheme();
  let source: ImageSourcePropType | undefined = undefined;

  const windowWidth = useWindowDimensions().width;

  const isCompact = windowWidth <= 800;

  if (userProfile) {
    source = { uri: userProfile.avatar };
    return (
      <AQLink
        to={{ screen: "SettingsTab", params: { screen: "AccountCategory" } }}
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

  if (!did) return null;

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
      <Button variant="secondary" style={[zero.r.full]}>
        <Upload size={16} color={theme.colors.textMuted} />
        <Text style={{ color: theme.colors.textMuted }}>Upload</Text>
      </Button>
    </AQLink>
  );
};
