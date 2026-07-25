// update-imports.js
const fs = require("fs");
const path = require("path");
const { execSync } = require("child_process");

// get all ts/tsx files
const files = execSync(
  'find . -type f \\( -name "*.ts" -o -name "*.tsx" \\) ! -name "*.d.ts"',
  {
    encoding: "utf-8",
  },
)
  .trim()
  .split("\n");

const appRoot = path.resolve(__dirname);

files.forEach((file) => {
  const content = fs.readFileSync(file, "utf-8");
  const fileDir = path.dirname(file);

  // regex to match import/export statements with relative paths
  const updated = content.replace(
    /^(import|export)(\s+(?:type\s+)?(?:{[^}]+}|\*\s+as\s+\w+|\w+)?\s*(?:,\s*{[^}]+})?\s+from\s+)['"](\.[^'"]+)['"]/gm,
    (match, keyword, middle, importPath) => {
      // resolve the relative path to absolute
      const absolutePath = path.resolve(fileDir, importPath);
      // make it relative to app root
      let relativePath = path.relative(appRoot, absolutePath);
      // convert to ~ path
      const tildeImport = "~/" + relativePath;
      return `${keyword}${middle}"${tildeImport}"`;
    },
  );

  if (updated !== content) {
    fs.writeFileSync(file, updated);
    console.log(`Updated: ${file}`);
  }
});

console.log("Done!");
