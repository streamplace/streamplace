// Color scheme provider. Reads persisted preference from localStorage,
// listens to system prefers-color-scheme for "system" mode, and
// syncs the `.dark` class on <html>.
const STORAGE_KEY = "streamplace:theme";

export type ThemePreference = "light" | "dark" | "system";

function getSystemScheme(): "light" | "dark" {
  if (typeof window === "undefined") return "dark";
  return window.matchMedia("(prefers-color-scheme: dark)").matches
    ? "dark"
    : "light";
}

export function getStoredPreference(): ThemePreference {
  if (typeof localStorage === "undefined") return "system";
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored === "light" || stored === "dark" || stored === "system") {
    return stored;
  }
  return "system";
}

export function getEffectiveScheme(
  preference?: ThemePreference,
): "light" | "dark" {
  const pref = preference ?? getStoredPreference();
  if (pref === "system") return getSystemScheme();
  return pref;
}

export function setThemePreference(pref: ThemePreference) {
  localStorage.setItem(STORAGE_KEY, pref);
  syncThemeClass(pref);
}

export function syncThemeClass(preference?: ThemePreference) {
  const scheme = getEffectiveScheme(preference);
  const root = document.documentElement;
  if (scheme === "dark") {
    root.classList.add("dark");
  } else {
    root.classList.remove("dark");
  }
}
