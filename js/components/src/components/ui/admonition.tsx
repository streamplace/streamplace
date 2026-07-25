import { AlertCircle, CheckCircle, Info, XCircle } from "lucide-react-native";
import { View, ViewStyle } from "react-native";
import { hexToRgba, useTheme } from "../../ui";
import { Text } from "./text";

type AdmonitionVariant = "default" | "success" | "error" | "info" | "warning";
type AdmonitionSize = "sm" | "md" | "lg";

type AdmonitionProps = {
  variant?: AdmonitionVariant;
  size?: AdmonitionSize;
  title?: string;
  children?: React.ReactNode;
  iconLeft?: React.ComponentType<any>;
  style?: ViewStyle;
};

export function Admonition({
  variant = "default",
  size = "md",
  title,
  children,
  iconLeft,
  style,
}: AdmonitionProps) {
  const { theme, icons } = useTheme();

  const defaultIconLeft = (() => {
    if (iconLeft) return iconLeft;
    switch (variant) {
      case "success":
        return CheckCircle;
      case "error":
        return XCircle;
      case "info":
        return Info;
      case "warning":
        return AlertCircle;
      default:
        return Info;
    }
  })();

  const FinalIconLeft = defaultIconLeft;

  const variantStyles: Record<AdmonitionVariant, ViewStyle> = {
    default: {
      backgroundColor: theme.colors.secondary,
      borderColor: theme.colors.border,
    },
    success: {
      backgroundColor: hexToRgba(theme.colors.success, 0.08),
      borderColor: theme.colors.success,
    },
    error: {
      backgroundColor: hexToRgba(theme.colors.destructive, 0.08),
      borderColor: theme.colors.destructive,
    },
    info: {
      backgroundColor: hexToRgba(theme.colors.info, 0.08),
      borderColor: theme.colors.info,
    },
    warning: {
      backgroundColor: hexToRgba(theme.colors.warning, 0.08),
      borderColor: theme.colors.warning,
    },
  };

  const iconColor = (() => {
    switch (variant) {
      case "success":
        return theme.colors.success;
      case "error":
        return theme.colors.destructive;
      case "info":
        return theme.colors.info;
      case "warning":
        return theme.colors.warning;
      default:
        return theme.colors.foreground;
    }
  })();

  const sizeConfig = (() => {
    switch (size) {
      case "sm":
        return {
          borderRadius: 8,
          padding: 12,
          gap: 6,
          iconSize: icons.size.md,
          titleSize: "base" as const,
          contentSize: "sm" as const,
          innerGap: 8,
        };
      case "lg":
        return {
          borderRadius: 16,
          padding: 20,
          gap: 12,
          iconSize: icons.size.xl,
          titleSize: "xl" as const,
          contentSize: "lg" as const,
          innerGap: 16,
        };
      case "md":
      default:
        return {
          borderRadius: 12,
          padding: 16,
          gap: 8,
          iconSize: icons.size.lg,
          titleSize: "lg" as const,
          contentSize: "base" as const,
          innerGap: 12,
        };
    }
  })();

  let childrenIn = (
    <View
      style={{
        paddingLeft: title ? sizeConfig.iconSize + sizeConfig.innerGap : 0,
      }}
    >
      {typeof children === "string" ? (
        <Text
          size={sizeConfig.contentSize}
          style={{ color: theme.colors.cardForeground, flexWrap: "wrap" }}
        >
          {children}
        </Text>
      ) : (
        children
      )}
    </View>
  );

  return (
    <View
      style={[
        {
          borderRadius: sizeConfig.borderRadius,
          borderWidth: 1,
          padding: sizeConfig.padding,
          gap: sizeConfig.gap,
        },
        variantStyles[variant],
        style,
      ]}
    >
      <View
        style={{
          flexDirection: "row",
          alignItems: "flex-start",
          gap: sizeConfig.innerGap,
        }}
      >
        {FinalIconLeft && (
          <FinalIconLeft size={sizeConfig.iconSize} color={iconColor} />
        )}
        {title ? (
          <Text
            size={sizeConfig.titleSize}
            weight="semibold"
            style={{ flex: 1 }}
          >
            {title}
          </Text>
        ) : (
          children && <View style={{ flex: 1 }}>{childrenIn}</View>
        )}
      </View>
      {children && title && childrenIn}
    </View>
  );
}
