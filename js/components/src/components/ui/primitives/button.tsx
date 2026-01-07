import React, { forwardRef } from "react";
import {
  AccessibilityRole,
  GestureResponderEvent,
  StyleSheet,
  Text,
  TextProps,
  TouchableOpacity,
  TouchableOpacityProps,
  View,
  ViewProps,
} from "react-native";

// Base button primitive interface
export interface ButtonPrimitiveProps
  extends Omit<TouchableOpacityProps, "onPress"> {
  onPress?: (event: GestureResponderEvent) => void;
  disabled?: boolean;
  loading?: boolean;
  accessibilityRole?: AccessibilityRole;
  accessibilityLabel?: string;
  accessibilityHint?: string;
  testID?: string;
}

// Button root primitive - handles all touch interactions
export const ButtonRoot = forwardRef<
  React.ComponentRef<typeof TouchableOpacity>,
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
      activeOpacity = 0.7,
      ...props
    },
    ref,
  ) => {
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

    return (
      <TouchableOpacity
        ref={ref}
        onPress={handlePress}
        onPressIn={handlePressIn}
        onPressOut={handlePressOut}
        onLongPress={handleLongPress}
        disabled={disabled || loading}
        activeOpacity={disabled || loading ? 1 : activeOpacity}
        accessibilityRole={accessibilityRole}
        accessibilityLabel={accessibilityLabel}
        accessibilityHint={accessibilityHint}
        accessibilityState={{
          disabled: disabled || loading,
          busy: loading,
          ...accessibilityState,
        }}
        testID={testID}
        style={[
          primitiveStyles.button,
          (disabled || loading) && primitiveStyles.disabled,
          style,
        ]}
        {...props}
      >
        {children}
      </TouchableOpacity>
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
