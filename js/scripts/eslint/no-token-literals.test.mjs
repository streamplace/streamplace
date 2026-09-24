import tsParser from "@typescript-eslint/parser";
import { RuleTester } from "eslint";
import assert from "node:assert/strict";
import { describe, it } from "node:test";
import rule from "./no-token-literals.mjs";

RuleTester.describe = describe;
RuleTester.it = it;
RuleTester.itOnly = it.only;

const ruleTester = new RuleTester({
  languageOptions: {
    parser: tsParser,
    ecmaVersion: "latest",
    sourceType: "module",
    parserOptions: { ecmaFeatures: { jsx: true } },
  },
});

ruleTester.run("no-token-literals", rule, {
  valid: [
    // Theme tokens are the whole point.
    "const style = { color: theme.colors.text1, backgroundColor: surface1 };",
    // A string that merely mentions a color name is not a literal.
    'const label = "the primary button is pink";',
    // rgb/rgba only count when a literal call is written.
    "const toRgb = (r, g, b) => formatRgb(r, g, b);",
    "const n = 500;",
    // A hex inside a comment is documentation, not a value.
    'const x = 1; // legacy value was "#0a0a0b"',
    "/* the old palette used #ffffff for text */ const y = 2;",
    // token-ok exemptions: trailing, block, and inline-in-JSX.
    'const a = { color: "#fff" }; // token-ok: brand swatch',
    'const b = { color: "rgba(0,0,0,0.6)" }; /* token-ok: overlay scrim */',
    'const c = <Text style={{ color: "#11e8b2" /* token-ok */ }} />;',
    "const d = [colors.primary[500]]; // token-ok: may render outside provider",
  ],
  invalid: [
    {
      code: 'const a = { color: "#fff" };',
      errors: [{ messageId: "literal", data: { kind: "hex", text: "#fff" } }],
    },
    {
      code: 'const a = { color: "#0a0a0b" };',
      errors: [
        { messageId: "literal", data: { kind: "hex", text: "#0a0a0b" } },
      ],
    },
    {
      code: 'const a = { color: "#0a0a0bff" };',
      errors: [
        { messageId: "literal", data: { kind: "hex", text: "#0a0a0bff" } },
      ],
    },
    {
      code: 'const a = { backgroundColor: "rgb(1, 2, 3)" };',
      errors: [
        { messageId: "literal", data: { kind: "rgb", text: "rgb(1, 2, 3)" } },
      ],
    },
    {
      code: 'const a = { backgroundColor: "rgba(0, 0, 0, 0.6)" };',
      errors: [
        {
          messageId: "literal",
          data: { kind: "rgb", text: "rgba(0, 0, 0, 0.6)" },
        },
      ],
    },
    {
      // Template literals report on the quasi element, not the whole template.
      code: "const a = { backgroundColor: `rgba(${r}, ${g}, ${b}, ${alpha})` };",
      errors: [{ messageId: "literal", data: { kind: "rgb", text: "rgba(" } }],
    },
    {
      code: "const a = colors.primary[500];",
      errors: [
        {
          messageId: "literal",
          data: { kind: "ramp", text: "colors.primary[500]" },
        },
      ],
    },
    {
      code: "const a = theme.colors.primary[500];",
      errors: [
        {
          messageId: "literal",
          data: { kind: "ramp", text: "theme.colors.primary[500]" },
        },
      ],
    },
    {
      code: "const a = tokens.colors.success[400];",
      errors: [
        {
          messageId: "literal",
          data: { kind: "ramp", text: "tokens.colors.success[400]" },
        },
      ],
    },
    {
      // Only the offending line is reported; the exempt sibling is untouched.
      code: 'const a = { color: "#fff" };\nconst b = { color: "#000" }; // token-ok',
      errors: [{ messageId: "literal", data: { kind: "hex", text: "#fff" } }],
    },
    {
      code: 'const a = { color: "#fff", borderColor: "#000" };',
      errors: [{ messageId: "literal" }, { messageId: "literal" }],
    },
  ],
});

describe("no-token-literals meta", () => {
  it("declares a problem-type rule", () => {
    assert.equal(rule.meta.type, "problem");
  });
});
