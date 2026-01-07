import { cva, type VariantProps } from "class-variance-authority";
import React, { forwardRef } from "react";
import { TouchableWithoutFeedback } from "react-native";
import { useTheme } from "../../lib/theme/theme";
import * as zero from "../../ui";
import { InputPrimitive, InputPrimitiveProps } from "./primitives/input";

const inputVariants = cva("", {
  variants: {
    variant: {
      default: "default",
      filled: "filled",
      underlined: "underlined",
    },
    size: {
      sm: "sm",
      md: "md",
      lg: "lg",
    },
  },
  defaultVariants: {
    variant: "default",
    size: "md",
  },
});

export interface InputProps
  extends Omit<InputPrimitiveProps, "style" | "error">,
    VariantProps<typeof inputVariants> {
  label?: string;
  description?: string;
  error?: string;
  required?: boolean;
  leftAddon?: React.ReactNode;
  rightAddon?: React.ReactNode;
  containerStyle?: any;
  inputStyle?: any;
}

export const Input = forwardRef<any, InputProps>(
  (
    {
      variant = "default",
      size = "md",
      label,
      description,
      error,
      required = false,
      leftAddon,
      rightAddon,
      disabled = false,
      containerStyle,
      inputStyle,
      ...props
    },
    ref,
  ) => {
    const { zero: zt, theme } = useTheme();
    const [isFocused, setIsFocused] = React.useState(false);
    const inputRef = React.useRef<any>(null);

    // Get variant and size styles using theme.zero
    const containerStyles = React.useMemo(() => {
      const variantStyle = (() => {
        switch (variant) {
          case "filled":
            return [zt.bg.muted];
          case "underlined":
            return [
              zt.bg.transparent,
              zt.border.bottom.default,
              { borderRadius: 0, paddingHorizontal: 0 },
            ];
          default:
            return [zt.bg.background, zt.border.default];
        }
      })();

      const sizeStyle = (() => {
        switch (size) {
          case "sm":
            return [
              zero.px[3],
              zero.py[2],
              { borderRadius: zero.borderRadius.md },
            ];
          case "lg":
            return [
              zero.px[4],
              zero.py[3],
              { borderRadius: zero.borderRadius.md },
            ];
          default:
            return [
              zero.px[3],
              zero.py[2],
              { borderRadius: zero.borderRadius.md },
            ];
        }
      })();

      const focusStyle = isFocused ? zt.border.primary : null;
      return [variantStyle, sizeStyle, focusStyle].filter(Boolean);
    }, [variant, size, zt, isFocused]);

    const textStyles = React.useMemo(() => {
      const baseTextStyle = [zt.text.foreground, zt.bg.transparent];

      switch (size) {
        case "sm":
          return [...baseTextStyle, zt.text.sm];
        case "lg":
          return [...baseTextStyle, zt.text.lg];
        default:
          return [...baseTextStyle, zt.text.md];
      }
    }, [size, zt]);

    const handleFocus = React.useCallback(
      (event: any) => {
        setIsFocused(true);
        if (props.onFocus) {
          props.onFocus(event);
        }
      },
      [props.onFocus],
    );

    const handleBlur = React.useCallback(
      (event: any) => {
        setIsFocused(false);
        if (props.onBlur) {
          props.onBlur(event);
        }
      },
      [props.onBlur],
    );

    const handleContainerPress = React.useCallback(() => {
      if (inputRef.current && !disabled) {
        inputRef.current.focus();
      }
    }, [disabled]);

    const hasAddons = leftAddon || rightAddon;

    if (hasAddons) {
      return (
        <InputPrimitive.Group>
          {label && (
            <InputPrimitive.Label
              required={required}
              disabled={disabled}
              error={!!error}
            >
              {label}
            </InputPrimitive.Label>
          )}

          <TouchableWithoutFeedback onPress={handleContainerPress}>
            <InputPrimitive.Container
              focused={isFocused}
              error={!!error}
              disabled={disabled}
              style={[containerStyles, containerStyle, { padding: 0 }]}
            >
              {leftAddon && (
                <InputPrimitive.Addon position="left">
                  {leftAddon}
                </InputPrimitive.Addon>
              )}

              <InputPrimitive.Root
                ref={(node) => {
                  inputRef.current = node;
                  if (ref) {
                    if (typeof ref === "function") {
                      ref(node);
                    } else {
                      ref.current = node;
                    }
                  }
                }}
                disabled={disabled}
                error={!!error}
                onFocus={handleFocus}
                onBlur={handleBlur}
                style={[textStyles, inputStyle, { outline: "none" }]}
                placeholderTextColor={
                  disabled ? theme.colors.textDisabled : theme.colors.textMuted
                }
                {...props}
              />

              {rightAddon && (
                <InputPrimitive.Addon position="right">
                  {rightAddon}
                </InputPrimitive.Addon>
              )}
            </InputPrimitive.Container>
          </TouchableWithoutFeedback>

          {description && !error && (
            <InputPrimitive.Description disabled={disabled}>
              {description}
            </InputPrimitive.Description>
          )}

          <InputPrimitive.Error visible={!!error}>{error}</InputPrimitive.Error>
        </InputPrimitive.Group>
      );
    }

    return (
      <InputPrimitive.Group>
        {label && (
          <InputPrimitive.Label
            required={required}
            disabled={disabled}
            error={!!error}
          >
            {label}
          </InputPrimitive.Label>
        )}

        <InputPrimitive.Root
          ref={(node) => {
            inputRef.current = node;
            if (ref) {
              if (typeof ref === "function") {
                ref(node);
              } else {
                ref.current = node;
              }
            }
          }}
          disabled={disabled}
          error={!!error}
          onFocus={handleFocus}
          onBlur={handleBlur}
          style={[
            containerStyles,
            textStyles,
            containerStyle,
            inputStyle,
            { outline: "none" },
          ]}
          placeholderTextColor={
            disabled ? theme.colors.textDisabled : theme.colors.textMuted
          }
          {...props}
        />

        {description && !error && (
          <InputPrimitive.Description disabled={disabled}>
            {description}
          </InputPrimitive.Description>
        )}

        <InputPrimitive.Error visible={!!error}>{error}</InputPrimitive.Error>
      </InputPrimitive.Group>
    );
  },
);

Input.displayName = "Input";

// Export input variants for external use
export { inputVariants };
