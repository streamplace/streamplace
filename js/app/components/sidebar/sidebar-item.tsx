import { Text, useTheme, zero } from "@streamplace/components";
import { motion, spacing } from "@streamplace/components/src/lib/theme/tokens";
import React, { ReactNode, useState } from "react";
import { GestureResponderEvent, Platform, Pressable, View } from "react-native";

/** 24px icons in a 40px row — YouTube's proportions, which give the rail a
 *  confident anchor against the 14px labels. Also the exact icon width the
 *  collapsed 64px rail leaves room for (64 - 8·2 sidebar - 12·2 row = 24). */
const ICON_SIZE = 24;

/**
 * Sidebar navigation row, Linear-style: quiet at rest (text2), surface pill
 * on hover, filled pill + text1 when active. 40px tall, 24px icons.
 */
export default function SidebarItem({
  icon,
  label,
  collapsed,
  active,
  onPress,
  href,
}: {
  icon:
    | React.ComponentType<any>
    | React.ReactElement
    | (() => React.ReactElement);
  label: string | ReactNode;
  collapsed: boolean;
  active: boolean;
  onPress: (event: GestureResponderEvent) => void;
  href: string;
}) {
  const [hover, setHover] = useState<boolean>(false);
  const { theme } = useTheme();

  const iconColor = active || hover ? theme.colors.text1 : theme.colors.text2;

  const renderIcon = () => {
    if (!icon) return null;
    if (React.isValidElement(icon)) {
      return React.cloneElement(icon as any, {
        color: iconColor,
        size: ICON_SIZE,
      });
    }
    if (typeof icon === "function") {
      const IconComponent = icon;
      return <IconComponent color={iconColor} size={ICON_SIZE} />;
    }
    if ((icon as any).$$typeof === Symbol.for("react.memo")) {
      const MemoizedIcon = (icon as any).type;
      return <MemoizedIcon color={iconColor} size={ICON_SIZE} />;
    }
    // forwardRef components (e.g. lucide icons) are objects, not functions
    if (typeof icon === "object") {
      const IconComponent = icon as any;
      return <IconComponent color={iconColor} size={ICON_SIZE} />;
    }
    return null;
  };

  const webTransition =
    Platform.OS === "web"
      ? ({
          transitionDuration: `${motion.fast}ms`,
          transitionTimingFunction: motion.easingCss,
          transitionProperty: "background-color, color",
        } as any)
      : null;

  return (
    <Pressable
      onPress={onPress}
      onHoverIn={() => setHover(true)}
      onHoverOut={() => setHover(false)}
      role="link"
      accessibilityLabel={typeof label === "string" ? label : "Link to " + href}
      // @ts-ignore This makes it render as <a> on web!
      href={href}
    >
      <View
        style={[
          zero.r.md,
          zero.layout.flex.row,
          zero.layout.flex.alignCenter,
          {
            height: 40,
            paddingHorizontal: spacing[3],
            gap: spacing[4],
            backgroundColor: active
              ? theme.colors.surface2
              : hover
                ? theme.colors.surface1
                : "transparent",
            overflow: "hidden",
          },
          webTransition,
        ]}
      >
        <View
          style={{
            width: ICON_SIZE,
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          {renderIcon()}
        </View>
        {!collapsed && (
          <View style={{ flex: 1, minWidth: 0 }}>
            {typeof label === "string" ? (
              <Text
                numberOfLines={1}
                weight={active ? "medium" : "normal"}
                style={{
                  color:
                    active || hover ? theme.colors.text1 : theme.colors.text2,
                }}
              >
                {label}
              </Text>
            ) : (
              label
            )}
          </View>
        )}
      </View>
    </Pressable>
  );
}
