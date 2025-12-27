import {
  Link,
  useLinkBuilder,
  useNavigation,
  useNavigationState,
  useRoute,
} from "@react-navigation/native";
import usePlatform from "hooks/usePlatform";
import { useEffect } from "react";
import { Pressable, StyleProp, ViewStyle } from "react-native";
import { useStore } from "store";
import { convertNavigationParams } from "../src/navigation-helper";
import Loading from "./loading/loading";

export type LinkParams = { screen: string; params?: Record<string, any> };

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
  const route = useRoute();
  const openLoginModal = useStore((state) => state.openLoginModal);

  // get the deepest active route for nested navigators
  const currentRoute = useNavigationState((state) => {
    let route: any = state.routes[state.index];
    while (route.state?.index !== undefined) {
      route = route.state.routes[route.state.index];
    }
    return { name: route.name, params: route.params };
  });

  const baseStyle: StyleProp<ViewStyle> = {
    display: "flex",
  };

  const handlePress = () => {
    // intercept login navigation and show modal instead
    if (to.screen === "Login") {
      console.log(
        "AQLink intercepting login navigation, current route:",
        currentRoute,
      );
      openLoginModal(currentRoute as any);
      return;
    }
    // Convert to platform-specific navigation params
    const converted = convertNavigationParams(to);
    // @ts-expect-error - dynamic navigation with LinkParams
    navigation.navigate(converted.screen, converted.params);
  };

  if (isWeb) {
    // on web, intercept login links with onClick handler
    if (to.screen === "Login") {
      return (
        <Pressable style={[baseStyle, style]} onPress={handlePress}>
          {children}
        </Pressable>
      );
    }
    return (
      // @ts-expect-error - Link component types don't support our dynamic LinkParams
      <Link style={[baseStyle, style]} to={to}>
        {children}
      </Link>
    );
  }

  return (
    <Pressable style={[baseStyle, style]} onPress={handlePress}>
      {children}
    </Pressable>
  );
}

export function Redirect({ to }: { to: LinkParams }) {
  const navigation = useNavigation();
  useEffect(() => {
    console.log("redirecting to", to);
    const converted = convertNavigationParams(to);
    // @ts-expect-error - dynamic navigation with LinkParams
    navigation.navigate(converted.screen, converted.params);
  }, []);
  return <Loading />;
}

// generates the proper href for a given LinkParams object, for better web support
export function useAQLinkHref(to: LinkParams): { href?: string } {
  const { isWeb } = usePlatform();
  const buildLink = useLinkBuilder();

  if (!isWeb) {
    return { href: undefined };
  }

  try {
    // @ts-expect-error - buildLink expects specific types but we're using dynamic LinkParams
    const href = buildLink(to.screen, to.params);
    return { href };
  } catch (e) {
    console.warn("Failed to build link for", to, e);
    return { href: undefined };
  }
}
