import { useNavigation } from "@react-navigation/native";
import { Text, useTheme, useUrl, zero } from "@streamplace/components";
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
      screen: "MainTabs",
      params: { screen: "HomeTab", params: { screen: "HomeMain" } },
    },
    {
      icon: () => <ShieldQuestion color={foregroundColor} size={24} />,
      label: <Text variant="h5">What's Streamplace?</Text>,
      screen: "MainTabs",
      params: { screen: "HomeTab", params: { screen: "About" } },
      hidden: isNative,
    },
    {
      icon: () => <Download color={foregroundColor} size={24} />,
      label: <Text variant="h5">Download</Text>,
      screen: "MainTabs",
      params: { screen: "HomeTab", params: { screen: "Download" } },
      hidden: !isBrowser,
    },
    {
      icon: () => <SettingsIcon color={foregroundColor} size={24} />,
      label: <Text variant="h5">Settings</Text>,
      screen: "MainTabs",
      params: { screen: "SettingsTab" },
    },
    {
      icon: () => <Video color={foregroundColor} size={24} />,
      label: <Text variant="h5">Live Dashboard</Text>,
      screen: "MainTabs",
      params: { screen: "HomeTab", params: { screen: "LiveDashboard" } },
      hidden: isNative,
    },
    {
      icon: () => <LogIn color={foregroundColor} size={24} />,
      label: <Text variant="h5">Login</Text>,
      screen: "MainTabs",
      params: { screen: "HomeTab", params: { screen: "Login" } },
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
        <Image
          source={require("../../assets/images/cube.png")}
          height={30}
          width={28}
          style={{ width: 28, height: 30, resizeMode: "contain" }}
        />
        {!sidebar.isCollapsed && <Text size="2xl">Streamplace</Text>}
      </Pressable>

      {navItems.map((item, index) => {
        if (item.hidden) return null;

        return (
          <SidebarItem
            key={index}
            icon={item.icon}
            label={item.label}
            active={false} // We'll handle active state separately if needed
            collapsed={sidebar.isCollapsed}
            onPress={(e) => {
              e.preventDefault();
              navigation.navigate(item.screen as any, item.params);
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
