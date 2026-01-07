import {
  BottomSheetTextInput,
  useBottomSheetInternal,
} from "@gorhom/bottom-sheet";
import * as React from "react";
import { Platform, TextInput, type TextInputProps } from "react-native";
import { bg, borders, flex, p, text } from "../../lib/theme/atoms";

const Textarea = React.forwardRef<TextInput, TextInputProps>(
  ({ style, multiline = true, numberOfLines = 4, ...props }, ref) => {
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
          borders.width.thin,
          borders.color.gray[400],
          bg.gray[900],
          p[3],
          text.gray[200],
          props.editable === false && { opacity: 0.5 },
          { borderRadius: 10 },
          style,
        ]}
        multiline={multiline}
        numberOfLines={numberOfLines}
        textAlignVertical="top"
        {...props}
      />
    );
  },
);

Textarea.displayName = "Textarea";

export { Textarea };
