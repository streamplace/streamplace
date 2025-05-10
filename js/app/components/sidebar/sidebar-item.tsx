import { FileQuestion } from "@tamagui/lucide-icons";
import { Text, View, AnimatePresence } from "tamagui";
import { Pressable } from "react-native-gesture-handler";
import { ReactNode, useState } from "react";
import { PressableStateCallbackType, StyleProp, ViewStyle } from "react-native";

export default function SidebarItem({
  icon,
  label,
  collapsed,
  active,
  onPress,
  style = null,
  tint = "rgba(189, 110, 134)",
}: {
  icon: React.ComponentType<any> | React.NamedExoticComponent<any>;
  label: React.ComponentType<any> | string | ReactNode;
  collapsed: boolean;
  active: boolean;
  onPress: () => void;
  style?:
    | StyleProp<ViewStyle>
    | ((state: PressableStateCallbackType) => StyleProp<ViewStyle>);
  tint: string;
}) {
  const [hover, setHover] = useState<boolean>(false);
  // if we don't have an icon for some reason default to filequestion
  const Icon: React.NamedExoticComponent<any> = (icon as any) || FileQuestion;
  return (
    <Pressable
      onPress={onPress}
      style={style}
      onHoverIn={() => setHover(true)}
      onHoverOut={() => setHover(false)}
      role="button"
    >
      <View
        backgroundColor={
          hover || active
            ? tint.replace(
                ")",
                ", " + (active && !hover ? "0.1" : "0.25") + ")",
              )
            : undefined
        }
        borderRadius="$radius.3"
        flexDirection="row"
        justifyContent="flex-start"
        alignItems="center"
        paddingHorizontal="$3"
        gap="$2"
        overflow="hidden"
      >
        <View width="$3" paddingVertical="$3">
          <Icon color="$color" />
        </View>
        <AnimatePresence>
          {!collapsed && (
            <Text
              // setting to maximum width of the sidebar
              // so we don't get collapsing text on collapse
              minWidth={270}
              maxHeight="auto"
              fontSize="$6"
              textAlign="left"
              animation="quick"
              enterStyle={{ opacity: 100, width: "100" }}
              exitStyle={{ opacity: 0, width: "100" }}
              animateOnly={["opacity"]}
            >
              {label}
            </Text>
          )}
        </AnimatePresence>
      </View>
    </Pressable>
  );
}
