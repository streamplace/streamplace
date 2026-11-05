import { resolve } from "node:path";
import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: {
    alias: {
      "@streamplace/components/src": resolve(import.meta.dirname, "src"),
      "@streamplace/core": resolve(import.meta.dirname, "../core/src"),
      "@streamplace/i18n": resolve(import.meta.dirname, "../i18n/src"),
      hooks: resolve(import.meta.dirname, "../app/hooks"),
      store: resolve(import.meta.dirname, "../app/store"),
      components: resolve(import.meta.dirname, "../app/components"),
      utils: resolve(import.meta.dirname, "../app/utils"),
    },
  },
  test: {
    environment: "jsdom",
    include: ["tests/**/*.test.tsx"],
  },
});
