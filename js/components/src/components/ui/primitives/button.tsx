import React, { forwardRef, useState } from "react";
import {
  AccessibilityRole,
  GestureResponderEvent,
  Platform,
  Pressable,
  PressableProps,
  StyleProp,
  StyleSheet,
  Text,
  TextProps,
  View,
  ViewProps,
  ViewStyle,
} from "react-native";
import { motion } from "../../../lib/theme/tokens";

// Base button primitive interface
export interface ButtonPrimitiveProps extends Omit<PressableProps, "onPress"> {
  onPress?: (event: GestureResponderEvent) => void;
  disabled?: boolean;
  loading?: boolean;
  accessibilityRole?: AccessibilityRole;
  accessibilityLabel?: string;
  accessibilityHint?: string;
  testID?: string;
  hoverStyle?: StyleProp<ViewStyle>;
  pressedStyle?: StyleProp<ViewStyle>;
}

// Button root primitive - handles all touch interactions
export const ButtonRoot = forwardRef<
  React.ComponentRef<typeof Pressable>,
  ButtonPrimitiveProps
>(
  (
    {
      children,
      disabled = false,
      loading = false,
      onPress,
      onPressIn,
      onPressOut,
      onLongPress,
      accessibilityRole = "button",
      accessibilityLabel,
      accessibilityHint,
      accessibilityState,
      testID,
      style,
      hoverStyle,
      pressedStyle,
      ...props
    },
    ref,
  ) => {
    const [isHovered, setIsHovered] = useState(false);

    const handlePress = React.useCallback(
      (event: GestureResponderEvent) => {
        if (!disabled && !loading && onPress) {
          onPress(event);
        }
      },
      [disabled, loading, onPress],
    );

    const handlePressIn = React.useCallback(
      (event: GestureResponderEvent) => {
        if (!disabled && !loading && onPressIn) {
          onPressIn(event);
        }
      },
      [disabled, loading, onPressIn],
    );

    const handlePressOut = React.useCallback(
      (event: GestureResponderEvent) => {
        if (!disabled && !loading && onPressOut) {
          onPressOut(event);
        }
      },
      [disabled, loading, onPressOut],
    );

    const handleLongPress = React.useCallback(
      (event: GestureResponderEvent) => {
        if (!disabled && !loading && onLongPress) {
          onLongPress(event);
        }
      },
      [disabled, loading, onLongPress],
    );

    const handleHoverIn = React.useCallback(() => {
      if (!disabled && !loading) {
        setIsHovered(true);
      }
    }, [disabled, loading]);

    const handleHoverOut = React.useCallback(() => {
      setIsHovered(false);
    }, []);

    return (
      <Pressable
        ref={ref}
        onPress={handlePress}
        onPressIn={handlePressIn}
        onPressOut={handlePressOut}
        onLongPress={handleLongPress}
        onHoverIn={handleHoverIn}
        onHoverOut={handleHoverOut}
        disabled={disabled || loading}
        accessibilityRole={accessibilityRole}
        accessibilityLabel={accessibilityLabel}
        accessibilityHint={accessibilityHint}
        accessibilityState={{
          disabled: disabled || loading,
          busy: loading,
          ...accessibilityState,
        }}
        testID={testID}
        style={({ pressed }) => [
          primitiveStyles.button,
          primitiveStyles.transition,
          (disabled || loading) && primitiveStyles.disabled,
          style as any,
          isHovered && hoverStyle,
          pressed && !disabled && !loading && pressedStyle,
        ]}
        {...props}
      >
        {children}
      </Pressable>
    );
  },
);

ButtonRoot.displayName = "ButtonRoot";

// Button text primitive
export interface ButtonTextProps extends TextProps {
  disabled?: boolean;
  loading?: boolean;
}

export const ButtonText = forwardRef<Text, ButtonTextProps>(
  ({ children, disabled, loading, style, ...props }, ref) => {
    return (
      <Text
        ref={ref}
        style={[
          primitiveStyles.text,
          (disabled || loading) && primitiveStyles.textDisabled,
          style,
        ]}
        {...props}
      >
        {children}
      </Text>
    );
  },
);

ButtonText.displayName = "ButtonText";

// Button icon primitive
export interface ButtonIconProps extends ViewProps {
  position?: "left" | "right";
  disabled?: boolean;
  loading?: boolean;
}

export const ButtonIcon = forwardRef<View, ButtonIconProps>(
  (
    { children, position = "left", disabled, loading, style, ...props },
    ref,
  ) => {
    return (
      <View
        ref={ref}
        style={[
          primitiveStyles.icon,
          (disabled || loading) && primitiveStyles.iconDisabled,
          style,
        ]}
        {...props}
      >
        {children}
      </View>
    );
  },
);

ButtonIcon.displayName = "ButtonIcon";

// Button loading indicator primitive
export interface ButtonLoadingProps extends ViewProps {
  visible?: boolean;
}

export const ButtonLoading = forwardRef<View, ButtonLoadingProps>(
  ({ children, visible = false, style, ...props }, ref) => {
    if (!visible) return null;

    return (
      <View ref={ref} style={[primitiveStyles.loading, style]} {...props}>
        {children}
      </View>
    );
  },
);

ButtonLoading.displayName = "ButtonLoading";

// Container for button content with flex layout
export interface ButtonContentProps extends ViewProps {
  direction?: "row" | "column";
  align?: "flex-start" | "center" | "flex-end";
  justify?:
    | "flex-start"
    | "center"
    | "flex-end"
    | "space-between"
    | "space-around";
}

export const ButtonContent = forwardRef<View, ButtonContentProps>(
  (
    {
      children,
      direction = "row",
      align = "center",
      justify = "center",
      style,
      ...props
    },
    ref,
  ) => {
    return (
      <View
        ref={ref}
        style={[
          primitiveStyles.content,
          {
            flexDirection: direction,
            alignItems: align,
            justifyContent: justify,
          },
          style,
        ]}
        {...props}
      >
        {children}
      </View>
    );
  },
);

ButtonContent.displayName = "ButtonContent";

// Primitive styles (minimal, unstyled)
const primitiveStyles = StyleSheet.create({
  button: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
  },
  transition:
    Platform.OS === "web"
      ? // web-only micro-interaction timing from the motion tokens
        ({
          transitionDuration: `${motion.fast}ms`,
          transitionTimingFunction: motion.easingCss,
          transitionProperty:
            "background-color, border-color, color, opacity, transform",
        } as any)
      : undefined,
  disabled: {
    opacity: 0.5,
  },
  content: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
  },
  text: {
    textAlign: "center" as const,
  },
  textDisabled: {
    opacity: 0.5,
  },
  icon: {
    alignItems: "center",
    justifyContent: "center",
  },
  iconDisabled: {
    opacity: 0.5,
  },
  loading: {
    position: "absolute",
    alignItems: "center",
    justifyContent: "center",
  },
});

// Export primitive collection
export const ButtonPrimitive = {
  Root: ButtonRoot,
  Text: ButtonText,
  Icon: ButtonIcon,
  Loading: ButtonLoading,
  Content: ButtonContent,
};
