import {
  createI18nextConfig,
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
import i18next from "i18next";
import i18nextFluent from "i18next-fluent";
import { initReactI18next } from "react-i18next";

const resources = {
  "en-US": { common: enUSCommon, settings: enUSSettings },
  "es-ES": { common: esESCommon, settings: esESSettings },
  "fr-FR": { common: frFRCommon, settings: frFRSettings },
  "pt-BR": { common: ptBRCommon, settings: ptBRSettings },
  "ro-RO": { common: roROCommon, settings: roROSettings },
  "zh-Hans": { common: zhHansCommon, settings: zhHansSettings },
  "zh-Hant": { common: zhHantCommon, settings: zhHantSettings },
};

const baseConfig = createI18nextConfig();

i18next
  .use(initReactI18next)
  .use(i18nextFluent)
  .init({
    ...baseConfig,
    resources,
  } as Parameters<typeof i18next.init>[0]);

export default i18next;
