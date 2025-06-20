import { Platform } from "react-native";
import { useKeyboard } from "../hooks/useKeyboard";
import { useOuterAndInnerDimensions } from "../hooks/useOuterAndInnerDimensions";

export function useKeyboardSlide() {
  const { keyboardHeight } = useKeyboard();
  const { outerHeight, innerHeight } = useOuterAndInnerDimensions();
  let slideKeyboard = 0;
  if (Platform.OS === "ios" && keyboardHeight > 0) {
    slideKeyboard = -keyboardHeight + (outerHeight - innerHeight);
  }
  return { keyboardHeight, slideKeyboard };
}
