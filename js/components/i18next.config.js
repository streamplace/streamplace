import { defineConfig } from "i18next-cli";
// i18next-cli configuration
// We use i18next-fluent, so variant handling (platform, count, etc.)
// is done using Fluent's select expressions within messages.
// Example: shortcut-key-search = { $platform -> [mac] Cmd+K [windows] Ctrl+K *[other] Ctrl+K }

export default defineConfig({
  locales: ["en-US", "es-ES", "fr-FR", "pt-BR", "zh-Hant"],
  extract: {
    input: [
      "src/**/*.{js,jsx,ts,tsx}",
      "../app/src/**/*.{js,jsx,ts,tsx}",
      "../app/components/**/*.{js,jsx,ts,tsx}",
    ],
    contextSeparator: "|",
    pluralSeparator: "/",
    output: "public/locales/{{language}}/{{namespace}}.json",
    primaryLanguage: "en-US",
    defaultNS: "common",
  },
});
