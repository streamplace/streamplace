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
    // Walk up the directory tree to find node_modules
    let searchDir = baseDir;
    let lucideExportsPath;
    while (searchDir !== path.dirname(searchDir)) {
      const candidatePath = path.join(
        searchDir,
        "node_modules/lucide-react-native/dist/esm/lucide-react-native.js",
      );
      if (fs.existsSync(candidatePath)) {
        lucideExportsPath = candidatePath;
        break;
      }
      searchDir = path.dirname(searchDir);
    }

    if (!lucideExportsPath) {
      const err = new Error(
        `Could not find lucide-react-native/dist/esm/lucide-react-native.js in node_modules. Searched from: ${baseDir}`,
      );
      console.error(err.message);
      throw err;
    }

    const content = fs.readFileSync(lucideExportsPath, "utf8");

    // Match lines like: export { default as User2, default as UserRound } from './icons/user-round.js';
    // Split by lines and parse each line individually
    const lines = content.split("\n");
    let matchCount = 0;

    for (const line of lines) {
      // Use string indexOf/substring instead of regex
      if (line.includes("from './icons/") && line.includes("export {")) {
        const iconsIndex = line.indexOf("from './icons/");
        if (iconsIndex === -1) continue;

        const filenameStart = iconsIndex + "from './icons/".length;
        const filenameEnd = line.indexOf(".js'", filenameStart);
        if (filenameEnd === -1) continue;

        const filename = line.substring(filenameStart, filenameEnd);

        const exportsStart = line.indexOf("{") + 1;
        const exportsEnd = line.indexOf("}");
        if (exportsStart === -1 || exportsEnd === -1) continue;

        const exports = line.substring(exportsStart, exportsEnd);
        matchCount++;

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
    }
  } catch (err) {
    console.error(
      "[lucide-transform] Failed to parse lucide exports, using hardcoded mappings:",
      err.message,
    );
    // Keep the hardcoded mappings we initialized with
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
