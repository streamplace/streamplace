// Native translation loader - imports translations directly for bundling
// Metro will use this file for React Native builds

import {
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
} from "@streamplace/i18n";

const translationMap: Record<string, any> = {
  "en-US/common": enUSCommon,
  "en-US/settings": enUSSettings,
  "pt-BR/common": ptBRCommon,
  "pt-BR/settings": ptBRSettings,
  "es-ES/common": esESCommon,
  "es-ES/settings": esESSettings,
  "zh-Hans/common": zhHansCommon,
  "zh-Hans/settings": zhHansSettings,
  "zh-Hant/common": zhHantCommon,
  "zh-Hant/settings": zhHantSettings,
  "fr-FR/common": frFRCommon,
  "fr-FR/settings": frFRSettings,
  "ro-RO/common": roROCommon,
  "ro-RO/settings": roROSettings,
};

export async function loadTranslationData(
  locale: string,
  namespace: string,
): Promise<any> {
  // Map base language codes to full locales
  const fullLocale = locale.includes("-")
    ? locale
    : {
        en: "en-US",
        pt: "pt-BR",
        es: "es-ES",
        zh: "zh-Hant",
        fr: "fr-FR",
        ro: "ro-RO",
      }[locale] || locale;

  const localeNamespaceKey = `${fullLocale}/${namespace}`;
  const translations = translationMap[localeNamespaceKey];

  if (!translations) {
    throw new Error(`No translation mapping for ${localeNamespaceKey}`);
  }

  if (!translations || Object.keys(translations).length === 0) {
    throw new Error("No translations found");
  }

  return translations;
}
