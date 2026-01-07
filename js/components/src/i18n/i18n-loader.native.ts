// Native translation loader - imports translations directly for bundling
// Metro will use this file for React Native builds

// Import all translations directly so they're bundled into the app
import enUSCommon from "../../public/locales/en-US/common.json";
import enUSSettings from "../../public/locales/en-US/settings.json";
import esESCommon from "../../public/locales/es-ES/common.json";
import esESSettings from "../../public/locales/es-ES/settings.json";
import frFRCommon from "../../public/locales/fr-FR/common.json";
import frFRSettings from "../../public/locales/fr-FR/settings.json";
import ptBRCommon from "../../public/locales/pt-BR/common.json";
import ptBRSettings from "../../public/locales/pt-BR/settings.json";
import zhHantCommon from "../../public/locales/zh-Hant/common.json";
import zhHantSettings from "../../public/locales/zh-Hant/settings.json";

const translationMap: Record<string, any> = {
  "en-US/common": enUSCommon,
  "en-US/settings": enUSSettings,
  "pt-BR/common": ptBRCommon,
  "pt-BR/settings": ptBRSettings,
  "es-ES/common": esESCommon,
  "es-ES/settings": esESSettings,
  "zh-Hant/common": zhHantCommon,
  "zh-Hant/settings": zhHantSettings,
  "fr-FR/common": frFRCommon,
  "fr-FR/settings": frFRSettings,
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
