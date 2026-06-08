// Resolution order: localStorage override > VITE_STREAMPLACE_URL env > window.location.origin.
// Changes to localStorage take effect on the next page reload.

const ENV_KEY = "VITE_STREAMPLACE_URL";
export const SERVER_URL_STORAGE_KEY = "streamplace:server-url";

export function getStoredServerUrl(): string | null {
  if (typeof localStorage === "undefined") return null;
  const v = localStorage.getItem(SERVER_URL_STORAGE_KEY);
  if (typeof v !== "string") return null;
  const trimmed = v.trim().replace(/\/+$/, "");
  return trimmed.length > 0 ? trimmed : null;
}

export function setStoredServerUrl(url: string): void {
  const trimmed = url.trim().replace(/\/+$/, "");
  if (trimmed.length === 0) {
    localStorage.removeItem(SERVER_URL_STORAGE_KEY);
  } else {
    localStorage.setItem(SERVER_URL_STORAGE_KEY, trimmed);
  }
}

export function clearStoredServerUrl(): void {
  if (typeof localStorage !== "undefined") {
    localStorage.removeItem(SERVER_URL_STORAGE_KEY);
  }
}

export function getStreamplaceUrl(): string {
  const stored = getStoredServerUrl();
  if (stored) return stored;

  const fromEnv = import.meta.env[ENV_KEY];
  if (typeof fromEnv === "string" && fromEnv.length > 0) {
    return fromEnv.replace(/\/+$/, "");
  }

  if (typeof window !== "undefined" && window.location?.origin) {
    return window.location.origin.replace(/\/+$/, "");
  }

  throw new Error(
    "Could not determine streamplaceUrl. Set VITE_STREAMPLACE_URL or run the web app from a browser.",
  );
}
