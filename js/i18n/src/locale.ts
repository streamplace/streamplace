/**
 * Platform-agnostic locale utilities.
 * No React, no platform-specific APIs — pure data + logic.
 */

import manifest from "../locales/manifest.json";

export interface LanguageInfo {
  code: string;
  name: string;
  nativeName: string;
  flag: string;
}

/**
 * Normalize a locale string: underscores → dashes, strip @ modifiers.
 */
export function cleanLocaleName(locale: string): string {
  return locale.replace("_", "-").replace(/@.*/, "");
}

/**
 * Build a fallback chain for a given locale code.
 * Regional variants (e.g. pt-PT) fall back to their base locale (pt-BR),
 * then to the manifest's fallback chain (typically en-US).
 */
export function getFallbackChain(code: string): string[] {
  const fallbacks: string[] = [];

  if (!code) return [...manifest.fallbackChain];

  if (code.match(/^es-/)) {
    fallbacks.push("es-ES");
  } else if (code.match(/^fr-/)) {
    fallbacks.push("fr-FR");
  } else if (code.match(/^pt-/)) {
    fallbacks.push("pt-BR");
  } else if (code.match(/^zh-/)) {
    fallbacks.push("zh-Hant");
  }

  return [...fallbacks, ...manifest.fallbackChain];
}

/**
 * Get all supported locale codes from the manifest.
 */
export function getSupportedLocales(): string[] {
  return [...manifest.supportedLocales];
}

/**
 * Get language metadata for a locale code, or null if unsupported.
 */
export function getLanguageInfo(locale: string): LanguageInfo | null {
  return (manifest.languages as Record<string, LanguageInfo>)[locale] || null;
}

/**
 * Check whether a locale code is in the supported set.
 */
export function isLocaleSupported(locale: string): boolean {
  return manifest.supportedLocales.includes(locale);
}

/**
 * Detect the best-matching locale from a system locale string.
 *
 * Platform-agnostic: passes through the `override` if provided,
 * otherwise falls back to `navigator.language` (web) or "en".
 *
 * For React Native with expo-localization, call with the detected locale:
 *   getLocaleFromSystemLocale(deviceLocale)
 */
export function getLocaleFromSystemLocale(override?: string): string {
  let systemLocale = override || "en";

  if (!override && typeof navigator !== "undefined" && navigator.language) {
    systemLocale = navigator.language;
  }

  const candidate = cleanLocaleName(systemLocale);

  // Exact match (e.g. "en-US")
  if (manifest.supportedLocales.includes(candidate)) {
    return candidate;
  }

  // Language-prefix match (e.g. "en-GB" → "en-US")
  const lang = candidate.split("-")[0];
  const matching = manifest.supportedLocales.find((l) =>
    l.startsWith(lang + "-"),
  );
  if (matching) return matching;

  // Fall back to default
  return manifest.fallbackChain[0];
}
