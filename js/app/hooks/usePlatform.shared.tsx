export type IsPlatform = {
  isNative: boolean;
  isIOS: boolean;
  isAndroid: boolean;

  isWeb: boolean;
  isElectron: boolean;
  isBrowser: boolean;

  // don't rely on this! just for defaults
  isSafari: boolean;
  // don't rely on this! just for defaults
  isChrome: boolean;
  // don't rely on this! just for defaults
  isFirefox: boolean;

  topSafeHeight: () => number;
};
