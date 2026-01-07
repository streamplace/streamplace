import { Check } from "lucide-react-native";
import { forwardRef } from "react";
import { StyleSheet, TouchableOpacity, View } from "react-native";
import { Theme, useTheme } from "../../lib/theme/theme";
import { Text } from "./text";

export interface CheckboxProps {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  disabled?: boolean;
  size?: "sm" | "md" | "lg";
  label?: string;
  description?: string;
  style?: any;
}

export const Checkbox = forwardRef<any, CheckboxProps>(
  (
    {
      checked,
      onCheckedChange,
      disabled = false,
      size = "md",
      label,
      description,
      style,
      ...props
    },
    ref,
  ) => {
    const { theme } = useTheme();

    const handlePress = () => {
      if (!disabled) {
        onCheckedChange(!checked);
      }
    };

    const styles = createStyles(theme, size, disabled, checked);

    return (
      <TouchableOpacity
        ref={ref}
        style={[styles.container, style]}
        onPress={handlePress}
        disabled={disabled}
        {...props}
      >
        <View style={styles.checkbox}>
          {checked && (
            <Check
              size={size === "sm" ? 12 : size === "lg" ? 18 : 14}
              color={
                disabled
                  ? theme.colors.textDisabled
                  : theme.colors.primaryForeground
              }
            />
          )}
        </View>
        {(label || description) && (
          <View style={styles.content}>
            {label && (
              <Text
                size={size === "sm" ? "sm" : size === "lg" ? "lg" : "base"}
                color={disabled ? "muted" : "default"}
                leading="snug"
              >
                {label}
              </Text>
            )}
            {description && (
              <Text
                size={size === "sm" ? "xs" : size === "lg" ? "base" : "sm"}
                color={disabled ? "muted" : "muted"}
                style={{ marginTop: theme.spacing[1] }}
              >
                {description}
              </Text>
            )}
          </View>
        )}
      </TouchableOpacity>
    );
  },
);

Checkbox.displayName = "Checkbox";

function createStyles(
  theme: Theme,
  size: string,
  disabled: boolean,
  checked: boolean,
) {
  const sizeStyles = {
    sm: {
      checkboxSize: 16,
      borderRadius: 2,
      padding: theme.spacing[1],
      gap: theme.spacing[1],
    },
    md: {
      checkboxSize: 20,
      borderRadius: 4,
      padding: theme.spacing[1],
      gap: theme.spacing[1],
    },
    lg: {
      checkboxSize: 24,
      borderRadius: 6,
      padding: theme.spacing[1],
      gap: theme.spacing[2],
    },
  };

  const currentSize = sizeStyles[size as keyof typeof sizeStyles];

  return StyleSheet.create({
    container: {
      flexDirection: "row",
      alignItems: "flex-start",
      opacity: disabled ? 0.5 : 1,
    },
    checkbox: {
      width: currentSize.checkboxSize,
      height: currentSize.checkboxSize,
      borderWidth: 1.5,
      borderColor: disabled
        ? theme.colors.border
        : checked
          ? theme.colors.primary
          : theme.colors.textMuted,
      borderRadius: currentSize.borderRadius,
      backgroundColor: disabled
        ? theme.colors.muted
        : checked
          ? theme.colors.primary
          : "transparent",
      alignItems: "center",
      justifyContent: "center",
    },
    content: {
      flex: 1,
      paddingTop: currentSize.padding * 0.25,
      paddingLeft: theme.spacing[2],
    },
  });
}
