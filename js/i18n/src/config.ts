/**
 * Base i18next configuration — no React plugin, no platform-specific loaders.
 *
 * Consumers (js/components, js/web) should:
 *  1. Call createI18nextConfig() to get the base config
 *  2. Add their own plugins (initReactI18next, Fluent, resourcesToBackend)
 *  3. Wire up platform-specific translation loaders
 *  4. Call i18next.init(config)
 */

import manifest from "../locales/manifest.json";
import { getFallbackChain } from "./locale";

/** Storage key for persisting the user's locale preference. */
export const STORAGE_KEY = "@streamplace/locale";

/** i18next namespace names. */
export const NAMESPACES = ["common", "settings"] as const;

/** Default namespace used when none is specified. */
export const DEFAULT_NAMESPACE = "common";

export interface I18nextConfigOptions {
  /** Override the initial language. Defaults to the manifest fallback (en-US). */
  lng?: string;
  /** Enable i18next debug logging. Defaults to NODE_ENV === "development". */
  debug?: boolean;
  /** Any additional i18next init options (merged last, can override). */
  [key: string]: any;
}

/**
 * Create a base i18next configuration object.
 *
 * The returned object is ready for `i18next.init()` after the consumer
 * adds their own plugins (React, Fluent, loaders, etc.).
 */
export function createI18nextConfig(options?: I18nextConfigOptions) {
  const { lng, debug, ...rest } = options ?? {};

  return {
    lng: lng ?? manifest.fallbackChain[0],
    ns: [...NAMESPACES],
    defaultNS: DEFAULT_NAMESPACE,
    interpolation: {
      escapeValue: false, // React already escapes
    },
    react: {
      useSuspense: false, // Prevent Android crashes
    },
    i18nFormat: {
      fluentBundleOptions: {
        useIsolating: false,
        functions: {
          VOWORCON: ([txt]: [string]) =>
            "aeiou".indexOf(txt[0].toLowerCase()) >= 0 ? "vow" : "con",
          JOIN: (args: string[], opts: { separator?: string } = {}) =>
            args
              .filter(Boolean)
              .filter((s) => typeof s === "string")
              .join(opts.separator || ""),
        },
      },
    },
    load: "currentOnly",
    cleanCode: true,
    fallbackLng: getFallbackChain,
    supportedLngs: [...manifest.supportedLocales],
    debug:
      debug ??
      (typeof process !== "undefined" &&
        process.env?.NODE_ENV === "development"),
    ...rest,
  };
}
