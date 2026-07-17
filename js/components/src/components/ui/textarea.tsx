import {
  BottomSheetTextInput,
  useBottomSheetInternal,
} from "@gorhom/bottom-sheet";
import * as React from "react";
import { Platform, TextInput, type TextInputProps } from "react-native";
import { flex, p } from "../../lib/theme/atoms";
import { borderRadius, fontFamilies, typeScale } from "../../lib/theme/tokens";
import { useTheme } from "../../ui";

const Textarea = React.forwardRef<TextInput, TextInputProps>(
  ({ style, multiline = true, numberOfLines = 4, ...props }, ref) => {
    let th = useTheme();
    const [focused, setFocused] = React.useState(false);
    // Detect if we're inside a bottom sheet
    let isInBottomSheet = false;
    try {
      useBottomSheetInternal();
      isInBottomSheet = true;
    } catch {
      // Not in a bottom sheet context
      isInBottomSheet = false;
    }

    // Use BottomSheetTextInput when inside a bottom sheet, regular TextInput otherwise
    const InputComponent =
      isInBottomSheet && Platform.OS !== "web"
        ? BottomSheetTextInput
        : TextInput;

    return (
      <InputComponent
        ref={ref as any}
        style={[
          flex.values[1],
          p[3],
          {
            borderWidth: 1,
            borderColor: focused
              ? th.theme.colors.ring
              : th.theme.colors.border,
            backgroundColor: th.theme.colors.input,
            color: th.theme.colors.text1,
            borderRadius: borderRadius.md,
            // ≥16px so mobile web browsers don't zoom on focus
            fontSize: typeScale.md.fontSize,
            fontFamily: fontFamilies.regular,
          },
          props.editable === false && { opacity: 0.5 },
          style,
        ]}
        autoComplete={props.autoComplete || "off"}
        textContentType={props.textContentType || "none"}
        multiline={multiline}
        numberOfLines={numberOfLines}
        textAlignVertical="top"
        placeholderTextColor={th.theme.colors.text3}
        {...props}
        onFocus={(e) => {
          setFocused(true);
          props.onFocus?.(e);
        }}
        onBlur={(e) => {
          setFocused(false);
          props.onBlur?.(e);
        }}
      />
    );
  },
);

Textarea.displayName = "Textarea";

export { Textarea };
