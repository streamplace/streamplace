import {
  Link,
  NavigationProp,
  ParamListBase,
  useNavigation,
} from "@react-navigation/native";
import usePlatform from "hooks/usePlatform";
import { useEffect } from "react";
import { Pressable, StyleProp, ViewStyle } from "react-native";
import Loading from "./loading/loading";

export type LinkParams = { screen: string; params?: Record<string, string> };

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
  const navigation = useNavigation<NavigationProp<ParamListBase>>();
  const baseStyle: StyleProp<ViewStyle> = {
    display: "flex",
  };

  if (isWeb) {
    return (
      <Link style={[baseStyle, style]} to={to as any}>
        {children}
      </Link>
    );
  }

  return (
    <Pressable
      style={[baseStyle, style]}
      onPress={() => navigation.navigate(to.screen, to.params)}
    >
      {children}
    </Pressable>
  );
}

export function Redirect({ to }: { to: LinkParams }) {
  const navigation = useNavigation<NavigationProp<ParamListBase>>();
  useEffect(() => {
    console.log("redirecting to", to);
    navigation.navigate(to.screen, to.params);
  }, []);
  return <Loading />;
}
