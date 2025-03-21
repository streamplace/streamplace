import { useEffect, useState } from "react";
import { Keyboard } from "react-native";

export function useKeyboard() {
  const [isKeyboardVisible, setIsKeyboardVisible] = useState(false);
  const [keyboardHeight, setKeyboardHeight] = useState(0);
  useEffect(() => {
    const willShowSubscription = Keyboard.addListener(
      "keyboardWillShow",
      (e) => {
        // setIsKeyboardVisible(true);
        setKeyboardHeight(e.endCoordinates.height);
        console.log("keyboardWillShow", e.endCoordinates.height);
      },
    );
    const willHideSubscription = Keyboard.addListener(
      "keyboardWillHide",
      (e) => {
        // setIsKeyboardVisible(false);
        setKeyboardHeight(0);
        console.log("keyboardWillHide", e.endCoordinates.height);
      },
    );
    const showSubscription = Keyboard.addListener("keyboardDidShow", (e) => {
      setIsKeyboardVisible(true);
      setKeyboardHeight(e.endCoordinates.height);
      console.log("keyboardDidShow", e.endCoordinates.height);
    });
    const hideSubscription = Keyboard.addListener("keyboardDidHide", () => {
      setIsKeyboardVisible(false);
      setKeyboardHeight(0);
    });
    return () => {
      showSubscription.remove();
      hideSubscription.remove();
      willShowSubscription.remove();
    };
  }, []);

  return { isKeyboardVisible, keyboardHeight };
}
