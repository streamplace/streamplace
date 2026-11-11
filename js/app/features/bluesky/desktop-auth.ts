// Desktop's preload exposes this; it is absent in a normal browser or on
// native. Keep DESKTOP_LOGIN_SCHEME in sync with js/desktop/src/index.ts.
export const DESKTOP_LOGIN_SCHEME = "streamplace-desktop";

export type DesktopAuth = {
  openLogin: (url: string) => Promise<void>;
  onCallback: (callback: (url: string) => void) => () => void;
};

declare global {
  interface Window {
    SP_ELECTRON?: {
      isElectron: boolean;
      auth?: DesktopAuth;
    };
  }
}

export function getDesktopAuth(): DesktopAuth | undefined {
  return typeof window === "undefined" ? undefined : window.SP_ELECTRON?.auth;
}
