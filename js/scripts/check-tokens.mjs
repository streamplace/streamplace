#!/usr/bin/env node
// Token ratchet: counts hardcoded style literals (hex colors, rgb()/rgba(),
// raw palette-ramp indexing) in component code and fails if the count ever
// increases relative to docs/redesign/token-baseline.json.
//
// Usage:
//   node js/scripts/check-tokens.mjs          # check against baseline
//   node js/scripts/check-tokens.mjs --update # rewrite baseline (only downward)
//   node js/scripts/check-tokens.mjs --list   # print every match
//
// Lines ending in a `// token-ok` (or `/* token-ok */`) comment are exempt —
// reserved for intentional literals (e.g. transparent OBS overlay roots).

import { existsSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = join(dirname(fileURLToPath(import.meta.url)), "..", "..");
const BASELINE = join(repoRoot, "docs", "redesign", "token-baseline.json");

const SCAN_DIRS = [
  "js/app/src",
  "js/app/components",
  "js/app/hooks",
  "js/components/src",
];
const EXCLUDE = [
  "js/components/src/lib/theme", // token definitions live here
  "node_modules",
  "dist",
];
const EXTENSIONS = [".ts", ".tsx"];

const RAMPS =
  "slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose|primary|destructive|success|warning";

const PATTERNS = [
  { name: "hex", re: /#(?:[0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{3,4})\b/g },
  { name: "rgb", re: /\brgba?\(/g },
  { name: "ramp", re: new RegExp(`\\bcolors\\.(?:${RAMPS})\\[`, "g") },
];

const ALLOW_MARKER = /token-ok/;

function* walk(dir) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    const rel = relative(repoRoot, full);
    if (EXCLUDE.some((ex) => rel === ex || rel.startsWith(ex + "/"))) continue;
    if (entry.isDirectory()) yield* walk(full);
    else if (EXTENSIONS.some((ext) => entry.name.endsWith(ext))) yield full;
  }
}

const matches = [];
for (const dir of SCAN_DIRS) {
  const abs = join(repoRoot, dir);
  if (!existsSync(abs)) continue;
  for (const file of walk(abs)) {
    const rel = relative(repoRoot, file);
    const lines = readFileSync(file, "utf8").split("\n");
    lines.forEach((line, i) => {
      if (ALLOW_MARKER.test(line)) return;
      const trimmed = line.trimStart();
      if (trimmed.startsWith("//") || trimmed.startsWith("*")) return;
      for (const { name, re } of PATTERNS) {
        re.lastIndex = 0;
        for (const m of line.matchAll(re)) {
          matches.push({ file: rel, line: i + 1, kind: name, text: m[0] });
        }
      }
    });
  }
}

const byDir = {};
for (const m of matches) {
  const top = SCAN_DIRS.find((d) => m.file.startsWith(d)) ?? "other";
  byDir[top] = (byDir[top] ?? 0) + 1;
}
const total = matches.length;

const args = process.argv.slice(2);
if (args.includes("--list")) {
  for (const m of matches) {
    console.log(`${m.file}:${m.line}  [${m.kind}] ${m.text}`);
  }
}

console.log(`token literals found: ${total}`);
for (const [dir, n] of Object.entries(byDir)) console.log(`  ${dir}: ${n}`);

if (args.includes("--update")) {
  writeFileSync(BASELINE, JSON.stringify({ total, byDir }, null, 2) + "\n");
  console.log(`baseline written to ${relative(repoRoot, BASELINE)}`);
  process.exit(0);
}

if (!existsSync(BASELINE)) {
  console.error("no baseline found — run with --update to create one");
  process.exit(1);
}
const baseline = JSON.parse(readFileSync(BASELINE, "utf8"));
if (total > baseline.total) {
  console.error(
    `FAIL: ${total} literals > baseline ${baseline.total}. ` +
      `Use theme tokens instead, or mark intentional literals with // token-ok`,
  );
  process.exit(1);
}
if (total < baseline.total) {
  console.log(
    `improved: ${baseline.total} → ${total}. Run with --update to ratchet the baseline down.`,
  );
}
console.log("token ratchet OK");
