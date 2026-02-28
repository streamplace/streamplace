#!/usr/bin/env node
// Generates assets/emoji-data-stripped.json from emojibase-data.
// Run with: node scripts/gen-emoji-data.js

// Generates an kv of `string` emoji id to an object with:
// {
//  id: "grinning-face", // from shortcodes, not necessarily the same as the hexcode
//  m: "Grinning Face",  // the official name, for display in the emoji picker
//  k: ["face", "grin"], // tags, for search in the emoji picker
//  s: [{ n: "😀" }],    // skins, with n for the native emoji character.
//                       // the first skin is always the default (no tone modifier),
//                       // and the rest correspond to tones 1-5, if supported by the emoji.
// } | string (alias to another id)

const path = require("path");
const fs = require("fs");

const emojibaseData = path.dirname(
  require.resolve("emojibase-data/package.json"),
);
const data = require(path.join(emojibaseData, "en/data.json"));
const shortcodes = require(
  path.join(emojibaseData, "en/shortcodes/emojibase.json"),
);

const slim = { emojis: {}, aliases: {} };

for (const emoji of data) {
  if (!emoji.emoji) continue;
  const codes = shortcodes[emoji.hexcode];
  if (!codes) continue;
  const all = Array.isArray(codes) ? codes : [codes];
  const id = all[0];
  // skins[0] is always the default (no tone modifier).
  // skins[1-5] correspond to Fitzpatrick tones 1-5, if the emoji supports them.
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

const out = path.join(__dirname, "../assets/emoji-data-stripped.json");
fs.writeFileSync(out, JSON.stringify(slim));

const kb = (fs.statSync(out).size / 1024).toFixed(1);
console.log(
  `wrote ${out} (${kb} KB, ${Object.keys(slim.emojis).length} emoji, ${Object.keys(slim.aliases).length} aliases)`,
);
