import { Platform } from "react-native";
import { initPushNotifications, topSafeHeight } from "./platform";
import { IsPlatform } from "./usePlatform.shared";

export default function usePlatform(): IsPlatform {
  return {
    initPushNotifications,
    topSafeHeight,
    isNative: true,
    isIOS: Platform.OS === "ios",
    isAndroid: Platform.OS === "android",
    isWeb: false,
    isElectron: false,
    isBrowser: false,
    isSafari: false,
    isChrome: false,
    isFirefox: false,
  };
}
