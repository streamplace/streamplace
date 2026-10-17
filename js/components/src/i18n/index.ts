// i18n infrastructure exports for Streamplace components
// Re-exports from @streamplace/i18n + React-specific bindings

// Core i18next exports
export { default as i18n } from "i18next";
export {
  I18nextProvider,
  Trans,
  Translation,
  initReactI18next,
  useTranslation,
  withTranslation,
} from "react-i18next";

// i18next plugins for common use cases
export { default as LanguageDetector } from "i18next-browser-languagedetector";
export { default as Backend } from "i18next-http-backend";
export { default as resourcesToBackend } from "i18next-resources-to-backend";

// Fluent support
export { default as Fluent } from "i18next-fluent";

// Basic provider components for consistent setup
export { I18nProvider } from "./provider";

// Bootstrap configuration and utilities (local — adds React + storage)
export {
  I18NEXT_CONFIG,
  changeLanguage,
  getCurrentLanguage,
  getCurrentLocale,
  getCurrentLocaleSync,
  getLocaleFromSystemLocale,
  i18next,
  default as initI18next,
} from "./i18next-config";

// Re-export pure utilities from @streamplace/i18n
export {
  STORAGE_KEY,
  createI18nextConfig,
  getFallbackChain,
  getLanguageInfo,
  getSupportedLocales,
  isLocaleSupported,
  manifest,
} from "@streamplace/i18n";

// TypeScript types
export type {
  i18n as I18nInstance,
  InitOptions,
  Resource,
  ResourceLanguage,
  TFunction,
} from "i18next";
