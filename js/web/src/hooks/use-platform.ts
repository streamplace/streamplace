// Port of js/app/hooks/usePlatform.web.tsx. Returns an IsPlatform-shaped
// object describing the runtime. For web, isNative is always false;
// isWeb is always true; UA-derived flags drive isWebAndroid/isWebIOS/
// isSafari/etc.
//
// Cached UA parse so the hook is cheap to call from many components.

interface IsPlatform {
  isNative: boolean;
  isIOS: boolean;
  isAndroid: boolean;
  isWeb: boolean;
  isWebAndroid: boolean;
  isWebIOS: boolean;
  isElectron: boolean;
  isBrowser: boolean;
  isSafari: boolean;
  isChrome: boolean;
  isFirefox: boolean;
  isMobileSafari: boolean;
  topSafeHeight: () => number;
}

interface UAResult {
  browser: { name: string };
  os: { name: string };
}

let cached: UAResult | null = null;

function getUA(): UAResult {
  if (!cached) {
    const ua = typeof navigator !== "undefined" ? navigator.userAgent : "";
    // Minimal UA parser — only the fields we actually use. Avoids the
    // ua-parser-js dependency that the app's web variant pulls in.
    cached = {
      browser: {
        name: /Edg\//.test(ua)
          ? "Edge"
          : /Chrome\//.test(ua)
            ? "Chrome"
            : /Firefox\//.test(ua)
              ? "Firefox"
              : /Safari\//.test(ua)
                ? "Safari"
                : "Unknown",
      },
      os: { name: "" },
    };
  }
  return cached;
}

export default function usePlatform(): IsPlatform {
  const ua = getUA();
  const electron =
    typeof window !== "undefined" && (window as any).SP_ELECTRON !== undefined;
  const userAgent = typeof navigator !== "undefined" ? navigator.userAgent : "";
  return {
    isNative: false,
    isIOS: false,
    isAndroid: false,
    isWeb: true,
    isWebAndroid: /Android/.test(userAgent),
    isWebIOS: /iPhone|iPad|iPod/.test(userAgent),
    isElectron: electron,
    isBrowser: !electron,
    isSafari: ua.browser.name.includes("Safari"),
    isChrome: ua.browser.name === "Chrome",
    isFirefox: ua.browser.name === "Firefox",
    isMobileSafari:
      /Safari/.test(userAgent) && !/CriOS|FxiOS|EdgiOS/.test(userAgent),
    topSafeHeight: () => 0,
  };
}
