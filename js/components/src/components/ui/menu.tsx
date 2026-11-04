import { forwardRef, ReactNode } from "react";
import { Platform, View, ViewInstance, ViewStyle } from "react-native";
import {
  a,
  borderRadius,
  fontSize,
  gap,
  mt,
  mx,
  p,
  pb,
  pl,
  pr,
  pt,
  px,
  py,
} from "../../lib/theme/atoms";
import { mergeStyles, useTheme } from "../../ui";
import { Text } from "./text";

export interface MenuContainerProps {
  children: ReactNode;
  style?: ViewStyle;
}

export const MenuContainer = forwardRef<ViewInstance, MenuContainerProps>(
  ({ children, style }, ref) => {
    const { theme } = useTheme();
    return (
      <View ref={ref} style={[gap.all[4], mt[4], mx[2], style]}>
        {children}
      </View>
    );
  },
);

export interface MenuGroupProps {
  children: ReactNode;
  style?: ViewStyle;
}

export const MenuGroup = forwardRef<ViewInstance, MenuGroupProps>(
  ({ children, style }, ref) => {
    const { theme } = useTheme();
    return (
      <View
        ref={ref}
        style={[
          // Solid recessed card + hairline border, matching the app's card
          // idiom (inputs, tiles, panels) rather than a muddy translucent fill.
          {
            backgroundColor: theme.colors.surface1,
            borderWidth: 1,
            borderColor: theme.colors.borderSubtle,
          },
          Platform.OS === "web" ? [px[1], py[1]] : p[1],
          gap.all[1],
          { borderRadius: borderRadius.lg },
          style,
        ]}
      >
        {children}
      </View>
    );
  },
);

export interface MenuItemProps {
  children: ReactNode;
  disabled?: boolean;
  style?: ViewStyle;
  onPress?: () => void;
}

export const MenuItem = forwardRef<ViewInstance, MenuItemProps>(
  ({ children, disabled, style }, ref) => {
    const { theme } = useTheme();

    return (
      <View
        ref={ref}
        style={[
          a.layout.flex.row,
          a.layout.flex.alignCenter,
          a.radius.all.sm,
          py[1],
          pl[3],
          pr[2],
          disabled && { opacity: 0.5 },
          style,
        ]}
      >
        {typeof children === "string" ? (
          <Text style={{ color: theme.colors.popoverForeground }}>
            {children}
          </Text>
        ) : (
          children
        )}
      </View>
    );
  },
);

export interface MenuLabelProps {
  children: ReactNode;
  style?: ViewStyle;
}

export const MenuLabel = forwardRef<ViewInstance, MenuLabelProps>(
  ({ children, style }, ref) => {
    const { theme } = useTheme();
    return (
      <Text
        ref={ref as any}
        style={mergeStyles(
          px[4],
          pt[2],
          { color: theme.colors.textMuted },
          a.fontSize.base,
          style,
        )}
      >
        {children}
      </Text>
    );
  },
);

export interface MenuSeparatorProps {
  style?: ViewStyle;
}

export const MenuSeparator = forwardRef<ViewInstance, MenuSeparatorProps>(
  ({ style }, ref) => {
    const { theme } = useTheme();
    return (
      <View
        ref={ref}
        style={[
          mx[2],
          {
            height: 1,
            backgroundColor: theme.colors.borderSubtle,
            marginVertical: 0,
          },
          style,
        ]}
      />
    );
  },
);

export interface MenuInfoProps {
  description: string;
  style?: ViewStyle;
}

export const MenuInfo = forwardRef<ViewInstance, MenuInfoProps>(
  ({ description, style }, ref) => {
    const { theme } = useTheme();
    return (
      <Text
        ref={ref as any}
        style={mergeStyles(
          { color: theme.colors.textMuted, marginTop: -8 },
          pt[1],
          pb[2],
          fontSize.sm,
          style,
        )}
      >
        {description}
      </Text>
    );
  },
);
