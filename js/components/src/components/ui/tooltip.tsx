import {
  cloneElement,
  forwardRef,
  isValidElement,
  useId,
  useRef,
  useState,
} from "react";
import { Platform, StyleSheet, View } from "react-native";
import { useTheme } from "../../lib/theme/theme";
import { Text } from "../ui/text";
import { Checkbox, CheckboxProps } from "./checkbox";

export interface TooltipProps {
  content: string;
  children: React.ReactNode;
  position?: "top" | "bottom" | "left" | "right";
  style?: any;
}

export const Tooltip = forwardRef<any, TooltipProps>(
  ({ content, children, position = "top", style }, ref) => {
    const { theme } = useTheme();
    const [isVisible, setIsVisible] = useState(false);
    const id = useId();
    const focused = useRef(false);
    const hovered = useRef(false);
    const styles = createStyles(theme, position);

    const handleHoverIn = () => {
      hovered.current = true;
      setIsVisible(true);
    };

    const handleHoverOut = () => {
      hovered.current = false;
      if (!focused.current) setIsVisible(false);
    };

    return (
      <View
        ref={ref}
        style={[styles.container, style]}
        onPointerEnter={handleHoverIn}
        onPointerLeave={handleHoverOut}
        {...(Platform.OS === "web"
          ? {
              onFocus: () => {
                focused.current = true;
                setIsVisible(true);
              },
              onBlur: () => {
                focused.current = false;
                if (!hovered.current) setIsVisible(false);
              },
              onKeyDown: (event: {
                key: string;
                stopPropagation: () => void;
              }) => {
                if (event.key === "Escape" && isVisible) {
                  event.stopPropagation();
                  setIsVisible(false);
                }
              },
            }
          : {})}
      >
        {Platform.OS === "web" &&
        isValidElement<CheckboxProps>(children) &&
        children.type === Checkbox
          ? cloneElement(children, { "aria-describedby": id })
          : children}
        {(isVisible || Platform.OS === "web") && (
          <View
            nativeID={id}
            pointerEvents={Platform.OS === "web" ? "none" : "auto"}
            style={[styles.tooltip, !isVisible && { display: "none" }]}
          >
            <Text style={styles.tooltipText}>{content}</Text>
          </View>
        )}
      </View>
    );
  },
);

Tooltip.displayName = "Tooltip";

function createStyles(theme: any, position: string) {
  const positionStyles = {
    top: {
      tooltip: {
        bottom: "100%",
        left: "50%",
        transform: [{ translateX: -50 }],
        marginBottom: theme.spacing[1],
      },
      arrow: {
        top: "100%",
        left: "50%",
        transform: [{ translateX: -50 }],
        borderTopColor: theme.colors.card,
      },
    },
    bottom: {
      tooltip: {
        top: "100%",
        left: "50%",
        transform: [{ translateX: -50 }],
        marginTop: theme.spacing[1],
      },
      arrow: {
        bottom: "100%",
        left: "50%",
        transform: [{ translateX: -50 }],
        borderBottomColor: theme.colors.card,
      },
    },
    left: {
      tooltip: {
        right: "100%",
        top: "50%",
        transform: [{ translateY: -50 }],
        marginRight: theme.spacing[1],
      },
      arrow: {
        left: "100%",
        top: "50%",
        transform: [{ translateY: -50 }],
        borderLeftColor: theme.colors.card,
      },
    },
    right: {
      tooltip: {
        left: "100%",
        top: "50%",
        transform: [{ translateY: -50 }],
        marginLeft: theme.spacing[1],
      },
      arrow: {
        right: "100%",
        top: "50%",
        transform: [{ translateY: -50 }],
        borderRightColor: theme.colors.card,
      },
    },
  };

  const currentPosition =
    positionStyles[position as keyof typeof positionStyles];

  return StyleSheet.create({
    container: {
      position: "relative",
    },
    tooltip: {
      position: "absolute",
      backgroundColor: theme.colors.card,
      borderRadius: theme.borderRadius.md,
      padding: theme.spacing[2],
      maxWidth: 200,
      ...theme.shadows.lg,
      ...currentPosition.tooltip,
      zIndex: 1000,
    },
    tooltipText: {
      color: theme.colors.text,
      fontSize: 12,
      lineHeight: 16,
      textAlign: "left",
    },
  });
}
