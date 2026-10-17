// Generates public/emoji-data.json from emojibase-data.
// Mirrors the shape used in js/app/assets/emoji-data-stripped.json so the
// search/UI code on the web side can match the reference implementation.
//
// Run with: pnpm gen:emoji
// (or automatically before `pnpm build` via the `prebuild` script)

import { mkdirSync, statSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const here = dirname(fileURLToPath(import.meta.url));
const emojibaseData = dirname(require.resolve("emojibase-data/package.json"));

const data = require(join(emojibaseData, "en/data.json"));
const shortcodes = require(join(emojibaseData, "en/shortcodes/emojibase.json"));

const slim = { emojis: {}, aliases: {} };

for (const emoji of data) {
  if (!emoji.emoji) continue;
  const codes = shortcodes[emoji.hexcode];
  if (!codes) continue;
  const all = Array.isArray(codes) ? codes : [codes];
  const id = all[0];
  const skins = [{ native: emoji.emoji }];
  if (emoji.skins) {
    for (let tone = 1; tone <= 5; tone++) {
      const variant = emoji.skins.find((s) => s.tone === tone);
      skins.push({ native: variant ? variant.emoji : emoji.emoji });
    }
  }
  slim.emojis[id] = {
    id,
    m: emoji.label,
    k: emoji.tags ?? [],
    s: skins.map((sk) => ({ n: sk.native })),
  };
  for (const alias of all.slice(1)) {
    slim.aliases[alias] = id;
  }
}

const out = join(here, "..", "public", "emoji-data.json");
mkdirSync(dirname(out), { recursive: true });
writeFileSync(out, JSON.stringify(slim));

const kb = (statSync(out).size / 1024).toFixed(1);
console.log(
  `wrote ${out} (${kb} KB, ${Object.keys(slim.emojis).length} emoji, ${Object.keys(slim.aliases).length} aliases)`,
);
