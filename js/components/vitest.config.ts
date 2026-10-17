import { resolve } from "node:path";
import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: {
    alias: {
      "@streamplace/components/src": resolve(import.meta.dirname, "src"),
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
