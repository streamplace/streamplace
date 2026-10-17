// @ts-check
import starlight from "@astrojs/starlight";
import { defineConfig, passthroughImageService } from "astro/config";
import starlightOpenAPI, { openAPISidebarGroups } from "starlight-openapi";
import starlightSidebarSwipe from "starlight-sidebar-swipe";
import starlightSidebarTopics from "starlight-sidebar-topics";

// https://astro.build/config
export default defineConfig({
  base: "/docs",
  image: {
    service: passthroughImageService(),
  },
  integrations: [
    starlight({
      title: "Streamplace Docs",
      customCss: [
        "@fontsource/atkinson-hyperlegible-next/400.css",
        "@fontsource/atkinson-hyperlegible-next/600.css",
        "./src/styles/custom-font-face.css",
        "./src/styles/pre-first-table-col.css",
        "./src/styles/widths.css",
      ],
      social: [
        {
          icon: "github",
          label: "GitHub",
          href: "https://github.com/streamplace/streamplace",
        },
      ],
      logo: {
        light: "/src/assets/logo-light.png",
        dark: "/src/assets/logo-dark.png",
        alt: "Streamplace Logo",
      },
      favicon: "/favicon.ico",
      plugins: [
        //starlightLinksValidator(),
        starlightSidebarSwipe(),
        starlightOpenAPI([
          {
            base: "/api",
            label: "Related XRPC API endpoints",
            schema: "./src/content/docs/lex-reference/openapi.json", // or your json generated from swagger
            sidebar: {
              operations: {
                badges: true,
                labels: "operationId",
              },
            },
          },
        ]),
        starlightSidebarTopics(
          [
            {
              label: "For Streamers & Viewers",
              link: "/",
              icon: "open-book",
              items: [
                {
                  label: "Start Streaming",
                  autogenerate: { directory: "guides/start-streaming" },
                },
                {
                  label: "Features",
                  autogenerate: { directory: "features" },
                },
              ],
            },
            {
              label: "For Developers",
              link: "/developers/",
              icon: "seti:config",
              id: "developers",
              items: [
                {
                  label: "Start Contributing",
                  autogenerate: { directory: "guides/start-contributing" },
                },
                {
                  label: "Installing Streamplace",
                  autogenerate: { directory: "guides/installing" },
                },
                {
                  label: "Features (Dev)",
                  autogenerate: { directory: "features-dev" },
                },
                {
                  label: "Video Metadata",
                  autogenerate: { directory: "video-metadata" },
                },
                {
                  label: "Components",
                  autogenerate: { directory: "components" },
                },
                {
                  label: "Localize Streamplace",
                  autogenerate: { directory: "guides/localizing" },
                },
              ],
            },
            {
              label: "API Reference",
              link: "/reference/",
              icon: "seti:json",
              id: "ref",
              items: [
                {
                  label: "Lexicon Reference",
                  autogenerate: { directory: "lex-reference" },
                },
                ...openAPISidebarGroups,
              ],
            },
          ],
          {
            topics: {
              ref: ["/api", "/api/**/*"],
            },
          },
        ),
      ],
    }),
  ],
});
