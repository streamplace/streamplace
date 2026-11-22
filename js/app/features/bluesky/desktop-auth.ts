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

const TOKEN_KEY = "desktopLoginToken";
const PENDING_KEY = "desktopLoginPendingUntil";
// Matches the node's desktopReturnTTL.
const PENDING_MS = 10 * 60_000;

// The local node stores the OAuth response under this secret. It's kept in
// storage and never rotated: the OAuth client's redirect URI embeds it, so a
// reload mid-login and the code exchange afterwards must both see the same one.
export function getDesktopLoginToken(): string {
  let token = localStorage.getItem(TOKEN_KEY);
  if (!token) {
    token =
      typeof crypto !== "undefined" && "randomUUID" in crypto
        ? crypto.randomUUID()
        : Array.from({ length: 4 }, () =>
            Math.random().toString(36).slice(2),
          ).join("");
    localStorage.setItem(TOKEN_KEY, token);
  }
  return token;
}

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));
let polling = false;

// Polls the node for the browser's OAuth response while a login is pending,
// including one a previous page load started.
export async function pollDesktopLogin(
  nodeURL: string,
  complete: (url: string) => Promise<void>,
  start = false,
): Promise<void> {
  if (start) {
    localStorage.setItem(PENDING_KEY, String(Date.now() + PENDING_MS));
  }
  if (polling) return;
  polling = true;
  try {
    const token = getDesktopLoginToken();
    while (Number(localStorage.getItem(PENDING_KEY)) > Date.now()) {
      await sleep(1000);
      try {
        const res = await fetch(
          `${nodeURL}/api/desktop/oauth-callback/${token}`,
        );
        if (res.status === 404) continue;
        if (!res.ok) {
          throw new Error(`desktop login poll returned ${res.status}`);
        }
        const params = (await res.json()) as Record<string, string>;
        localStorage.removeItem(PENDING_KEY);
        await complete(
          `${DESKTOP_LOGIN_CALLBACK_PREFIX}?${new URLSearchParams(params).toString()}`,
        );
        return;
      } catch (e) {
        console.error("desktop login poll failed", e);
      }
    }
  } finally {
    polling = false;
  }
}
