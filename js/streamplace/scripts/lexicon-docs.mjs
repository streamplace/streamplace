// The one reader of the source lexicon documents under lexicons/, shared by
// gen-raw-lexicons.mjs (bundle) and release-lib.mjs (schema publication).
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

function walk(dir) {
  const out = [];
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) out.push(...walk(full));
    else if (name.endsWith(".json")) out.push(full);
  }
  return out;
}

/** Every lexicon document (a JSON file with a string `id`) under `roots`, sorted by id. */
export function readLexiconDocs(roots) {
  return roots
    .flatMap(walk)
    .map((f) => JSON.parse(readFileSync(f, "utf8")))
    .filter((doc) => typeof doc?.id === "string")
    .sort((a, b) => a.id.localeCompare(b.id));
}
