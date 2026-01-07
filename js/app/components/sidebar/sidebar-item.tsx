import { DrawerNavigationState, ParamListBase } from "@react-navigation/native";
import { Text, useTheme, zero } from "@streamplace/components";
import { useAQLinkHref } from "components/aqlink";
import React, { ReactNode, useState } from "react";
import {
  GestureResponderEvent,
  Pressable,
  PressableStateCallbackType,
  StyleProp,
  View,
  ViewStyle,
} from "react-native";

export default function SidebarItem({
  icon,
  label,
  collapsed,
  active,
  onPress,
  route,
  style = null,
  tint = "rgba(189, 110, 134)",
}: {
  icon:
    | React.ComponentType<any>
    | React.ReactElement
    | (() => React.ReactElement);
  label: string | ReactNode;
  collapsed: boolean;
  active: boolean;
  onPress: (event: GestureResponderEvent) => void;
  route?: DrawerNavigationState<ParamListBase>["routes"][number];
  style?:
    | StyleProp<ViewStyle>
    | ((state: PressableStateCallbackType) => StyleProp<ViewStyle>);
  tint: string;
}) {
  const [hover, setHover] = useState<boolean>(false);
  const theme = useTheme();
  const { href } = useAQLinkHref({
    screen: route?.name || "Home",
    params: route?.params as any,
  });

  // Handle different icon types - component, JSX element, or function returning JSX
  const renderIcon = () => {
    if (!icon) {
      return <Text>📄</Text>; // Default fallback
    }

    // Get theme color for icons - use theme colors if available, fallback to white
    const iconColor = theme?.theme?.colors?.foreground || "#ffffff";

    // If it's already a JSX element, clone it with theme color
    if (React.isValidElement(icon)) {
      // Clone the element and override the color prop
      return React.cloneElement(icon as any, {
        color: iconColor,
        size: 20, // Ensure consistent sizing
      });
    }

    // If it's a function (component), call it with theme color
    if (typeof icon === "function") {
      const IconComponent = icon;
      return <IconComponent color={iconColor} size={20} />;
    }
    if ((icon as any).$$typeof === Symbol.for("react.memo")) {
      const MemoizedIcon = (icon as any).type;
      return <MemoizedIcon color={iconColor} size={20} />;
    }

    // Fallback
    console.log(
      "tried to render item for route",
      label,
      href,
      ", but couldn't",
      (icon as any).$$typeof,
    );

    return <Text>📄</Text>;
  };

  return (
    <Pressable
      onPress={onPress}
      style={style}
      onHoverIn={() => setHover(true)}
      onHoverOut={() => setHover(false)}
      role="link"
      accessibilityLabel={typeof label === "string" ? label : "Link to " + href}
      // @ts-ignore This makes it render as <a> on web!
      href={route ? href : undefined}
    >
      <View
        style={[
          zero.r.md,
          zero.layout.flex.row,
          zero.layout.flex.alignCenter,
          zero.px[3],
          zero.gap.all[2],
          {
            backgroundColor:
              hover || active
                ? tint.replace(
                    ")",
                    ", " + (active && !hover ? "0.1" : "0.25") + ")",
                  )
                : undefined,
            overflow: "hidden",
          },
        ]}
      >
        <View style={[zero.w[8], zero.py[3]]}>{renderIcon()}</View>
        {!collapsed && (
          <View
            style={[
              {
                minWidth: 270,
                maxHeight: "auto",
                opacity: collapsed ? 0 : 1,
              },
            ]}
          >
            <Text>{label}</Text>
          </View>
        )}
      </View>
    </Pressable>
  );
}
