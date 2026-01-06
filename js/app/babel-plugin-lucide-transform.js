const fs = require("fs");
const path = require("path");

const wordSeparators =
  /[\s\u2000-\u206F\u2E00-\u2E7F\\'!"#$%&()*+,\-.\/:;<=>?@\[\]^_`{|}~]+/;
const capital_plus_lower = /[A-ZÀ-Ý\u00C0-\u00D6\u00D9-\u00DD][a-zà-ÿ]/g;
const capitals = /[A-Z0-9À-Ý\u00C0-\u00D6\u00D9-\u00DD]+/g;

function kebabCase(str) {
  let res = str;
  res = str.replace(
    capital_plus_lower,
    (match) => ` ${match[0].toLowerCase() || match[0]}${match[1]}`,
  );
  res = res.replace(capitals, (match) => ` ${match.toLowerCase()}`);
  return res
    .trim()
    .split(wordSeparators)
    .join("-")
    .replace(/^-/, "")
    .replace(/-\s*$/, "");
}

// Build icon name to file mapping from lucide's export file
let iconToFileMap;
function getIconMapping(baseDir) {
  if (iconToFileMap) return iconToFileMap;

  iconToFileMap = {};
  try {
    const lucideExportsPath = path.join(
      baseDir,
      "node_modules/lucide-react-native/dist/esm/lucide-react-native.js",
    );
    const content = fs.readFileSync(lucideExportsPath, "utf8");

    // Match lines like: export { default as User2, default as UserRound } from './icons/user-round.js';
    const exportRegex =
      /export\s*\{([^}]+)\}\s*from\s*['"]\\.\/icons\/([^'"]+)\.js['"]/g;

    let match;
    while ((match = exportRegex.exec(content)) !== null) {
      const exports = match[1];
      const filename = match[2];

      // Extract all icon names from the export list
      const names = exports
        .split(",")
        .map((name) => name.trim())
        .filter((name) => name.startsWith("default as "))
        .map((name) => name.replace("default as ", "").trim());

      // Map each name to the filename
      names.forEach((name) => {
        iconToFileMap[name] = filename;
      });
    }
  } catch (err) {
    console.warn(
      "Failed to parse lucide exports, falling back to kebab-case:",
      err.message,
    );
    iconToFileMap = {};
  }

  return iconToFileMap;
}

function lucideTransformPlugin({ types: t }) {
  return {
    visitor: {
      ImportDeclaration(path, state) {
        const source = path.node.source.value;
        if (source === "lucide-react-native") {
          const imports = [];
          const baseDir = state.file.opts.root || process.cwd();
          const mapping = getIconMapping(baseDir);

          path.node.specifiers.forEach((specifier) => {
            if (specifier.type === "ImportSpecifier") {
              const importedName = specifier.imported.name;
              const localName = specifier.local.name;

              // Use the mapping if available, otherwise fall back to kebab-case
              const filename = mapping[importedName] || kebabCase(importedName);

              // Create individual default import for each icon
              imports.push(
                t.importDeclaration(
                  [t.importDefaultSpecifier(t.identifier(localName))],
                  t.stringLiteral(
                    `lucide-react-native/dist/esm/icons/${filename}`,
                  ),
                ),
              );
            }
          });

          // Replace the original import with individual imports
          path.replaceWithMultiple(imports);
        }
      },
    },
  };
}

module.exports = lucideTransformPlugin;
