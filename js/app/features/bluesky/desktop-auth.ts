// Desktop's preload exposes this; it is absent in a normal browser or on
// native. Desktop logins return through the local node, which holds the
// OAuth response under a per-session token.
export const DESKTOP_LOGIN_CALLBACK_PREFIX = "streamplace-desktop:/";

export type DesktopAuth = {
  openLogin: (url: string) => Promise<void>;
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

let desktopLoginToken: string | undefined;

// One secret per renderer session: the local node stores the OAuth response
// under it and the app polls for it, so no custom scheme is needed.
export function getDesktopLoginToken(): string {
  if (!desktopLoginToken) {
    desktopLoginToken =
      typeof crypto !== "undefined" && "randomUUID" in crypto
        ? crypto.randomUUID()
        : Array.from({ length: 4 }, () =>
            Math.random().toString(36).slice(2),
          ).join("");
  }
  return desktopLoginToken;
}
