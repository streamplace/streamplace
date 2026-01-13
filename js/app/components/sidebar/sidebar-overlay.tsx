import { useNavigation } from "@react-navigation/native";
import {
  Text,
  useMainLogo,
  useSidebarBackgroundImage,
  useSiteTitle,
  useTheme,
  useUrl,
  zero,
} from "@streamplace/components";
import usePlatform from "hooks/usePlatform";
import { useSidebarControl } from "hooks/useSidebarControl";
import {
  Book,
  Download,
  ExternalLink,
  Home,
  LogIn,
  Settings as SettingsIcon,
  ShieldQuestion,
  Video,
} from "lucide-react-native";
import React from "react";
import { Image, Linking, Platform, Pressable } from "react-native";
import Animated, { useAnimatedStyle } from "react-native-reanimated";
import { convertNavigationParams } from "src/navigation-helper";
import SidebarItem from "./sidebar-item";

export interface SidebarNavItem {
  icon:
    | React.ComponentType<any>
    | React.ReactElement
    | (() => React.ReactElement);
  label: React.ReactNode;
  screen: string;
  params?: any;
  hidden?: boolean;
}

export interface ExternalSidebarItem {
  icon:
    | React.ComponentType<any>
    | React.ReactElement
    | (() => React.ReactElement);
  label: React.ReactNode;
  onPress: () => void;
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

  // Don't render if sidebar is not active (small screen) or hidden
  if (!sidebar.isActive || sidebar.isHidden) {
    return null;
  }

  const animatedSidebarStyle = useAnimatedStyle(() => {
    return {
      minWidth: sidebar.animatedWidth.value,
      maxWidth: sidebar.animatedWidth.value,
    };
  });

  const foregroundColor = theme.colors.text || "#fff";

  const navItems: SidebarNavItem[] = [
    {
      icon: () => <Home color={foregroundColor} size={24} />,
      label: <Text variant="h5">Home</Text>,
      screen: "HomeMain",
    },
    {
      icon: () => <ShieldQuestion color={foregroundColor} size={24} />,
      label: <Text variant="h5">What's Streamplace?</Text>,
      screen: "About",
      hidden: isNative,
    },
    {
      icon: () => <Download color={foregroundColor} size={24} />,
      label: <Text variant="h5">Download</Text>,
      screen: "Download",
      hidden: !isBrowser,
    },
    {
      icon: () => <SettingsIcon color={foregroundColor} size={24} />,
      label: <Text variant="h5">Settings</Text>,
      screen: "MainSettings",
    },
    {
      icon: () => <Video color={foregroundColor} size={24} />,
      label: <Text variant="h5">Live Dashboard</Text>,
      screen: "LiveDashboard",
      hidden: isNative,
    },
    {
      icon: () => <LogIn color={foregroundColor} size={24} />,
      label: <Text variant="h5">Login</Text>,
      screen: "Login",
    },
  ];

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
        const u = new URL(streamplaceUrl);
        u.pathname = "/docs";
        Linking.openURL(u.toString());
      },
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
        },
      ]}
    >
      {sidebarBackgroundImageAsset?.data && (
        <Image
          source={{ uri: sidebarBackgroundImageAsset.data }}
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
            resizeMode: "contain",
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
        onPress={() =>
          navigation.navigate("MainTabs" as any, { screen: "HomeTab" })
        }
      >
        {mainLogo ? (
          <Image
            source={{ uri: mainLogo }}
            height={30}
            width={28}
            style={{ width: 28, height: 30, resizeMode: "contain" }}
          />
        ) : (
          <Image
            source={require("../../assets/images/cube.png")}
            height={30}
            width={28}
            style={{ width: 28, height: 30, resizeMode: "contain" }}
          />
        )}
        {!sidebar.isCollapsed && <Text size="2xl">{nodeName}</Text>}
      </Pressable>

      {navItems.map((item, index) => {
        if (item.hidden) return null;

        // todo: properly verify this navigation params conversion
        const converted = convertNavigationParams({
          screen: item.screen as any,
          params: item.params,
        });

        return (
          <SidebarItem
            key={index}
            icon={item.icon}
            label={item.label}
            active={false} // We'll handle active state separately if needed
            collapsed={sidebar.isCollapsed}
            route={{ screen: item.screen as any, params: item.params }}
            onPress={(e) => {
              e.preventDefault();
              navigation.navigate(converted.screen as any, converted.params);
            }}
            tint="rgba(189, 110, 134)"
          />
        );
      })}

      {externalItems.map((item, index) => (
        <SidebarItem
          key={`external-${index}`}
          icon={item.icon}
          label={item.label}
          active={false}
          collapsed={sidebar.isCollapsed}
          onPress={() => item.onPress()}
          tint="rgba(189, 110, 134)"
        />
      ))}
    </Animated.View>
  );
}
