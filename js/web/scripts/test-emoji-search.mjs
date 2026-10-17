// Ad-hoc smoke test for the search function. Not wired into the build.
// Run with: node --experimental-strip-types scripts/test-emoji-search.mjs
// (Node 22+ supports stripping TS types natively.)

import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { searchEmojis } from "../src/lib/emoji-data.ts";

const data = JSON.parse(
  readFileSync(
    join(
      dirname(fileURLToPath(import.meta.url)),
      "..",
      "public",
      "emoji-data.json",
    ),
    "utf8",
  ),
);

function check(label, actual, expected) {
  const pass = JSON.stringify(actual) === JSON.stringify(expected);
  console.log(`${pass ? "ok" : "FAIL"}  ${label}`);
  if (!pass) {
    console.log("  expected:", expected);
    console.log("  actual:  ", actual);
  }
  return pass;
}

let pass = true;
const ids = (emojis) => emojis.map((e) => e.id);

pass &= check(
  "exact alias 'shrug' returns person_shrugging first",
  ids(searchEmojis(data, "shrug")).slice(0, 1),
  ["person_shrugging"],
);

pass &= check(
  "alias startsWith 'shru' returns person_shrugging first",
  ids(searchEmojis(data, "shru")).slice(0, 1),
  ["person_shrugging"],
);

pass &= check(
  "exact alias 'smile' ranks ahead of 'smiley' (alias startWith)",
  ids(searchEmojis(data, "smile")).slice(0, 2),
  ["grinning_face_with_closed_eyes", "grinning_face_with_big_eyes"],
);

pass &= check(
  "alias matches dominate the top of the list",
  // Many *_cat aliases contain 'cat', so they win at matchType=2 and the
  // standalone 'cat' id (matchType=3) falls below them.
  ids(searchEmojis(data, "cat")).slice(0, 1),
  ["grinning_cat_with_closed_eyes"],
);

pass &= check(
  "the standalone 'cat' id appears somewhere in the results",
  ids(searchEmojis(data, "cat")).includes("cat"),
  true,
);

pass &= check(
  "query shorter than 3 chars returns empty",
  ids(searchEmojis(data, "ab")),
  [],
);

pass &= check(
  "query with space returns empty",
  ids(searchEmojis(data, "smile face")),
  [],
);

pass &= check("null data returns empty", searchEmojis(null, "smile"), []);

pass &= check(
  "capped at MAX_RESULTS (10)",
  searchEmojis(data, "face").length <= 10,
  true,
);

pass &= check(
  "each result has skin variants",
  searchEmojis(data, "wave")[0]?.s[0]?.n.length > 0,
  true,
);

process.exit(pass ? 0 : 1);
