import { useNavigation } from "@react-navigation/native";
import { useDID } from "@streamplace/components";
import usePlatform from "hooks/usePlatform";
import { useEffect } from "react";
import { Pressable, StyleProp, ViewStyle } from "react-native";
import { interpolateParams, SCREEN_PATHS } from "src/linking-config";
import { useStore } from "store";
import {
  convertNavigationParams,
  ROOT_SCREENS,
} from "../src/navigation-helper";
import type {
  HomeStackParamList,
  RootStackParamList,
  SettingsStackParamList,
  TabParamList,
} from "../src/navigation-types";
import Loading from "./loading/loading";

// Union type of all screens across all navigators
type AllScreens = RootStackParamList &
  HomeStackParamList &
  SettingsStackParamList &
  TabParamList;

// Type-safe link parameters - params can be any to handle NavigatorScreenParams flexibility
export type LinkParams<T extends keyof AllScreens = keyof AllScreens> = {
  screen: T;
  params?: any;
};

// Web and native have some disagreements about link styling
// so we have a custom component that handles that
export default function AQLink({
  children,
  to,
  style,
}: {
  children: React.ReactNode;
  to: LinkParams;
  style?: StyleProp<ViewStyle>;
}) {
  const { isWeb } = usePlatform();
  const navigation = useNavigation();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const did = useDID();
  const { href } = useAQLinkHref(to);

  const getCurrentRoute = () => {
    const state = navigation.getState();
    if (!state) return { name: undefined, params: undefined };
    let route: any = state.routes[state.index];
    while (route.state?.index !== undefined) {
      route = route.state.routes[route.state.index];
    }
    return { name: route.name, params: route.params };
  };

  const baseStyle: StyleProp<ViewStyle> = {
    display: "flex",
  };

  const handlePress = (e: any) => {
    // Prevent default browser navigation on web (href is for a11y only)
    if (isWeb && e?.preventDefault) {
      e.preventDefault();
    }
    // intercept login navigation and show modal instead
    if (to.screen === "Login") {
      // if we're logged in, navigate to the settings page instead of showing the login modal
      if (did) {
        navigation.navigate("MainTabs", {
          screen: "SettingsTab",
          params: { screen: "AccountCategory" },
        });
        return;
      }
      console.log(
        "AQLink intercepting login navigation, current route:",
        getCurrentRoute(),
      );
      if (openLoginModal === null) {
        console.warn("openLoginModal is null, cannot open login modal");
        return;
      }
      openLoginModal(getCurrentRoute() as any);
      console.log(
        "AQLink login navigation intercepted, current route:",
        getCurrentRoute(),
      );
      return;
    }
    // For root-level screens, use CommonActions to navigate from root
    if (ROOT_SCREENS.includes(to.screen)) {
      const rootNav = navigation.getParent()?.getParent() || navigation;
      // @ts-expect-error - dynamic navigation
      rootNav.navigate(to.screen, to.params);
      console.log(
        "AQLink root navigation intercepted, current route:",
        getCurrentRoute(),
      );
      return;
    }
    // Convert to platform-specific navigation params for nested screens
    const converted = convertNavigationParams(to);
    // @ts-expect-error - dynamic navigation with LinkParams
    navigation.navigate(converted.screen, converted.params);
    console.log(
      "AQLink navigation intercepted, current route:",
      getCurrentRoute(),
    );
  };

  // use Pressable with href on web to render as <a> tag for copy/paste support
  return (
    <Pressable
      style={[baseStyle, style]}
      onPress={handlePress}
      // @ts-ignore - href prop makes this render as <a> on web
      href={isWeb ? href : undefined}
    >
      {children}
    </Pressable>
  );
}

export function Redirect({ to }: { to: LinkParams }) {
  const navigation = useNavigation();
  useEffect(() => {
    console.log("redirecting to", to);
    // For root-level screens, use root navigator
    if (ROOT_SCREENS.includes(to.screen)) {
      const rootNav = navigation.getParent()?.getParent() || navigation;
      // @ts-expect-error - dynamic navigation
      rootNav.navigate(to.screen, to.params);
      return;
    }
    const converted = convertNavigationParams(to);
    // @ts-expect-error - dynamic navigation with LinkParams
    navigation.navigate(converted.screen, converted.params);
  }, []);
  return <Loading />;
}

// generates the proper href for a given LinkParams object, for better web support
export function useAQLinkHref(to: LinkParams): { href?: string } {
  const { isWeb } = usePlatform();

  if (!isWeb) {
    return { href: undefined };
  }

  const screenPath = SCREEN_PATHS[to.screen as keyof typeof SCREEN_PATHS];
  if (screenPath === undefined) {
    return { href: undefined };
  }

  // add leading slash and interpolate params
  const path = screenPath === "" ? "/" : `/${screenPath}`;
  const href = interpolateParams(path, to.params);

  return { href };
}
