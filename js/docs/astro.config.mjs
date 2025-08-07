// @ts-check
import starlight from "@astrojs/starlight";
import { defineConfig } from "astro/config";
import starlightOpenAPI, { openAPISidebarGroups } from "starlight-openapi";

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
      plugins: [
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
      sidebar: [
        {
          label: "Guides",
          items: [
            {
              label: "Start Streaming",
              autogenerate: { directory: "guides/start-streaming" },
            },
            {
              label: "Installing Streamplace",
              autogenerate: { directory: "guides/installing" },
            },
            {
              label: "Start Contributing",
              autogenerate: { directory: "guides/start-contributing" },
            },
          ],
        },
        // {
        //   label: "Reference",
        //   autogenerate: { directory: "reference" },
        // },
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
    }),
  ],
});
