// @ts-check
import starlight from "@astrojs/starlight";
import { defineConfig } from "astro/config";
import starlightLinksValidator from "starlight-links-validator";
import starlightOpenAPI, { openAPISidebarGroups } from "starlight-openapi";
import starlightSidebarSwipe from "starlight-sidebar-swipe";
import starlightSidebarTopics from "starlight-sidebar-topics";

// https://astro.build/config
export default defineConfig({
  base: "/docs",
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
        src: "/src/assets/cube.png",
        alt: "Streamplace Logo",
      },
      favicon: "/favicon.ico",
      plugins: [
        starlightLinksValidator(),
        starlightSidebarSwipe(),
        starlightSidebarTopics([
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
                label: "Video Metadata",
                autogenerate: { directory: "video-metadata" },
              },
              {
                label: "Components",
                autogenerate: { directory: "components" },
              },
              {
                label: "Lexicon Reference",
                autogenerate: { directory: "lex-reference" },
              },
              ...openAPISidebarGroups,
            ],
          },
        ]),
        starlightOpenAPI([
          {
            base: "api",
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
      ],
    }),
  ],
});
