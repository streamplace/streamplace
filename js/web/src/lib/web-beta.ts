// Cookie-based opt-in to the new Vite web app. The server reads
// `sp_web_beta=1` and serves the new frontend; without it (or any other
// value) the legacy RN app is served. Toggling the cookie in the
// advanced settings UI sets or clears it and reloads the page so the
// server picks up the new choice.

const COOKIE_NAME = "sp_web_beta";
// Long, but not "forever". A year is enough that the average user
// doesn't have to think about it but not so long the toggle becomes
// effectively permanent.
const ONE_YEAR_SECONDS = 60 * 60 * 24 * 365;

const isBrowser = (): boolean => typeof document !== "undefined";

export function isWebBetaEnabled(): boolean {
  if (!isBrowser()) return false;
  const cookies = document.cookie.split(";");
  for (const raw of cookies) {
    const [name, ...rest] = raw.trim().split("=");
    if (name === COOKIE_NAME) {
      return rest.join("=") === "1";
    }
  }
  return false;
}

export function setWebBetaEnabled(enabled: boolean): void {
  if (!isBrowser()) return;
  if (enabled) {
    document.cookie = `${COOKIE_NAME}=1; path=/; max-age=${ONE_YEAR_SECONDS}; SameSite=Lax`;
  } else {
    document.cookie = `${COOKIE_NAME}=; path=/; max-age=0; SameSite=Lax`;
  }
}

export const WEB_BETA_COOKIE = COOKIE_NAME;
