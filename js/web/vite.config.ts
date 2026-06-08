import tailwindcss from "@tailwindcss/vite";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import react from "@vitejs/plugin-react";
import { resolve } from "node:path";
import { defineConfig } from "vite";

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    // TanStack Router must be registered BEFORE @vitejs/plugin-react so the
    // generated route tree is in place when the React plugin runs.
    tanstackRouter({
      target: "react",
      autoCodeSplitting: true,
      routesDirectory: "./src/routes",
      generatedRouteTree: "./src/routeTree.gen.ts",
    }),
    react(),
    tailwindcss(),
  ],
  resolve: {
    alias: {
      // Workspace packages — Vite reads each package's package.json `main`/`types`
      // fields directly, so these aliases just make resolution explicit.
      "@streamplace/core": resolve(__dirname, "../core/src"),
      streamplace: resolve(__dirname, "../streamplace/src"),
      // @-prefix for src/ — makes imports a bit cleaner and avoids relative path hell.
      "@": resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 5173,
    strictPort: false,
  },
  preview: {
    port: 5173,
  },
});
