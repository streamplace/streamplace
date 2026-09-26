#!/usr/bin/env node
// Streamplace brand asset generator.
//
// Reads a brand directory (see brand/README.md for the contract) and
// generates every derived brand asset in the repo: the Expo
// icon/splash/adaptive-icon/favicon PNGs (Expo prebuild derives all native
// iOS/Android formats from those), the OG link banner, desktop ICO/ICNS,
// docs logos, the downloadable /brand SVGs, and a TypeScript module the
// app's logo components render from. Every output is gitignored; the brand
// directory is the only source of truth.
//
// A brand directory is a node branding bundle unzipped: branding.yaml
// naming the files beside it, in the same vocabulary a node imports
// (pkg/branding/vocab.go), so one directory brands both the builds and the
// running node. Older directories with a brand.json and conventionally
// named files (mark.svg, icon.png, ...) still work.
//
// Brand dir resolution: $SP_BRAND_DIR, else brand/custom/ (gitignored — for
// first-party or private art), else brand/ (the open-source default).

import {
  copyFileSync,
  existsSync,
  mkdirSync,
  readFileSync,
  writeFileSync,
} from "node:fs";
import { basename, dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import sharp from "sharp";
import YAML from "yaml";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");

const isBrandDir = (dir) =>
  existsSync(join(dir, "branding.yaml")) || existsSync(join(dir, "brand.json"));

const brandDir = process.env.SP_BRAND_DIR
  ? resolve(process.env.SP_BRAND_DIR)
  : isBrandDir(join(repoRoot, "brand", "custom"))
    ? join(repoRoot, "brand", "custom")
    : join(repoRoot, "brand");

function fail(msg) {
  console.error(`brand: ${msg}`);
  process.exit(1);
}

if (!isBrandDir(brandDir)) {
  fail(`no branding.yaml (or legacy brand.json) in ${brandDir}`);
}

// The brand, whichever format it came in: `config` in brand.json's shape,
// `files` the art by role (a file name in brandDir, or undefined).
function loadBrand() {
  const yamlPath = join(brandDir, "branding.yaml");
  if (existsSync(yamlPath)) {
    const doc = YAML.parse(readFileSync(yamlPath, "utf8")) ?? {};
    const b = doc.branding ?? {};
    let story = b.appStory ?? null;
    if (typeof story === "string") {
      try {
        story = JSON.parse(story);
      } catch {
        fail("appStory must be JSON");
      }
    }
    return {
      config: {
        name: b.appName ?? b.siteTitle,
        wordmark: b.appWordmark,
        defaultSiteTitle: b.siteTitle,
        monochrome: b.appMonochrome === "on",
        colors: b.appColors ?? {},
        story,
        bundleId: b.appBundleId,
        host: b.appHost,
      },
      files: {
        mark: b.mainLogo,
        icon: b.appIcon,
        "icon-foreground": b.appIconForeground,
        splash: b.appSplash,
        wordmark: b.appWordmarkImage,
        linkbanner: b.linkBanner,
      },
    };
  }
  const config = JSON.parse(readFileSync(join(brandDir, "brand.json"), "utf8"));
  const files = {};
  for (const role of [
    "mark",
    "icon",
    "icon-foreground",
    "splash",
    "wordmark",
    "linkbanner",
  ]) {
    files[role] = ["svg", "png"]
      .map((ext) => `${role}.${ext}`)
      .find((f) => existsSync(join(brandDir, f)));
  }
  return { config, files };
}

const { config, files: brandFiles } = loadBrand();
if (!config.name) fail("the brand must set appName (or siteTitle)");

const colors = {
  ink: "#0A0A0B",
  paper: "#ffffff",
  iconBackground: "#ffffff",
  iconForeground: null, // defaults to ink
  adaptiveIconBackground: "#111113",
  adaptiveIconForeground: null, // defaults to ink
  splashBackground: "#ffffff",
  splashForeground: null, // defaults to ink
  tileBackground: "#111113",
  tileForeground: "#ffffff",
  tileHairline: "rgba(255,255,255,0.10)",
  bannerBackground: null, // defaults to paper
  bannerForeground: null, // defaults to ink
  ...config.colors,
};
colors.iconForeground ??= colors.ink;
colors.adaptiveIconForeground ??= colors.ink;
colors.splashForeground ??= colors.ink;
colors.bannerBackground ??= colors.paper;
colors.bannerForeground ??= colors.ink;

const name = config.name;
const wordmark = config.wordmark ?? name;
// What an unbranded node calls itself in the nav / titles until its operator
// sets a runtime siteTitle. The first-party brand sets this to its wordmark
// so its own nodes show the styled wordmark with no runtime config.
const defaultSiteTitle = config.defaultSiteTitle ?? `My ${name} Node`;
const mono = config.monochrome === true;
const story = config.story ?? null;
// Native identity of a white-label build; app.config.ts falls back to the
// first-party values (and the SP_* env overrides still win).
const bundleId = config.bundleId ?? null;
const host = config.host ?? null;
const slug = name
  .toLowerCase()
  .replace(/[^a-z0-9]+/g, "-")
  .replace(/^-|-$/g, "");

// ---------------------------------------------------------------- svg utils

const stripSize = (svg) =>
  svg.replace(
    /<svg([^>]*)>/,
    (_, attrs) => `<svg${attrs.replace(/\s(?:width|height)="[^"]*"/g, "")}>`,
  );

function viewBoxOf(svg) {
  const m = svg.match(/viewBox\s*=\s*"([^"]+)"/);
  if (!m) fail("SVG inputs must declare a viewBox");
  const [x, y, w, h] = m[1]
    .trim()
    .split(/[\s,]+/)
    .map(Number);
  return { x, y, w, h };
}

const innerOf = (svg) =>
  svg.replace(/^[\s\S]*?<svg[^>]*>/, "").replace(/<\/svg>\s*$/, "");

const withColor = (svg, color) =>
  color ? svg.replaceAll("currentColor", color) : svg;

const escapeXml = (s) =>
  s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");

// Nest an SVG inside a box of a larger composition. preserveAspectRatio
// keeps non-square art centered without distortion. (Raster-only outputs —
// runtime SVG for react-native-svg uses group transforms instead.)
function embed(svg, x, y, w, h) {
  const vb = viewBoxOf(svg);
  return `<svg x="${x}" y="${y}" width="${w}" height="${h}" viewBox="${vb.x} ${vb.y} ${vb.w} ${vb.h}">${innerOf(svg)}</svg>`;
}

// Same, but as a <g transform>: react-native-svg's SvgXml handles nested
// <svg> poorly on native, so the runtime tile uses translate+scale.
function embedGroup(svg, x, y, w, h) {
  const vb = viewBoxOf(svg);
  const s = Math.min(w / vb.w, h / vb.h);
  const tx = x + (w - vb.w * s) / 2 - vb.x * s;
  const ty = y + (h - vb.h * s) / 2 - vb.y * s;
  return `<g transform="translate(${round(tx)} ${round(ty)}) scale(${round(s)})">${innerOf(svg)}</g>`;
}

const round = (n) => Math.round(n * 1000) / 1000;

const compose = (w, h, parts) =>
  `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}">${parts.join("")}</svg>`;

const rasterize = (svg) => sharp(Buffer.from(svg)).png().toBuffer();

// ------------------------------------------------------------ brand inputs

function findArt(role) {
  const file = brandFiles[role];
  if (!file) return null;
  const p = join(brandDir, basename(file));
  if (!existsSync(p)) fail(`${role}: ${file} is not in ${brandDir}`);
  if (/\.svg$/i.test(p)) return { svg: stripSize(readFileSync(p, "utf8")) };
  if (/\.png$/i.test(p)) return { png: p };
  fail(`${role}: ${file} must be an SVG or PNG`);
}

const markArt = findArt("mark");
if (!markArt?.svg) fail("the mark (mainLogo) is required and must be an SVG");
const markSvg = markArt.svg;

const TRANSPARENT = { r: 0, g: 0, b: 0, alpha: 0 };

// Render brand art onto a size×size canvas: `scale` insets the art,
// `background` fills the canvas, `color` resolves currentColor for
// monochrome SVG art.
async function squarePng(art, size, { background, scale = 1, color } = {}) {
  if (art.svg) {
    const box = size * scale;
    const off = (size - box) / 2;
    const parts = [];
    if (background)
      parts.push(
        `<rect width="${size}" height="${size}" fill="${background}"/>`,
      );
    parts.push(embed(withColor(art.svg, color), off, off, box, box));
    return rasterize(compose(size, size, parts));
  }
  const box = Math.round(size * scale);
  const img = await sharp(art.png)
    .resize(box, box, { fit: "contain", background: TRANSPARENT })
    .png()
    .toBuffer();
  return sharp({
    create: {
      width: size,
      height: size,
      channels: 4,
      background: background ?? TRANSPARENT,
    },
  })
    .composite([{ input: img }])
    .png()
    .toBuffer();
}

// -------------------------------------------------------- container formats

// ICO: a directory of embedded PNGs (supported everywhere since Vista).
function buildIco(pngs) {
  const header = Buffer.alloc(6);
  header.writeUInt16LE(1, 2);
  header.writeUInt16LE(pngs.length, 4);
  const entries = [];
  const blobs = [];
  let offset = 6 + 16 * pngs.length;
  for (const { size, buf } of pngs) {
    const e = Buffer.alloc(16);
    e.writeUInt8(size >= 256 ? 0 : size, 0);
    e.writeUInt8(size >= 256 ? 0 : size, 1);
    e.writeUInt16LE(1, 4);
    e.writeUInt16LE(32, 6);
    e.writeUInt32LE(buf.length, 8);
    e.writeUInt32LE(offset, 12);
    offset += buf.length;
    entries.push(e);
    blobs.push(buf);
  }
  return Buffer.concat([header, ...entries, ...blobs]);
}

// ICNS: typed chunks of embedded PNGs.
function buildIcns(chunks) {
  const body = Buffer.concat(
    chunks.map(({ type, buf }) => {
      const h = Buffer.alloc(8);
      h.write(type, 0, 4, "ascii");
      h.writeUInt32BE(8 + buf.length, 4);
      return Buffer.concat([h, buf]);
    }),
  );
  const h = Buffer.alloc(8);
  h.write("icns", 0, 4, "ascii");
  h.writeUInt32BE(8 + body.length, 4);
  return Buffer.concat([h, body]);
}

// ------------------------------------------------------------- compositions

function tileSvgString() {
  const squircle =
    "M 0 16 C 0 4 4 0 16 0 C 28 0 32 4 32 16 C 32 28 28 32 16 32 C 4 32 0 28 0 16 Z";
  const mark = withColor(markSvg, mono ? colors.tileForeground : null);
  return compose(32, 32, [
    `<path d="${squircle}" fill="${colors.tileBackground}"/>`,
    `<path d="${squircle}" fill="none" stroke="${colors.tileHairline}" stroke-width="1"/>`,
    embedGroup(mark, 4, 4, 24, 24),
  ]).replace(/^<svg([^>]*) width="32" height="32"/, "<svg$1");
}

// Keeps currentColor so the runtime brand menu can tint it; file outputs
// resolve it to ink.
function wordmarkSvgString() {
  const provided = findArt("wordmark");
  if (provided?.svg) return provided.svg;
  const w = Math.max(140, Math.round(wordmark.length * 38));
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${w} 96"><text x="0" y="68" fill="currentColor" font-family="Geist, Inter, Arial, sans-serif" font-size="72" font-weight="600" letter-spacing="-1.44">${escapeXml(wordmark)}</text></svg>`;
}

function lockupSvgString() {
  const wm = withColor(wordmarkSvgString(), colors.ink);
  const wmVb = viewBoxOf(wm);
  const textW = Math.round((wmVb.w / wmVb.h) * 96);
  const markW = 96;
  const gap = 24;
  return compose(markW + gap + textW, 128, [
    embed(withColor(markSvg, mono ? colors.ink : null), 0, 16, markW, markW),
    embed(wm, markW + gap, 16, textW, 96),
  ]);
}

// -------------------------------------------------------------------- main

const outApp = join(repoRoot, "js/app/assets/generated");
const outPublic = join(repoRoot, "js/app/public");
const outPublicBrand = join(outPublic, "brand");
const outDesktop = join(repoRoot, "js/desktop/assets/images");
const outDocs = join(repoRoot, "js/docs/src/assets");
for (const d of [outApp, outPublicBrand, outDesktop, outDocs]) {
  mkdirSync(d, { recursive: true });
}

const written = [];
function write(path, data) {
  writeFileSync(path, data);
  written.push(path.slice(repoRoot.length + 1));
}

// Expo sources: prebuild derives every native iOS/Android icon from these.
const iconArt = findArt("icon") ?? {
  synth: {
    background: colors.iconBackground,
    scale: 0.62,
    color: colors.iconForeground,
  },
};
const iconPng = (size) =>
  iconArt.synth
    ? squarePng(markArt, size, iconArt.synth)
    : squarePng(iconArt, size, {});
write(join(outApp, "icon.png"), await iconPng(1024));

const adaptiveArt = findArt("icon-foreground");
write(
  join(outApp, "adaptive-icon.png"),
  adaptiveArt
    ? await squarePng(adaptiveArt, 1024, {})
    : await squarePng(markArt, 1024, {
        scale: 0.45,
        color: colors.adaptiveIconForeground,
      }),
);

const splashArt = findArt("splash");
write(
  join(outApp, "splash.png"),
  splashArt
    ? await squarePng(splashArt, 1024, {})
    : await squarePng(markArt, 1024, {
        scale: 0.5,
        color: colors.splashForeground,
      }),
);

const favicon = await squarePng(markArt, 256, {
  scale: 0.92,
  color: colors.ink,
});
write(join(outApp, "favicon.png"), favicon);
write(join(outPublic, "favicon.png"), favicon);

// OG / social card.
const bannerArt = findArt("linkbanner");
if (bannerArt?.png) {
  copyFileSync(bannerArt.png, join(outPublic, "linkbanner.png"));
  written.push("js/app/public/linkbanner.png");
} else if (bannerArt?.svg) {
  write(
    join(outPublic, "linkbanner.png"),
    await rasterize(
      compose(1200, 630, [embed(bannerArt.svg, 0, 0, 1200, 630)]),
    ),
  );
} else {
  write(
    join(outPublic, "linkbanner.png"),
    await rasterize(
      compose(1200, 630, [
        `<rect width="1200" height="630" fill="${colors.bannerBackground}"/>`,
        embed(withColor(markSvg, colors.bannerForeground), 450, 165, 300, 300),
      ]),
    ),
  );
}

// Downloadable brand SVGs (served from /brand, listed on the brand screen).
const brandAssets = [
  { file: `${slug}-mark.svg`, label: "Mark" },
  { file: `${slug}-tile.svg`, label: "App tile" },
  { file: `${slug}-wordmark.svg`, label: "Wordmark" },
  { file: `${slug}-lockup.svg`, label: "Lockup" },
];
write(
  join(outPublicBrand, `${slug}-mark.svg`),
  withColor(markSvg, mono ? colors.ink : null),
);
write(join(outPublicBrand, `${slug}-tile.svg`), tileSvgString());
write(
  join(outPublicBrand, `${slug}-wordmark.svg`),
  withColor(wordmarkSvgString(), colors.ink),
);
write(join(outPublicBrand, `${slug}-lockup.svg`), lockupSvgString());

// Desktop (Electron Forge): window/taskbar PNG plus ICO and ICNS.
write(join(outDesktop, "streamplace-logo.png"), await iconPng(512));
const icoSizes = [16, 24, 32, 48, 64, 128, 256];
write(
  join(outDesktop, "streamplace-logo.ico"),
  buildIco(
    await Promise.all(
      icoSizes.map(async (size) => ({ size, buf: await iconPng(size) })),
    ),
  ),
);
const icnsTypes = [
  ["ic11", 32],
  ["ic12", 64],
  ["ic07", 128],
  ["ic08", 256],
  ["ic13", 256],
  ["ic09", 512],
  ["ic14", 512],
  ["ic10", 1024],
];
write(
  join(outDesktop, "streamplace-logo.icns"),
  buildIcns(
    await Promise.all(
      icnsTypes.map(async ([type, size]) => ({
        type,
        buf: await iconPng(size),
      })),
    ),
  ),
);

// Docs (Starlight header logo, light/dark themes).
const docsLight = await squarePng(markArt, 512, {
  color: mono ? colors.ink : null,
});
const docsDark = mono
  ? await squarePng(markArt, 512, { color: colors.paper })
  : docsLight;
write(join(outDocs, "logo-light.png"), docsLight);
write(join(outDocs, "logo-dark.png"), docsDark);

// Resolved config for app.config.ts (splash/adaptive-icon colors).
write(
  join(outApp, "brand.json"),
  JSON.stringify(
    {
      name,
      slug,
      wordmark,
      defaultSiteTitle,
      monochrome: mono,
      colors,
      bundleId,
      host,
    },
    null,
    2,
  ) + "\n",
);

// TypeScript module the app's logo components render from.
const brandTs = `// AUTO-GENERATED by js/brand/generate.mjs — do not edit.
// Regenerate with \`pnpm run brand\` from the repo root.

export type BrandStory = {
  tagline?: string;
  readingsIntro?: string;
  readings?: { title: string; body: string }[];
  geometry?: {
    grid: number;
    tilePath: string;
    markPath: string;
    upperPlay: number[][];
    lowerPlay: number[][];
    center: number[];
  };
  constructionNotes?: string;
  specs?: { label: string; value: string }[];
  usage?: { ok: boolean; text: string }[];
};

export type Brand = {
  name: string;
  slug: string;
  wordmark: string;
  defaultSiteTitle: string;
  monochrome: boolean;
  /** Hostname of the node a white-label build talks to by default. */
  host: string | null;
  colors: {
    ink: string;
    paper: string;
    tileBackground: string;
    tileForeground: string;
    tileHairline: string;
  };
  markSvg: string;
  tileSvg: string;
  wordmarkSvg: string;
  assets: { file: string; label: string }[];
  story: BrandStory | null;
};

export const BRAND: Brand = {
  name: ${JSON.stringify(name)},
  slug: ${JSON.stringify(slug)},
  wordmark: ${JSON.stringify(wordmark)},
  defaultSiteTitle: ${JSON.stringify(defaultSiteTitle)},
  monochrome: ${mono},
  host: ${JSON.stringify(host)},
  colors: {
    ink: ${JSON.stringify(colors.ink)},
    paper: ${JSON.stringify(colors.paper)},
    tileBackground: ${JSON.stringify(colors.tileBackground)},
    tileForeground: ${JSON.stringify(colors.tileForeground)},
    tileHairline: ${JSON.stringify(colors.tileHairline)},
  },
  markSvg: ${JSON.stringify(markSvg)},
  tileSvg: ${JSON.stringify(tileSvgString())},
  wordmarkSvg: ${JSON.stringify(wordmarkSvgString())},
  assets: ${JSON.stringify(brandAssets)},
  story: ${story ? JSON.stringify(story, null, 2) : "null"},
};
`;
write(join(outApp, "brand.ts"), brandTs);

console.log(
  `brand: generated ${written.length} assets for "${name}" from ${brandDir.slice(repoRoot.length + 1) || brandDir}`,
);
