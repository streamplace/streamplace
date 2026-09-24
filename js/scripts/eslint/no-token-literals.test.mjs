import tsParser from "@typescript-eslint/parser";
import { Linter, RuleTester } from "eslint";
import assert from "node:assert/strict";
import { describe, it } from "node:test";
import rule from "./no-token-literals.mjs";

const RULE = "streamplace/no-token-literals";
const PARSER = {
  languageOptions: {
    parser: tsParser,
    ecmaVersion: "latest",
    sourceType: "module",
    parserOptions: { ecmaFeatures: { jsx: true } },
  },
};

// What the rule reports.
RuleTester.describe = describe;
RuleTester.it = it;
RuleTester.itOnly = it.only;

const ruleTester = new RuleTester(PARSER);

ruleTester.run(RULE, rule, {
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
      code: 'const a = { color: "#fff", borderColor: "#000" };',
      errors: [{ messageId: "literal" }, { messageId: "literal" }],
    },
  ],
});

// Suppression runs through ESLint core, which matches on the fully-qualified
// rule name. RuleTester would rename the rule, so drive a real Linter here.
const linter = new Linter();

function lint(code) {
  return linter.verify(
    code,
    [
      {
        ...PARSER,
        files: ["**/*.tsx"],
        plugins: {
          streamplace: {
            rules: { "no-token-literals": rule, noop: { create: () => ({}) } },
          },
        },
        rules: { [RULE]: "error" },
      },
    ],
    "test.tsx",
  );
}

describe("suppression via eslint-disable-line", () => {
  it("suppresses a trailing line comment", () => {
    const code = `const a = { color: "#fff" }; // eslint-disable-line ${RULE} -- swatch`;
    assert.equal(lint(code).length, 0);
  });

  it("suppresses an inline block comment in JSX", () => {
    const code = `const b = <Text style={{ color: "#000" /* eslint-disable-line ${RULE} -- inline */ }} />;`;
    assert.equal(lint(code).length, 0);
  });

  it("suppresses a ramp index", () => {
    const code = `const c = colors.primary[500]; // eslint-disable-line ${RULE} -- ramp`;
    assert.equal(lint(code).length, 0);
  });

  it("suppresses a ramp index a formatter split across lines", () => {
    const code = `const c =\n  colors\n    .primary[500]; // eslint-disable-line ${RULE} -- swatch`;
    assert.equal(lint(code).length, 0);
  });

  it("still reports the unsuppressed sibling line", () => {
    const code = `const a = { color: "#fff" };\nconst b = { color: "#000" }; // eslint-disable-line ${RULE}`;
    const messages = lint(code);
    assert.equal(messages.length, 1);
    assert.equal(messages[0].line, 1);
  });

  it("does not suppress when the directive names another rule", () => {
    const code = `const d = { color: "#fff" }; // eslint-disable-line streamplace/noop`;
    const messages = lint(code);
    // The unrelated directive suppresses nothing, so the rule still reports.
    assert.equal(messages.filter((m) => m.ruleId === RULE).length, 1);
  });
});
