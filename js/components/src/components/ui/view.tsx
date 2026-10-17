import { cva, type VariantProps } from "class-variance-authority";
import { forwardRef } from "react";
import {
  View as RNView,
  ViewProps as RNViewProps,
  ViewStyle,
} from "react-native";
import { useTheme } from "../../lib/theme/theme";
import * as zero from "../../ui";

// View variants using class-variance-authority pattern
const viewVariants = cva("", {
  variants: {
    variant: {
      default: "default",
      card: "card",
      overlay: "overlay",
      surface: "surface",
      container: "container",
    },
    padding: {
      none: "none",
      xs: "xs",
      sm: "sm",
      md: "md",
      lg: "lg",
      xl: "xl",
    },
    margin: {
      none: "none",
      xs: "xs",
      sm: "sm",
      md: "md",
      lg: "lg",
      xl: "xl",
    },
    direction: {
      row: "row",
      column: "column",
      "row-reverse": "row-reverse",
      "column-reverse": "column-reverse",
    },
    align: {
      start: "start",
      center: "center",
      end: "end",
      stretch: "stretch",
      baseline: "baseline",
    },
    justify: {
      start: "start",
      center: "center",
      end: "end",
      between: "between",
      around: "around",
      evenly: "evenly",
    },
    flex: {
      none: "none",
      auto: "auto",
      initial: "initial",
    },
  },
  defaultVariants: {
    variant: "default",
    padding: "none",
    margin: "none",
    direction: "column",
    align: "stretch",
    justify: "start",
    flex: "none",
  },
});

export interface ViewProps
  extends
    Omit<RNViewProps, "style">,
    Omit<VariantProps<typeof viewVariants>, "flex"> {
  // Style props
  style?: ViewStyle | ViewStyle[];

  // Convenience props
  fullWidth?: boolean;
  fullHeight?: boolean;
  centered?: boolean;

  // Background
  backgroundColor?: string;

  // Border
  borderColor?: string;
  borderWidth?: number;
  borderRadius?: number;

  // Shadow (web only)
  shadow?: boolean;

  // Custom flex values
  flex?: number | "none" | "auto" | "initial";
}

export const View = forwardRef<RNView, ViewProps>(
  (
    {
      variant = "default",
      padding = "none",
      margin = "none",
      direction = "column",
      align = "stretch",
      justify = "start",
      flex = "none",
      fullWidth = false,
      fullHeight = false,
      centered = false,
      backgroundColor,
      borderColor,
      borderWidth,
      borderRadius,
      shadow = false,
      style,
      ...props
    },
    ref,
  ) => {
    const { zero: zt } = useTheme();

    // Map variant to styles using theme.zero
    const variantStyles: ViewStyle = (() => {
      switch (variant) {
        case "card":
          return {
            ...zt.bg.card,
            borderRadius: zero.borderRadius.lg,
            ...zero.shadows.md,
          };
        case "overlay":
          return zt.bg.overlay;
        case "surface":
          return zt.bg.background;
        case "container":
          return {
            ...zt.bg.background,
            ...zero.p[8],
          };
        default:
          return {};
      }
    })();

    // Map padding to zero tokens
    const paddingValue = (() => {
      switch (padding) {
        case "xs":
          return zero.p[1];
        case "sm":
          return zero.p[2];
        case "md":
          return zero.p[4];
        case "lg":
          return zero.p[6];
        case "xl":
          return zero.p[8];
        default:
          return {};
      }
    })();

    // Map margin to zero tokens
    const marginValue = (() => {
      switch (margin) {
        case "xs":
          return zero.m[1];
        case "sm":
          return zero.m[2];
        case "md":
          return zero.m[4];
        case "lg":
          return zero.m[6];
        case "xl":
          return zero.m[8];
        default:
          return {};
      }
    })();

    // Map flex direction
    const flexDirection = (() => {
      switch (direction) {
        case "row":
          return "row";
        case "column":
          return "column";
        case "row-reverse":
          return "row-reverse";
        case "column-reverse":
          return "column-reverse";
        default:
          return "column";
      }
    })() as ViewStyle["flexDirection"];

    // Map align items
    const alignItems = (() => {
      switch (align) {
        case "start":
          return "flex-start";
        case "center":
          return "center";
        case "end":
          return "flex-end";
        case "stretch":
          return "stretch";
        case "baseline":
          return "baseline";
        default:
          return "stretch";
      }
    })() as ViewStyle["alignItems"];

    // Map justify content
    const justifyContent = (() => {
      switch (justify) {
        case "start":
          return "flex-start";
        case "center":
          return "center";
        case "end":
          return "flex-end";
        case "between":
          return "space-between";
        case "around":
          return "space-around";
        case "evenly":
          return "space-evenly";
        default:
          return "flex-start";
      }
    })() as ViewStyle["justifyContent"];

    // Map flex value
    const flexValue = (() => {
      if (typeof flex === "number") {
        return flex;
      }
      switch (flex) {
        case "auto":
          return undefined; // auto is default
        case "initial":
          return 0;
        case "none":
        default:
          return undefined;
      }
    })();

    const computedStyle: ViewStyle = {
      ...variantStyles,
      ...paddingValue,
      ...marginValue,
      flexDirection,
      alignItems,
      justifyContent,
      ...(flexValue !== undefined && { flex: flexValue }),
      ...(fullWidth && { width: "100%" }),
      ...(fullHeight && { height: "100%" }),
      ...(centered && {
        alignItems: "center",
        justifyContent: "center",
      }),
      ...(backgroundColor && { backgroundColor }),
      ...(borderColor && { borderColor }),
      ...(borderWidth !== undefined && { borderWidth }),
      ...(borderRadius !== undefined && { borderRadius }),
      ...(shadow && zero.shadows.md),
    };

    const finalStyle = Array.isArray(style)
      ? [computedStyle, ...style]
      : [computedStyle, style];

    return <RNView ref={ref} style={finalStyle} {...props} />;
  },
);

View.displayName = "View";

// Convenience components
// NOTE: the old `Card`/`Surface` View-variant wrappers moved to
// ./surface.tsx as level-based components (see MIGRATION.md).

export const Container = forwardRef<RNView, Omit<ViewProps, "variant">>(
  (props, ref) => <View ref={ref} variant="container" {...props} />,
);

Container.displayName = "Container";

export const Overlay = forwardRef<RNView, Omit<ViewProps, "variant">>(
  (props, ref) => <View ref={ref} variant="overlay" {...props} />,
);

Overlay.displayName = "Overlay";

export const Row = forwardRef<RNView, Omit<ViewProps, "direction">>(
  (props, ref) => <View ref={ref} direction="row" {...props} />,
);

Row.displayName = "Row";

export const Column = forwardRef<RNView, Omit<ViewProps, "direction">>(
  (props, ref) => <View ref={ref} direction="column" {...props} />,
);

Column.displayName = "Column";

export const Center = forwardRef<RNView, ViewProps>((props, ref) => (
  <View ref={ref} centered {...props} />
));

Center.displayName = "Center";

// Export view variants for external use
export { viewVariants };
