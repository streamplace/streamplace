import * as fs from "fs";
import * as path from "path";

// Guards AC.13: no debug junk in the rewritten clip UI files. Reads the
// sources directly so the guard survives future edits (not just this rewrite).

const targets = [
  "../components/mobile-player/ui/clip-button.tsx",
  "../components/mobile-player/ui/clip-editor.tsx",
];

describe("no debug junk in the clip UI", () => {
  for (const rel of targets) {
    const abs = path.join(__dirname, rel);
    const src = fs.readFileSync(abs, "utf8");

    it(`keeps ${rel} free of debug junk`, () => {
      expect(src).not.toContain("log in bro");
      expect(src).not.toContain("clip is rendering");
      expect(src).not.toMatch(/console\.(log|debug)\(/);
    });
  }
});
