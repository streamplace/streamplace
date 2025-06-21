import { IsPlatform } from "./usePlatform.shared";
// it's only for setting defaults, i promise!
import uaParser from "ua-parser-js";
import { topSafeHeight } from "./platform";

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
    const parser = uaParser.UAParser;
    ua = parser(navigator.userAgent);
  }
  const electron = typeof window["SP_ELECTRON"] !== "undefined";
  return {
    topSafeHeight,
    isNative: false,
    isIOS: false,
    isAndroid: false,
    isWeb: true,
    isElectron: electron,
    isBrowser: !electron,
    isSafari: ua.browser.name.includes("Safari"),
    isFirefox: ua.browser.name === "Firefox",
    isChrome: ua.browser.name === "Chrome",
  };
}
