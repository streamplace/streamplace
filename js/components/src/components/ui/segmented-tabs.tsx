import { useState } from "react";
import { Pressable, View, type ViewStyle } from "react-native";
import { useTheme } from "../../lib/theme/theme";
import { Text } from "./text";

// The one segmented tab control. A recessed track holding one raised segment per
// option; the active segment is a pink fill (pink = active nav/tab state per
// the design system). Use it for view switchers — content tabs, settings
// sub-nav — anywhere a hand-rolled "ButtonSelector"/"TabButton" used to live.

export interface SegmentedTabOption {
  label: string;
  value: string;
  disabled?: boolean;
}

export type SegmentedTabsSize = "sm" | "md";

// "sm" is for dense contexts — narrow side panels where a full-size tab would
// crowd its label (e.g. the live dashboard's Stream Settings panel).
const SIZES = {
  sm: { paddingHorizontal: 10, paddingVertical: 7, fontSize: 13 },
  md: { paddingHorizontal: 14, paddingVertical: 9, fontSize: 14 },
} as const;

function Segment({
  label,
  active,
  disabled,
  fullWidth,
  size,
  onPress,
}: {
  label: string;
  active: boolean;
  disabled?: boolean;
  fullWidth?: boolean;
  size: SegmentedTabsSize;
  onPress: () => void;
}) {
  const { theme } = useTheme();
  const c = theme.colors;
  const [hovered, setHovered] = useState(false);
  return (
    <Pressable
      onPress={onPress}
      disabled={disabled}
      style={fullWidth ? { flex: 1 } : undefined}
    >
      <View
        onPointerEnter={() => setHovered(true)}
        onPointerLeave={() => setHovered(false)}
        style={{
          paddingHorizontal: SIZES[size].paddingHorizontal,
          paddingVertical: SIZES[size].paddingVertical,
          borderRadius: theme.borderRadius.md,
          alignItems: "center",
          justifyContent: "center",
          opacity: disabled ? 0.4 : 1,
          backgroundColor: active
            ? c.primary
            : hovered && !disabled
              ? c.surface3
              : "transparent",
          ...(active ? theme.shadows.sm : null),
        }}
      >
        <Text
          style={{
            color: active
              ? c.primaryForeground
              : hovered && !disabled
                ? c.text1
                : c.text2,
            fontWeight: "600",
            fontSize: SIZES[size].fontSize,
          }}
        >
          {label}
        </Text>
      </View>
    </Pressable>
  );
}

export function SegmentedTabs({
  options,
  value,
  onChange,
  fullWidth = false,
  size = "md",
  style,
}: {
  options: SegmentedTabOption[];
  value: string;
  onChange: (value: string) => void;
  /** Segments stretch to fill the track (flex:1). Default: hug content. */
  fullWidth?: boolean;
  /** "sm" tightens text + padding for dense/narrow panels. Default: "md". */
  size?: SegmentedTabsSize;
  style?: ViewStyle;
}) {
  const { theme } = useTheme();
  const c = theme.colors;
  return (
    <View
      style={[
        {
          flexDirection: "row",
          gap: 2,
          padding: 4,
          borderRadius: theme.borderRadius.lg,
          backgroundColor: c.surface2,
          borderWidth: 1,
          borderColor: c.borderStrong,
          alignSelf: fullWidth ? "stretch" : "flex-start",
        },
        style,
      ]}
    >
      {options.map((opt) => (
        <Segment
          key={opt.value}
          label={opt.label}
          active={value === opt.value}
          disabled={opt.disabled}
          fullWidth={fullWidth}
          size={size}
          onPress={() => onChange(opt.value)}
        />
      ))}
    </View>
  );
}
