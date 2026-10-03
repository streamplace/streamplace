import { readFileSync } from "node:fs";
import { expect, it } from "vitest";

it("keeps theme aliases free of self-references", () => {
  const css = readFileSync("src/styles.css", "utf8");
  const aliases = [...css.matchAll(/(--[\w-]+):\s*var\((--[\w-]+)\)\s*;/g)];
  expect(aliases.length).toBeGreaterThan(0);
  // A self-reference invalidates the token, leaving highlight backgrounds
  // transparent even when the paired foreground color is applied.
  const selfReferences = aliases
    .filter(([, name, target]) => name === target)
    .map(([, name]) => name);
  expect(selfReferences).toEqual([]);
});
