import { readFileSync, writeFileSync } from "node:fs";
import ts from "typescript";
const destination = process.argv[2];
const source = readFileSync("js/streamplace/src/captioner/worker.ts", "utf8");
const result = ts.transpileModule(source, {
  fileName: "worker.ts",
  compilerOptions: {
    module: ts.ModuleKind.ES2022,
    target: ts.ScriptTarget.ES2022,
    verbatimModuleSyntax: true,
  },
});
if (result.diagnostics?.length) {
  throw new Error(
    result.diagnostics
      .map((diagnostic) =>
        ts.flattenDiagnosticMessageText(diagnostic.messageText, "\n"),
      )
      .join("\n"),
  );
}
writeFileSync(`${destination}/worker.js`, result.outputText);
