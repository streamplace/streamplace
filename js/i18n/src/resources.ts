/**
 * Bundled locale resources for native/SSR consumption.
 *
 * React Native (Metro) statically imports JSON so it's bundled into the app.
 * For web, use the HTTP loader with the JSON files served from public/locales/.
 *
 * This module re-exports every compiled translation JSON so that
 * `@streamplace/i18n/resources` can be imported by platform-specific loaders.
 */

// en-US
export { default as enUSCommon } from "../public/locales/en-US/common.json";
export { default as enUSSettings } from "../public/locales/en-US/settings.json";

// es-ES
export { default as esESCommon } from "../public/locales/es-ES/common.json";
export { default as esESSettings } from "../public/locales/es-ES/settings.json";

// fr-FR
export { default as frFRCommon } from "../public/locales/fr-FR/common.json";
export { default as frFRSettings } from "../public/locales/fr-FR/settings.json";

// pt-BR
export { default as ptBRCommon } from "../public/locales/pt-BR/common.json";
export { default as ptBRSettings } from "../public/locales/pt-BR/settings.json";

// ro-RO
export { default as roROCommon } from "../public/locales/ro-RO/common.json";
export { default as roROSettings } from "../public/locales/ro-RO/settings.json";

// zh-Hans
export { default as zhHansCommon } from "../public/locales/zh-Hans/common.json";
export { default as zhHansSettings } from "../public/locales/zh-Hans/settings.json";

// zh-Hant
export { default as zhHantCommon } from "../public/locales/zh-Hant/common.json";
export { default as zhHantSettings } from "../public/locales/zh-Hant/settings.json";
