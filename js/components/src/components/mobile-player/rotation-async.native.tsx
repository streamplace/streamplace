let ScreenOrientation: typeof import("expo-screen-orientation") | null = null;

try {
  ScreenOrientation = require("expo-screen-orientation");
} catch {
  // expo-screen-orientation not available
  if (__DEV__) {
    console.warn(
      "expo-screen-orientation not installed, rotation features disabled",
    );
  }
}

export const isRotationAvailable = ScreenOrientation != null;
export { ScreenOrientation };
