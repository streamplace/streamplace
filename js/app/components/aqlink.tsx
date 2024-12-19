import { Link, useNavigation } from "@react-navigation/native";
import { NavigationProp, ParamListBase } from "@react-navigation/native";
import usePlatform from "hooks/usePlatform";
import { Pressable, StyleProp, ViewStyle } from "react-native";

// Web and native have some disagreements about link styling
// so we have a custom component that handles that
export default function AQLink({
  children,
  to,
  style,
}: {
  children: React.ReactNode;
  to: { screen: string; params?: Record<string, string> };
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
