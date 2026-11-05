import tailwindcss from "@tailwindcss/vite";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import react from "@vitejs/plugin-react";
import { resolve } from "node:path";
import { defineConfig } from "vite";

// Conditionally add Sentry plugin for source maps upload in CI.
function sentryPlugin() {
  const authToken = process.env.SENTRY_AUTH_TOKEN;
  const org = process.env.SENTRY_ORG;
  const project = process.env.SENTRY_PROJECT;
  if (!authToken || !org || !project) return [];
  // Dynamic import so the plugin isn't required for local dev.
  return import("@sentry/vite-plugin").then(({ sentryVitePlugin }) =>
    sentryVitePlugin({
      org,
      project,
      authToken,
      telemetry: false,
      sourcemaps: { assets: "./dist/**" },
    }),
  );
}

// https://vite.dev/config/
export default defineConfig(async () => {
  const sentry = await sentryPlugin();
  return {
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
      ...(sentry ? [sentry] : []),
    ],
    resolve: {
      alias: {
        // Workspace packages — their `exports` default to the published `dist`
        // build, so point Vite at source instead.
        "@streamplace/core": resolve(__dirname, "../core/src"),
        "@streamplace/i18n": resolve(__dirname, "../i18n/src"),
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
  };
});
