import { useCallback } from "react";
import { Pressable, Text, View, type ViewProps } from "react-native";
import { useTheme } from "../../lib/theme/theme";
import {
  fontFamilies,
  fontWeights,
  spacing,
  touchTargets,
  typeScale,
} from "../../lib/theme/tokens";

export interface TabItem {
  value: string;
  label: string;
}

export interface TabsProps extends Omit<ViewProps, "children"> {
  items: TabItem[];
  value: string;
  onValueChange: (value: string) => void;
}

/**
 * Underline tabs. Active tab: text1 + 2px accent underline; inactive:
 * text2, hover text1. Arrow keys move between tabs on web.
 */
export function Tabs({
  items,
  value,
  onValueChange,
  style,
  ...props
}: TabsProps) {
  const { theme } = useTheme();

  const moveFocus = useCallback(
    (delta: number) => {
      const idx = items.findIndex((t) => t.value === value);
      const next = items[(idx + delta + items.length) % items.length];
      if (next) onValueChange(next.value);
    },
    [items, value, onValueChange],
  );

  return (
    <View
      accessibilityRole="tablist"
      style={[
        {
          flexDirection: "row",
          gap: spacing[1],
          borderBottomWidth: 1,
          borderBottomColor: theme.colors.borderSubtle,
        },
        style,
      ]}
      {...props}
    >
      {items.map((item) => {
        const active = item.value === value;
        return (
          <Pressable
            key={item.value}
            accessibilityRole="tab"
            accessibilityState={{ selected: active }}
            onPress={() => onValueChange(item.value)}
            // Arrow-key navigation (web)
            {...({
              onKeyDown: (e: any) => {
                if (e.key === "ArrowRight") {
                  e.preventDefault();
                  moveFocus(1);
                } else if (e.key === "ArrowLeft") {
                  e.preventDefault();
                  moveFocus(-1);
                }
              },
            } as any)}
            style={({ hovered, pressed }: any) => [
              {
                minHeight: touchTargets.minimum,
                paddingHorizontal: spacing[3],
                justifyContent: "center",
                borderBottomWidth: 2,
                borderBottomColor: active
                  ? theme.colors.primary
                  : "transparent",
                marginBottom: -1,
              },
              (hovered || pressed) &&
                !active && {
                  borderBottomColor: theme.colors.borderStrong,
                },
            ]}
          >
            {({ hovered }: any) => (
              <Text
                style={{
                  fontSize: typeScale.base.fontSize,
                  lineHeight: typeScale.base.lineHeight,
                  color:
                    active || hovered ? theme.colors.text1 : theme.colors.text2,
                  fontWeight: active ? fontWeights.medium : fontWeights.regular,
                  fontFamily: active
                    ? fontFamilies.medium
                    : fontFamilies.regular,
                }}
              >
                {item.label}
              </Text>
            )}
          </Pressable>
        );
      })}
    </View>
  );
}
