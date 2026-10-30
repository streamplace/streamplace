// Generates js/streamplace/src/raw-lexicons.ts — a bundle of the raw Streamplace
// lexicon documents, exported as `schemas`. The old @atproto/lex-cli codegen emitted
// this array; @atproto/lex does not, but a few consumers (e.g. metadata-constants.ts)
// still introspect the raw lexicon docs at runtime, so we keep exporting it.
//
// Run via `make js-lexicons`. Do not edit the generated output by hand.
import { writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { readLexiconDocs } from "./lexicon-docs.mjs";

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(here, "..", "..", "..");
// Only our own lexicons — bsky/com.atproto docs come from @atproto/api at runtime.
const roots = [
  join(repoRoot, "lexicons", "place"),
  join(repoRoot, "lexicons", "games"),
];

const docs = readLexiconDocs(roots);

const body = `/**
 * GENERATED CODE - DO NOT MODIFY
 *
 * Raw Streamplace lexicon documents, produced by scripts/gen-raw-lexicons.mjs.
 * Run \`make js-lexicons\` to regenerate.
 */
import type { LexiconDoc } from "@atproto/lexicon";

export const schemas = ${JSON.stringify(docs, null, 2)} as unknown as LexiconDoc[];
`;

const outFile = join(here, "..", "src", "raw-lexicons.ts");
writeFileSync(outFile, body);
console.log(`wrote ${docs.length} lexicon docs to ${outFile}`);
