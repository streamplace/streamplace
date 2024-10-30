import { IsPlatform } from "./usePlatform.shared";
// it's only for setting defaults, i promise!
import uaParser from "ua-parser-js";
import { initPushNotifications, topSafeHeight } from "./platform";

function supportsHLS() {
  var video = document.createElement("video");
  return Boolean(
    video.canPlayType("application/vnd.apple.mpegURL") ||
      video.canPlayType("audio/mpegurl"),
  );
}

let ua;

export default function usePlatform(): IsPlatform {
  if (!ua) {
    ua = uaParser(navigator.userAgent);
  }
  const electron = typeof window["AQ_ELECTRON"] !== "undefined";
  return {
    initPushNotifications,
    topSafeHeight,
    isNative: false,
    isIOS: false,
    isAndroid: false,
    isWeb: true,
    isElectron: electron,
    isBrowser: !electron,
    isSafari: ua.browser.name === "Safari",
    isFirefox: ua.browser.name === "Firefox",
    isChrome: ua.browser.name === "Chrome",
  };
}
