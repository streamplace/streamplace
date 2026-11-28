import { build } from "esbuild";
import { writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

// The VOD page's inline boot script and the engine it imports after the first
// frame (docs/vod-instant-start.md), next to Expo's own immutable bundles.
const output = process.argv[2] ?? "dist";
const web = "_expo/static/js/web";
const outdir = path.join(output, web);
const src = (file) =>
  fileURLToPath(
    new URL(
      `../../components/src/components/mobile-player/vod-engine/${file}`,
      import.meta.url,
    ),
  );
const common = {
  bundle: true,
  minify: true,
  target: "es2022",
  legalComments: "none",
  logLevel: "warning",
};

const boot = await build({
  ...common,
  entryPoints: [src("boot.ts")],
  format: "iife",
  write: false,
});
// @moq/net is its own file, imported only once the engine connects: a split
// build would also hoist shared helpers into a chunk the engine loads eagerly.
const emit = async (name, options) => {
  const { metafile } = await build({
    ...common,
    ...options,
    format: "esm",
    outdir,
    entryNames: `${name}-[hash]`,
    metafile: true,
  });
  const [file, { bytes }] = Object.entries(metafile.outputs)[0];
  return { name: path.basename(file), bytes };
};
const moq = await emit("vod-moq", {
  stdin: {
    contents: 'export * from "@moq/net";',
    resolveDir: src("."),
    sourcefile: "vod-moq.js",
  },
});
const engine = await emit("vod-engine", {
  entryPoints: [src("engine.ts")],
  plugins: [
    {
      name: "vod-moq",
      setup: (b) =>
        b.onResolve({ filter: /^@moq\/net$/ }, () => ({
          path: `./${moq.name}`,
          external: true,
        })),
    },
  ],
});

const script = boot.outputFiles[0].text.trim();
await writeFile(
  path.join(outdir, "vod-boot.json"),
  JSON.stringify({ boot: script, engine: `/${web}/${engine.name}` }),
);
console.log(
  `VOD boot: ${script.length} inline bytes; engine ${engine.bytes} bytes, MoQ ${moq.bytes} more`,
);
