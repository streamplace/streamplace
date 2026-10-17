/**
 * @streamplace/i18n — platform-agnostic i18n data, config, and utilities.
 *
 * This package contains:
 *  - Locale manifest and metadata
 *  - Fluent (.ftl) source files and compiled JSON translations
 *  - Pure locale detection / fallback utilities
 *  - Base i18next configuration factory
 *
 * It does NOT contain React hooks, providers, or platform-specific loaders.
 * Those live in @streamplace/components (React Native) or the web app.
 */

// Manifest data
export { default as manifest } from "../locales/manifest.json";

// Locale utilities
export {
  cleanLocaleName,
  getFallbackChain,
  getLanguageInfo,
  getLocaleFromSystemLocale,
  getSupportedLocales,
  isLocaleSupported,
} from "./locale";
export type { LanguageInfo } from "./locale";

// Config factory
export {
  DEFAULT_NAMESPACE,
  NAMESPACES,
  STORAGE_KEY,
  createI18nextConfig,
} from "./config";
export type { I18nextConfigOptions } from "./config";

// Bundled locale resources (for native/SSR loaders)
export {
  enUSCommon,
  enUSSettings,
  esESCommon,
  esESSettings,
  frFRCommon,
  frFRSettings,
  ptBRCommon,
  ptBRSettings,
  roROCommon,
  roROSettings,
  zhHansCommon,
  zhHansSettings,
  zhHantCommon,
  zhHantSettings,
} from "./resources";
