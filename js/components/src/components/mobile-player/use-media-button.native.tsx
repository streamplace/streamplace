import { useEffect } from "react";
import { DeviceEventEmitter } from "react-native";

export function useMediaButton(onPress: () => void) {
  useEffect(() => {
    const subscription = DeviceEventEmitter.addListener(
      "mediaButtonPress",
      () => {
        onPress();
      },
    );
    return () => {
      subscription.remove();
    };
  }, [onPress]);
}
