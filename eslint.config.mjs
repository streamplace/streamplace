import tsParser from "@typescript-eslint/parser";
import reactHooks from "eslint-plugin-react-hooks";
import noTokenLiterals from "./js/scripts/eslint/no-token-literals.mjs";

// Component code governed by the design tokens. Mirrors the directories the
// check-tokens ratchet scanned before it was replaced by this rule; see the
// streamplace-design skill for the contract.
const SOURCES = [
  "js/app/src/**/*.{ts,tsx}",
  "js/app/components/**/*.{ts,tsx}",
  "js/app/hooks/**/*.{ts,tsx}",
  "js/components/src/**/*.{ts,tsx}",
];

export default [
  {
    ignores: [
      "**/dist/**",
      "**/node_modules/**",
      "js/components/src/lib/theme/**", // token definitions live here
    ],
  },
  {
    files: SOURCES,
    languageOptions: {
      parser: tsParser,
      parserOptions: { ecmaFeatures: { jsx: true } },
    },
    plugins: {
      streamplace: { rules: { "no-token-literals": noTokenLiterals } },
      "react-hooks": reactHooks,
    },
    rules: {
      "streamplace/no-token-literals": "error",
      // Warn, not error: the codebase has 15 pre-existing violations that need
      // a dedicated refactor before this can block the commit gate.
      "react-hooks/rules-of-hooks": "warn",
      // Off by design, matching js/app/biome.json. Registering the plugin keeps
      // the existing `react-hooks/exhaustive-deps` directives resolvable.
      "react-hooks/exhaustive-deps": "off",
    },
  },
];
