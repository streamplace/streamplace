export type IsPlatform = {
  isNative: boolean;
  isIOS: boolean;
  isAndroid: boolean;

  isWeb: boolean;
  isWebAndroid: boolean;
  isWebIOS: boolean;

  isElectron: boolean;
  isBrowser: boolean;

  // don't rely on this! just for defaults
  isSafari: boolean;
  // don't rely on this! just for defaults
  isChrome: boolean;
  // don't rely on this! just for defaults
  isFirefox: boolean;

  // True for Mobile Safari, which already shows a native app banner,
  // so we skip our custom one.
  isMobileSafari: boolean;

  topSafeHeight: () => number;
};
