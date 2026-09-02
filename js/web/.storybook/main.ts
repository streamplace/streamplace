import type { StorybookConfig } from "@storybook/react-vite";

import { dirname } from "path";
import { fileURLToPath } from "url";

function getAbsolutePath(value: string) {
  return dirname(fileURLToPath(import.meta.resolve(`${value}/package.json`)));
}

const config: StorybookConfig = {
  stories: ["../src/**/*.mdx", "../src/**/*.stories.@(js|jsx|mjs|ts|tsx)"],
  addons: [
    getAbsolutePath("@storybook/addon-a11y"),
    getAbsolutePath("@storybook/addon-docs"),
  ],
  framework: getAbsolutePath("@storybook/react-vite"),
  viteConfig: async () => {
    const { default: viteConfig } = await import("../vite.config.ts");
    return typeof viteConfig === "function"
      ? viteConfig({ mode: "development", command: "serve" })
      : viteConfig;
  },
};

export default config;
