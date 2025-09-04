import { Check } from "lucide-react-native";
import { forwardRef } from "react";
import { StyleSheet, TouchableOpacity, View } from "react-native";
import { useTheme } from "../../lib/theme/theme";
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
            {label && <Text style={styles.label}>{label}</Text>}
            {description && (
              <Text style={styles.description}>{description}</Text>
            )}
          </View>
        )}
      </TouchableOpacity>
    );
  },
);

Checkbox.displayName = "Checkbox";

function createStyles(
  theme: any,
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
      gap: theme.spacing[2],
    },
    lg: {
      checkboxSize: 24,
      borderRadius: 6,
      padding: theme.spacing[2],
      gap: theme.spacing[3],
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
          : theme.colors.border,
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
      paddingTop: currentSize.padding * 0.5,
      paddingLeft: theme.spacing[2],
    },
    label: {
      fontSize: size === "sm" ? 14 : size === "lg" ? 18 : 16,
      fontWeight: "500",
      color: disabled ? theme.colors.textDisabled : theme.colors.text,
      lineHeight: size === "sm" ? 18 : size === "lg" ? 22 : 20,
    },
    description: {
      fontSize: size === "sm" ? 12 : size === "lg" ? 16 : 14,
      color: disabled ? theme.colors.textDisabled : theme.colors.textMuted,
      marginTop: theme.spacing[1],
      lineHeight: size === "sm" ? 16 : size === "lg" ? 20 : 18,
    },
  });
}
