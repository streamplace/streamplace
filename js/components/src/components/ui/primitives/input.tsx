import {
  BottomSheetTextInput,
  useBottomSheetInternal,
} from "@gorhom/bottom-sheet";
import React, { forwardRef } from "react";
import {
  BlurEvent,
  FocusEvent,
  Platform,
  StyleSheet,
  Text,
  TextInput,
  TextInputProps,
  TextProps,
  TouchableOpacity,
  View,
  ViewProps,
} from "react-native";
import { useTheme } from "../../../lib/theme/theme";
import * as tokens from "../../../lib/theme/tokens";

export interface InputPrimitiveProps extends Omit<
  TextInputProps,
  "onChange" | "onFocus" | "onBlur"
> {
  error?: boolean;
  disabled?: boolean;
  loading?: boolean;
  onChange?: (text: string) => void;
  onFocus?: (event: FocusEvent) => void;
  onBlur?: (event: BlurEvent) => void;
}

export const InputRoot = forwardRef<any, InputPrimitiveProps>(
  (
    {
      value,
      onChangeText,
      onChange,
      onFocus,
      onBlur,
      error = false,
      disabled = false,
      loading = false,
      editable,
      style,
      placeholderTextColor: placeholderTextColorProp,
      ...props
    },
    ref,
  ) => {
    const [isFocused, setIsFocused] = React.useState(false);
    const { theme } = useTheme();
    const placeholderTextColor =
      placeholderTextColorProp ?? theme.colors.textMuted;

    let isInBottomSheet = false;
    try {
      useBottomSheetInternal();
      isInBottomSheet = true;
    } catch {
      isInBottomSheet = false;
    }

    const InputComponent =
      isInBottomSheet && Platform.OS !== "web"
        ? BottomSheetTextInput
        : TextInput;

    const handleChangeText = React.useCallback(
      (text: string) => {
        if (onChangeText) {
          onChangeText(text);
        }
        if (onChange) {
          onChange(text);
        }
      },
      [onChangeText, onChange],
    );

    const handleFocus = React.useCallback(
      (event: FocusEvent) => {
        setIsFocused(true);
        if (onFocus) {
          onFocus(event);
        }
      },
      [onFocus],
    );

    const handleBlur = React.useCallback(
      (event: BlurEvent) => {
        setIsFocused(false);
        if (onBlur) {
          onBlur(event);
        }
      },
      [onBlur],
    );

    const colorStyles = {
      borderColor: error ? theme.colors.destructive : theme.colors.border,
      ...(disabled && {
        backgroundColor: theme.colors.muted,
        opacity: 0.6,
      }),
      ...(loading && { opacity: 0.7 }),
    };

    return (
      <InputComponent
        ref={ref}
        value={value}
        onChangeText={handleChangeText}
        onFocus={handleFocus}
        onBlur={handleBlur}
        editable={!disabled && !loading && editable}
        placeholderTextColor={placeholderTextColor}
        style={[structuralStyles.input, colorStyles, style]}
        {...props}
      />
    );
  },
);

InputRoot.displayName = "InputRoot";

export interface InputContainerProps extends ViewProps {
  focused?: boolean;
  error?: boolean;
  disabled?: boolean;
}

export const InputContainer = forwardRef<View, InputContainerProps>(
  (
    {
      children,
      focused = false,
      error = false,
      disabled = false,
      style,
      ...props
    },
    ref,
  ) => {
    const { theme } = useTheme();

    const colorStyles = {
      borderColor: error
        ? theme.colors.destructive
        : focused
          ? theme.colors.ring
          : theme.colors.border,
      backgroundColor: theme.colors.background,
      ...(disabled && {
        backgroundColor: theme.colors.muted,
        opacity: 0.6,
      }),
    };

    return (
      <View
        ref={ref}
        style={[structuralStyles.container, colorStyles, style]}
        {...props}
      >
        {children}
      </View>
    );
  },
);

InputContainer.displayName = "InputContainer";

export interface InputLabelProps extends TextProps {
  required?: boolean;
  disabled?: boolean;
  error?: boolean;
}

export const InputLabel = forwardRef<Text, InputLabelProps>(
  (
    {
      children,
      required = false,
      disabled = false,
      error = false,
      style,
      ...props
    },
    ref,
  ) => {
    const { theme } = useTheme();

    const colorStyle = {
      color: error
        ? theme.colors.destructive
        : disabled
          ? theme.colors.textMuted
          : theme.colors.text,
      ...(disabled && { opacity: 0.6 }),
    };

    return (
      <Text
        ref={ref}
        style={[structuralStyles.label, colorStyle, style]}
        {...props}
      >
        {children}
        {required && (
          <Text style={{ color: theme.colors.destructive }}> *</Text>
        )}
      </Text>
    );
  },
);

InputLabel.displayName = "InputLabel";

export interface InputDescriptionProps extends TextProps {
  error?: boolean;
  disabled?: boolean;
}

export const InputDescription = forwardRef<Text, InputDescriptionProps>(
  ({ children, error = false, disabled = false, style, ...props }, ref) => {
    const { theme } = useTheme();

    const colorStyle = {
      color: error
        ? theme.colors.destructive
        : disabled
          ? theme.colors.textMuted
          : theme.colors.textMuted,
      ...(disabled && { opacity: 0.6 }),
    };

    return (
      <Text
        ref={ref}
        style={[structuralStyles.description, colorStyle, style]}
        {...props}
      >
        {children}
      </Text>
    );
  },
);

InputDescription.displayName = "InputDescription";

export interface InputErrorProps extends TextProps {
  visible?: boolean;
}

export const InputError = forwardRef<Text, InputErrorProps>(
  ({ children, visible = true, style, ...props }, ref) => {
    const { theme } = useTheme();

    if (!visible || !children) return null;

    return (
      <Text
        ref={ref}
        style={[
          structuralStyles.error,
          { color: theme.colors.destructive },
          style,
        ]}
        {...props}
      >
        {children}
      </Text>
    );
  },
);

InputError.displayName = "InputError";

export interface InputAddonProps extends ViewProps {
  position?: "left" | "right";
  touchable?: boolean;
  onPress?: () => void;
}

export const InputAddon = forwardRef<
  React.ComponentRef<typeof View> | React.ComponentRef<typeof TouchableOpacity>,
  InputAddonProps
>(
  (
    {
      children,
      position = "left",
      touchable = false,
      onPress,
      style,
      ...props
    },
    ref,
  ) => {
    const addonStyle = [
      structuralStyles.addon,
      structuralStyles[
        `addon${position.charAt(0).toUpperCase() + position.slice(1)}` as keyof typeof structuralStyles
      ],
      style,
    ];

    const { onBlur, onFocus, ...restProps } = props;
    const touchableProps = {
      ...restProps,
      ...(onBlur !== null && onBlur !== undefined && { onBlur }),
      ...(onFocus !== null && onFocus !== undefined && { onFocus }),
    };

    if (touchable && onPress) {
      return (
        <TouchableOpacity
          ref={ref as React.Ref<React.ComponentRef<typeof TouchableOpacity>>}
          style={addonStyle as any}
          onPress={onPress}
          {...touchableProps}
        >
          {children}
        </TouchableOpacity>
      );
    }

    return (
      <View
        ref={ref as React.Ref<React.ComponentRef<typeof View>>}
        style={addonStyle as any}
        {...props}
      >
        {children}
      </View>
    );
  },
);

InputAddon.displayName = "InputAddon";

export interface InputGroupProps extends ViewProps {
  spacing?: number;
}

export const InputGroup = forwardRef<View, InputGroupProps>(
  ({ children, spacing = 8, style, ...props }, ref) => {
    return (
      <View
        ref={ref}
        style={[structuralStyles.group, { gap: spacing }, style]}
        {...props}
      >
        {children}
      </View>
    );
  },
);

InputGroup.displayName = "InputGroup";

const structuralStyles = StyleSheet.create({
  input: {
    minHeight: 44,
    paddingHorizontal: 12,
    paddingVertical: 8,
    fontSize: 16,
    borderWidth: 1,
    borderRadius: 8,
    boxShadow: "none",
    fontFamily: tokens.fontFamilies.regular,
    ...Platform.select({
      ios: {
        paddingVertical: 12,
      },
      android: {
        paddingVertical: 8,
        textAlignVertical: "center",
      },
    }),
  },
  container: {
    flexDirection: "row",
    alignItems: "center",
    borderWidth: 1,
    borderRadius: 8,
    paddingHorizontal: 12,
    minHeight: 44,
  },
  label: {
    fontSize: 14,
    fontWeight: "500",
    marginBottom: 4,
  },
  description: {
    fontSize: 12,
    marginTop: 4,
  },
  error: {
    fontSize: 12,
    marginTop: 4,
  },
  addon: {
    alignItems: "center",
    justifyContent: "center",
    paddingHorizontal: 8,
  },
  addonLeft: {
    marginRight: 8,
  },
  addonRight: {
    marginLeft: 8,
  },
  group: {
    flexDirection: "column",
  },
});

export const InputPrimitive = {
  Root: InputRoot,
  Container: InputContainer,
  Label: InputLabel,
  Description: InputDescription,
  Error: InputError,
  Addon: InputAddon,
  Group: InputGroup,
};
