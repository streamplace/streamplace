import { Link, useNavigation } from "@react-navigation/native";
import { NavigationProp, ParamListBase } from "@react-navigation/native";
import usePlatform from "hooks/usePlatform";
import { Pressable } from "react-native";

// Web and native have some disagreements about link styling
// so we have a custom component that handles that
export default function AQLink({
  children,
  to,
}: {
  children: React.ReactNode;
  to: { screen: string; params?: Record<string, string> };
}) {
  const { isWeb } = usePlatform();
  const navigation = useNavigation<NavigationProp<ParamListBase>>();

  if (isWeb) {
    return <Link to={to as any}>{children}</Link>;
  }

  return (
    <Pressable onPress={() => navigation.navigate(to.screen, to.params)}>
      {children}
    </Pressable>
  );
}
