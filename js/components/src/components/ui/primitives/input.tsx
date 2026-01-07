import {
  BottomSheetTextInput,
  useBottomSheetInternal,
} from "@gorhom/bottom-sheet";
import React, { forwardRef } from "react";
import {
  NativeSyntheticEvent,
  Platform,
  StyleSheet,
  Text,
  TextInput,
  TextInputFocusEventData,
  TextInputProps,
  TextProps,
  TouchableOpacity,
  View,
  ViewProps,
} from "react-native";
import { tokens } from "../../../ui";

// Base input primitive interface
export interface InputPrimitiveProps extends Omit<TextInputProps, "onChange"> {
  error?: boolean;
  disabled?: boolean;
  loading?: boolean;
  onChange?: (text: string) => void;
  onFocus?: (event: NativeSyntheticEvent<TextInputFocusEventData>) => void;
  onBlur?: (event: NativeSyntheticEvent<TextInputFocusEventData>) => void;
}

// Input root primitive - the main TextInput component
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
      placeholderTextColor = "#9ca3af",
      ...props
    },
    ref,
  ) => {
    const [isFocused, setIsFocused] = React.useState(false);

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
      (event: NativeSyntheticEvent<TextInputFocusEventData>) => {
        setIsFocused(true);
        if (onFocus) {
          onFocus(event);
        }
      },
      [onFocus],
    );

    const handleBlur = React.useCallback(
      (event: NativeSyntheticEvent<TextInputFocusEventData>) => {
        setIsFocused(false);
        if (onBlur) {
          onBlur(event);
        }
      },
      [onBlur],
    );

    return (
      <InputComponent
        ref={ref}
        value={value}
        onChangeText={handleChangeText}
        onFocus={handleFocus}
        onBlur={handleBlur}
        editable={!disabled && !loading && editable}
        placeholderTextColor={placeholderTextColor}
        style={[
          primitiveStyles.input,
          style,
          error && primitiveStyles.inputError,
          disabled && primitiveStyles.inputDisabled,
          loading && primitiveStyles.inputLoading,
        ]}
        {...props}
      />
    );
  },
);

InputRoot.displayName = "InputRoot";

// Input container primitive - wraps input with additional elements
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
    return (
      <View
        ref={ref}
        style={[
          primitiveStyles.container,
          style,
          focused && primitiveStyles.containerFocused,
          error && primitiveStyles.containerError,
          disabled && primitiveStyles.containerDisabled,
        ]}
        {...props}
      >
        {children}
      </View>
    );
  },
);

InputContainer.displayName = "InputContainer";

// Input label primitive
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
    return (
      <Text
        ref={ref}
        style={[
          primitiveStyles.label,
          style,
          error && primitiveStyles.labelError,
          disabled && primitiveStyles.labelDisabled,
        ]}
        {...props}
      >
        {children}
        {required && <Text style={primitiveStyles.required}> *</Text>}
      </Text>
    );
  },
);

InputLabel.displayName = "InputLabel";

// Input description/helper text primitive
export interface InputDescriptionProps extends TextProps {
  error?: boolean;
  disabled?: boolean;
}

export const InputDescription = forwardRef<Text, InputDescriptionProps>(
  ({ children, error = false, disabled = false, style, ...props }, ref) => {
    return (
      <Text
        ref={ref}
        style={[
          primitiveStyles.description,
          style,
          error && primitiveStyles.descriptionError,
          disabled && primitiveStyles.descriptionDisabled,
        ]}
        {...props}
      >
        {children}
      </Text>
    );
  },
);

InputDescription.displayName = "InputDescription";

// Input error message primitive
export interface InputErrorProps extends TextProps {
  visible?: boolean;
}

export const InputError = forwardRef<Text, InputErrorProps>(
  ({ children, visible = true, style, ...props }, ref) => {
    if (!visible || !children) return null;

    return (
      <Text ref={ref} style={[primitiveStyles.error, style]} {...props}>
        {children}
      </Text>
    );
  },
);

InputError.displayName = "InputError";

// Input addon primitive (for icons, buttons, etc.)
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
      primitiveStyles.addon,
      primitiveStyles[
        `addon${position.charAt(0).toUpperCase() + position.slice(1)}` as keyof typeof primitiveStyles
      ],
      style,
    ];

    if (touchable && onPress) {
      return (
        <TouchableOpacity
          ref={ref as React.Ref<React.ComponentRef<typeof TouchableOpacity>>}
          style={addonStyle as any}
          onPress={onPress}
          {...props}
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

// Input group primitive - groups label, input, description, error
export interface InputGroupProps extends ViewProps {
  spacing?: number;
}

export const InputGroup = forwardRef<View, InputGroupProps>(
  ({ children, spacing = 8, style, ...props }, ref) => {
    return (
      <View
        ref={ref}
        style={[primitiveStyles.group, { gap: spacing }, style]}
        {...props}
      >
        {children}
      </View>
    );
  },
);

InputGroup.displayName = "InputGroup";

// Primitive styles (minimal, unstyled)
const primitiveStyles = StyleSheet.create({
  input: {
    minHeight: 44, // iOS minimum touch target
    paddingHorizontal: 12,
    paddingVertical: 8,
    fontSize: 16,
    borderWidth: 1,
    borderColor: "#d1d5db",
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
  inputFocused: {
    // No focus styles for the actual input
  },
  inputError: {
    borderColor: "#ef4444",
    borderWidth: 1,
  },
  inputDisabled: {
    backgroundColor: "#f3f4f6",
    borderColor: "#e5e7eb",
    opacity: 0.6,
  },
  inputLoading: {
    opacity: 0.7,
  },
  container: {
    flexDirection: "row",
    alignItems: "center",
    borderWidth: 1,
    borderColor: "#d1d5db",
    borderRadius: 8,
    backgroundColor: "white",
    paddingHorizontal: 12,
    minHeight: 44,
  },
  containerFocused: {
    borderColor: "#3b82f6",
    borderWidth: 1,
  },
  containerError: {
    borderColor: "#ef4444",
    borderWidth: 1,
  },
  containerDisabled: {
    backgroundColor: "#f3f4f6",
    borderColor: "#e5e7eb",
    opacity: 0.6,
  },
  label: {
    fontSize: 14,
    fontWeight: "500",
    color: "#374151",
    marginBottom: 4,
  },
  labelError: {
    color: "#ef4444",
  },
  labelDisabled: {
    color: "#9ca3af",
    opacity: 0.6,
  },
  required: {
    color: "#ef4444",
  },
  description: {
    fontSize: 12,
    color: "#6b7280",
    marginTop: 4,
  },
  descriptionError: {
    color: "#ef4444",
  },
  descriptionDisabled: {
    color: "#9ca3af",
    opacity: 0.6,
  },
  error: {
    fontSize: 12,
    color: "#ef4444",
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

// Export primitive collection
export const InputPrimitive = {
  Root: InputRoot,
  Container: InputContainer,
  Label: InputLabel,
  Description: InputDescription,
  Error: InputError,
  Addon: InputAddon,
  Group: InputGroup,
};
