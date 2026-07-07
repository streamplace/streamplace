// Writes the external (Bluesky / com.atproto / chat / ozone) lexicon documents
// bundled in @atproto/api into a target directory as a JSON file tree, so that
// `lex build` can resolve the refs that Streamplace's own lexicons make into
// them. We source these from the pinned @atproto/api dependency rather than a
// checked-out upstream copy so the JS codegen is self-contained in CI.
//
// Usage: node dump-external-lexicons.mjs <outDir>
import { mkdirSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";

const require = createRequire(import.meta.url);
// @atproto/api is CommonJS; load its bundled lexicon docs via require.
const { schemas } = require("@atproto/api/dist/client/lexicons");

// @atproto/api's compiled docs express refs as `lex:`-prefixed URIs
// (e.g. "lex:app.bsky.actor.defs#profileView"), but `lex build` resolves plain
// NSID refs. Strip the scheme so the docs match the on-disk raw lexicon format.
function stripLexUris(value) {
  if (typeof value === "string") {
    return value.startsWith("lex:") ? value.slice(4) : value;
  }
  if (Array.isArray(value)) return value.map(stripLexUris);
  if (value && typeof value === "object") {
    const out = {};
    for (const [k, v] of Object.entries(value)) out[k] = stripLexUris(v);
    return out;
  }
  return value;
}

const outDir = process.argv[2];
if (!outDir) {
  console.error("usage: dump-external-lexicons.mjs <outDir>");
  process.exit(1);
}

let count = 0;
for (const doc of schemas) {
  if (typeof doc?.id !== "string") continue;
  const file = join(outDir, doc.id.replace(/\./g, "/") + ".json");
  mkdirSync(dirname(file), { recursive: true });
  writeFileSync(file, JSON.stringify(stripLexUris(doc), null, 2));
  count++;
}
console.log(`wrote ${count} external lexicon docs to ${outDir}`);
