// Simple i18n infrastructure exports for Streamplace components
// Re-exports i18next tools for app-level customization

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

// Bootstrap configuration and utilities
export {
  I18NEXT_CONFIG,
  changeLanguage,
  getCurrentLanguage,
  getCurrentLocale,
  getLanguageInfo,
  getLocaleFromSystemLocale,
  getSupportedLocales,
  i18next,
  default as initI18next,
  isLocaleSupported,
} from "./i18next-config";

// Manifest data
export { default as manifest } from "../../locales/manifest.json";

// TypeScript types
export type {
  i18n as I18nInstance,
  InitOptions,
  Resource,
  ResourceLanguage,
  TFunction,
} from "i18next";
