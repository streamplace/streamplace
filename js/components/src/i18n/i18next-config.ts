// i18next configuration for React Native / Expo
// Uses @streamplace/i18n for manifest, config, and locale utilities.
// This file adds React plugin + platform-specific loaders + storage persistence.

import i18next from "i18next";
import Fluent from "i18next-fluent";
import resourcesToBackend from "i18next-resources-to-backend";
import "intl-pluralrules";
import { initReactI18next } from "react-i18next";

import {
  getLocaleFromSystemLocale as baseGetLocaleFromSystemLocale,
  createI18nextConfig,
  manifest,
  STORAGE_KEY,
} from "@streamplace/i18n";
import storage from "../storage";

// Try to import expo-localization, but make it optional
let Localization: typeof import("expo-localization") | null = null;
try {
  const localizationModule = require("expo-localization");
  Localization = localizationModule.default
    ? localizationModule.default
    : localizationModule;
} catch {
  // expo-localization not available, will use browser/fallback detection
}

/**
 * Detect the system locale using expo-localization (if available),
 * falling back to the platform-agnostic detection from @streamplace/i18n.
 */
export function getLocaleFromSystemLocale(): string {
  if (Localization && typeof Localization.getLocales === "function") {
    try {
      const locales = Localization.getLocales();
      const deviceLocale = locales?.[0]?.languageTag;
      if (deviceLocale) {
        return baseGetLocaleFromSystemLocale(deviceLocale);
      }
    } catch (error) {
      console.warn("Failed to get locales from expo-localization:", error);
    }
  }

  return baseGetLocaleFromSystemLocale();
}

// Cache for the current locale to avoid async lookups
let cachedLocale: string | null = null;

export async function getCurrentLocale(): Promise<string> {
  if (cachedLocale) {
    return cachedLocale;
  }

  const stored = await storage.getItem(STORAGE_KEY);
  if (stored && manifest.supportedLocales.includes(stored)) {
    cachedLocale = stored;
    return stored;
  }

  const systemLocale = getLocaleFromSystemLocale();
  cachedLocale = systemLocale;
  return systemLocale;
}

// Synchronous version for initial load - returns cached or system locale
export function getCurrentLocaleSync(): string {
  return cachedLocale || getLocaleFromSystemLocale();
}

// Use sync version for initial config - will be updated when storage loads
const LOCALE = getCurrentLocaleSync();

export const I18NEXT_CONFIG = createI18nextConfig({ lng: LOCALE });

// Import platform-specific translation loader
import { loadTranslationData as platformLoadTranslationData } from "./i18n-loader";

// Translation loading function with error handling
async function loadTranslationData(
  locale: string,
  namespace: string,
): Promise<any> {
  try {
    return await platformLoadTranslationData(locale, namespace);
  } catch (error: any) {
    console.error(
      `Failed to load ${namespace} translations for ${locale}:`,
      error,
    );
    return { loading: "Loading...", error: "Error", cancel: "Cancel" };
  }
}

// Initialize i18next with our configuration
let initPromise: Promise<typeof i18next> | null = null;

export default async function initI18next(
  config: any = {},
): Promise<typeof i18next> {
  if (initPromise) return initPromise;

  const storedLocale = await getCurrentLocale();

  const finalConfig = {
    ...I18NEXT_CONFIG,
    lng: storedLocale,
    ...config,
  };

  initPromise = i18next
    .use(initReactI18next)
    .use(Fluent)
    .use(
      resourcesToBackend((locale: string, namespace: string, callback: any) => {
        loadTranslationData(locale, namespace)
          .then((translations) => callback(null, translations))
          .catch((error) => callback(error, null));
      }),
    )
    .init(finalConfig)
    .then(() => {
      // Persist language changes to storage
      i18next.on("languageChanged", (lng) => {
        if (lng && manifest.supportedLocales.includes(lng)) {
          cachedLocale = lng;
          storage.setItem(STORAGE_KEY, lng);
        }
      });
      return i18next;
    });

  return initPromise;
}

// Utility functions for language management
export async function changeLanguage(locale: string): Promise<void> {
  await i18next.changeLanguage(locale);
}

export function getCurrentLanguage(): string {
  return getCurrentLocaleSync();
}

// Re-export pure utilities from @streamplace/i18n for backwards compat
export {
  getFallbackChain,
  getLanguageInfo,
  getSupportedLocales,
  isLocaleSupported,
} from "@streamplace/i18n";

export { manifest };

export { i18next };
