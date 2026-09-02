import { cva, type VariantProps } from "class-variance-authority";
import React, { forwardRef, useMemo } from "react";
import {
  ActivityIndicator,
  Platform,
  Pressable,
  type ViewStyle,
} from "react-native";
import { useTheme } from "../../lib/theme/theme";
import {
  borderRadius,
  fontFamilies,
  fontWeights,
  spacing,
  touchTargets,
  typeScale,
} from "../../lib/theme/tokens";
import { ButtonTextColorContext } from "./button-text-color";
import { ButtonPrimitive, ButtonPrimitiveProps } from "./primitives/button";
import { TextPrimitive } from "./primitives/text";

// Button variants. Emphasis comes from CONTRAST, not hue:
//   primary  → Paper/Ink monochrome fill (the one hero action per view)
//   secondary→ tonal raised surface
//   ghost    → transparent until hover
//   danger   → red ink (outline), never a fill, so it can't fight LIVE red
//   accent   → the reserved pink fill; opt-in escape hatch, use sparingly
// Pink otherwise lives on state (active nav/tab, selected chips, links,
// focus rings), not on button fills. The remaining keys are deprecated aliases
// kept for compatibility (see MIGRATION.md).
const buttonVariants = cva("", {
  variants: {
    variant: {
      primary: "primary",
      secondary: "secondary",
      ghost: "ghost",
      danger: "danger",
      accent: "accent",
      /** @deprecated use secondary */
      outline: "outline",
      /** @deprecated use danger */
      destructive: "destructive",
      /** @deprecated use primary */
      success: "success",
    },
    size: {
      sm: "sm",
      md: "md",
      lg: "lg",
      /** @deprecated use lg */
      xl: "xl",
      /** @deprecated use size md + pill prop */
      pill: "pill",
      /** @deprecated use IconButton */
      icon: "icon",
    },
  },
  defaultVariants: {
    variant: "primary",
    size: "md",
  },
});

type CanonicalVariant = "primary" | "secondary" | "ghost" | "danger" | "accent";

function canonicalVariant(
  variant: string | null | undefined,
): CanonicalVariant {
  switch (variant) {
    case "secondary":
    case "outline":
      return "secondary";
    case "ghost":
      return "ghost";
    case "danger":
    case "destructive":
      return "danger";
    case "accent":
      return "accent";
    default:
      return "primary";
  }
}

export interface ButtonProps
  extends
    Omit<ButtonPrimitiveProps, "children">,
    VariantProps<typeof buttonVariants> {
  href?: string; // For web support
  children?: React.ReactNode;
  leftIcon?: React.ReactNode;
  rightIcon?: React.ReactNode;
  loading?: boolean;
  loadingText?: string;
  /**
   * @deprecated No-op — buttons are fully rounded (pill) by default now. Kept
   * for backward compatibility.
   */
  pill?: boolean;
  width?: "full" | "min" | number;
  hoverStyle?: ButtonPrimitiveProps["hoverStyle"];
}

/**
 * The one button in the design system. **Never hand-roll a button from
 * Pressable/TouchableOpacity — use this.** Emphasis comes from CONTRAST, not
 * hue; pick the variant by the role the action plays on the screen:
 *
 * | variant     | looks like               | use it for |
 * |-------------|--------------------------|------------|
 * | `primary`   | Paper/Ink fill (lifted)  | the single most important action in a view — Publish, Save, Sign up. One per view. |
 * | `secondary` | tonal raised surface     | supporting actions beside a primary — Cancel, Save draft, filters. The default for most buttons. |
 * | `ghost`     | transparent until hover  | low-stakes or repeated actions, or buttons inside dense UI (toolbars, list rows). |
 * | `danger`    | red ink outline, no fill | destructive actions — Delete, End livestream. Ink (not a fill) so it can't fight the reserved LIVE red. |
 * | `accent`    | reserved pink fill       | a deliberate brand moment, used sparingly. Pink otherwise means *state*, not actions. |
 *
 * `size`: sm · md (default) · lg.  `width`: "full" (default) · "min" · number.
 * Every button is a fully rounded pill — the app's button shape. (Square
 * icon-only actions live in `IconButton`.)
 * Deprecated aliases: `outline`→secondary, `destructive`→danger, `success`→primary.
 */
export const Button = forwardRef<any, ButtonProps>(
  (
    {
      variant = "primary",
      size = "md",
      href,
      children,
      leftIcon,
      rightIcon,
      loading = false,
      loadingText,
      pill: _pill = false,
      disabled,
      style,
      width = "full",
      hoverStyle,
      ...props
    },
    ref,
  ) => {
    const { theme, icons } = useTheme();
    const v = canonicalVariant(variant);

    // Variant styles: emphasis from contrast, pink reserved for state.
    const variantStyles = useMemo(() => {
      const c = theme.colors;
      switch (v) {
        case "secondary":
          return {
            base: {
              backgroundColor: c.surface2,
              borderWidth: 1,
              borderColor: c.border,
            },
            hover: {
              backgroundColor: c.surfaceHover,
              borderColor: c.borderStrong,
            },
            pressed: { backgroundColor: c.surface3 },
            text: c.text1,
          };
        case "ghost":
          return {
            base: { backgroundColor: "transparent", borderWidth: 0 },
            hover: { backgroundColor: c.surface2 },
            pressed: { backgroundColor: c.surface3 },
            text: c.text1,
          };
        case "danger":
          // Red ink, not a red fill — an outline that tints on hover. Keeps
          // destructive legible without competing with the reserved LIVE red.
          return {
            base: {
              backgroundColor: "transparent",
              borderWidth: 1,
              borderColor: c.borderStrong,
            },
            hover: { backgroundColor: c.dangerSoft, borderColor: c.danger },
            pressed: { backgroundColor: c.dangerSoft },
            text: c.danger,
          };
        case "accent":
          // Reserved pink fill. Opt-in only, for a deliberate brand moment.
          return {
            base: { backgroundColor: c.primary, borderWidth: 0 },
            hover: { opacity: 0.9 },
            pressed: { opacity: 0.8 },
            text: c.primaryForeground,
          };
        case "primary":
        default:
          // Paper on dark / Ink on light — the highest-contrast fill, lifted
          // with a hairline shadow. One per view.
          return {
            base: {
              backgroundColor: c.inverse,
              borderWidth: 0,
              ...theme.shadows.sm,
            },
            hover: { opacity: 0.92 },
            pressed: { opacity: 0.85 },
            text: c.inverseForeground,
          };
      }
    }, [v, theme.colors, theme.shadows]);

    // Size styles: three sizes on the 4px grid; text from the type scale.
    // Touch platforms keep ≥44px effective targets via hitSlop below.
    const sizeStyles = useMemo(() => {
      switch (size) {
        case "sm":
        case "pill":
          return {
            button: {
              minHeight: spacing[8],
              paddingHorizontal: spacing[3],
              borderRadius: borderRadius.full,
            },
            inner: { gap: spacing[1] },
            text: typeScale.sm,
            hitSlop: (touchTargets.minimum - spacing[8]) / 2,
          };
        case "lg":
        case "xl":
          return {
            button: {
              minHeight: touchTargets.comfortable,
              paddingHorizontal: spacing[6],
              borderRadius: borderRadius.full,
            },
            inner: { gap: spacing[2] },
            text: typeScale.md,
            hitSlop: 0,
          };
        case "icon":
          return {
            button: {
              minHeight: spacing[10],
              minWidth: spacing[10],
              paddingHorizontal: 0,
              borderRadius: borderRadius.full,
            },
            inner: { gap: 0 },
            text: typeScale.base,
            hitSlop: (touchTargets.minimum - spacing[10]) / 2,
          };
        case "md":
        default:
          return {
            button: {
              minHeight: spacing[10],
              paddingHorizontal: spacing[4],
              borderRadius: borderRadius.full,
            },
            inner: { gap: spacing[2] },
            text: typeScale.base,
            hitSlop: (touchTargets.minimum - spacing[10]) / 2,
          };
      }
    }, [size]);

    const textStyle = useMemo(
      () => [
        sizeStyles.text,
        {
          color: variantStyles.text,
          fontWeight: fontWeights.medium,
          fontFamily: fontFamilies.medium,
        },
      ],
      [sizeStyles.text, variantStyles.text],
    );

    const iconSize = useMemo(() => {
      switch (size) {
        case "sm":
        case "pill":
          return icons.size.sm;
        case "lg":
        case "xl":
          return icons.size.lg;
        default:
          return icons.size.md;
      }
    }, [size, icons]);

    const widthStyle = useMemo<ViewStyle>(() => {
      if (width === "full") {
        return { width: "100%" };
      } else if (width === "min") {
        return { alignSelf: "flex-start" };
      } else {
        return { width };
      }
    }, [width]);

    // Touch platforms only: expand the touch target to ≥44px
    const hitSlop =
      Platform.OS !== "web" && sizeStyles.hitSlop > 0
        ? sizeStyles.hitSlop
        : undefined;

    // Icons inherit the button's text color — so an icon stays legible when the
    // fill flips (e.g. a near-white icon would vanish on the Paper primary).
    // Callers that need a bespoke icon color can wrap it in a colored element.
    const tintIcon = (node: React.ReactNode): React.ReactNode =>
      React.isValidElement(node)
        ? React.cloneElement(node as React.ReactElement<{ color?: string }>, {
            color: variantStyles.text,
          })
        : node;

    // if href, wrap in pressable that renders as <a> on web
    let Wrapper = React.Fragment;
    let wrapperProps = {};
    if (href) {
      Wrapper = Pressable;
      wrapperProps = {
        href,
        as: "a",
      };
    }
    return (
      <Wrapper {...wrapperProps}>
        <ButtonPrimitive.Root
          ref={ref}
          disabled={disabled || loading}
          hitSlop={hitSlop}
          style={
            [variantStyles.base, sizeStyles.button, widthStyle, style] as any
          }
          hoverStyle={[variantStyles.hover, hoverStyle]}
          pressedStyle={variantStyles.pressed}
          {...props}
        >
          <ButtonPrimitive.Content style={sizeStyles.inner}>
            <ButtonTextColorContext.Provider value={variantStyles.text}>
              {loading && !leftIcon ? (
                <ButtonPrimitive.Icon position="left">
                  <ActivityIndicator size="small" color={variantStyles.text} />
                </ButtonPrimitive.Icon>
              ) : leftIcon ? (
                <ButtonPrimitive.Icon position="left">
                  {tintIcon(leftIcon)}
                </ButtonPrimitive.Icon>
              ) : null}

              {typeof children === "string" ? (
                <TextPrimitive.Root style={textStyle as any}>
                  {loading && loadingText ? loadingText : children}
                </TextPrimitive.Root>
              ) : loading && loadingText ? (
                loadingText
              ) : (
                children
              )}

              {loading && rightIcon ? (
                <ButtonPrimitive.Icon position="right">
                  <ActivityIndicator size="small" color={variantStyles.text} />
                </ButtonPrimitive.Icon>
              ) : rightIcon ? (
                <ButtonPrimitive.Icon
                  position="right"
                  style={{ width: iconSize, height: iconSize }}
                >
                  {tintIcon(rightIcon)}
                </ButtonPrimitive.Icon>
              ) : null}
            </ButtonTextColorContext.Provider>
          </ButtonPrimitive.Content>
        </ButtonPrimitive.Root>
      </Wrapper>
    );
  },
);

Button.displayName = "Button";

// Export button variants for external use
export { buttonVariants };
