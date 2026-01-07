#!/usr/bin/env node

const fs = require("fs");
const path = require("path");
const { parse: parseFtl } = require("@fluent/syntax");

// Load language manifest
const MANIFEST_PATH = path.join(__dirname, "..", "locales", "manifest.json");
const manifest = JSON.parse(fs.readFileSync(MANIFEST_PATH, "utf-8"));

// Configuration
const LOCALES_SOURCE_DIR = path.join(__dirname, "..", "locales");
const LOCALES_OUTPUT_DIR = path.join(__dirname, "..", "public", "locales");

/**
 * Find all .ftl files in a directory (non-recursive, just top-level)
 */
function findFtlFiles(dir) {
  const files = [];
  const entries = fs.readdirSync(dir, { withFileTypes: true });

  for (const entry of entries) {
    if (entry.isFile() && entry.name.endsWith(".ftl")) {
      files.push({
        path: path.join(dir, entry.name),
        namespace: path.basename(entry.name, ".ftl"),
      });
    }
  }

  return files;
}

/**
 * Parse FTL content using @fluent/syntax and extract messages
 */
function extractMessagesFromFtl(content) {
  const messages = {};

  try {
    const resource = parseFtl(content);

    for (const entry of resource.body) {
      // Only process Message entries (not Comments, Terms, etc.)
      if (entry.type === "Message" && entry.id) {
        const key = entry.id.name;

        // For now, just serialize the pattern back to FTL format
        // i18next-fluent will handle the actual parsing at runtime
        if (entry.value) {
          messages[key] = serializePattern(entry.value);
        }
      }
    }
  } catch (error) {
    console.error("Error parsing FTL:", error.message);
    throw error;
  }

  return messages;
}

/**
 * Serialize a Fluent pattern back to FTL string format
 * This is a simple serializer - @fluent/syntax has a full serializer
 * but we just need the pattern text for JSON storage
 */
function serializePattern(pattern) {
  if (!pattern || !pattern.elements) {
    return "";
  }

  let result = "";

  for (const element of pattern.elements) {
    if (element.type === "TextElement") {
      result += element.value;
    } else if (element.type === "Placeable") {
      result += serializePlaceable(element);
    }
  }

  return result;
}

/**
 * Serialize a placeable (variables, select expressions, etc.)
 */
function serializePlaceable(placeable) {
  if (!placeable.expression) {
    return "{}";
  }

  const expr = placeable.expression;

  if (expr.type === "VariableReference") {
    return `{$${expr.id.name}}`;
  } else if (expr.type === "SelectExpression") {
    let result = `{${serializeInlineExpression(expr.selector)} ->`;

    for (const variant of expr.variants) {
      const key =
        variant.key.type === "Identifier"
          ? `[${variant.key.name}]`
          : `[${variant.key.value}]`;

      result += `\n   ${variant.default ? "*" : ""}${key} ${serializePattern(variant.value)}`;
    }

    result += "\n  }";
    return result;
  } else if (expr.type === "FunctionReference") {
    const args = expr.arguments.positional
      .map((arg) => serializeInlineExpression(arg))
      .join(", ");
    return `{${expr.id.name}(${args})}`;
  }

  return "{}";
}

/**
 * Serialize inline expressions (for function arguments, etc.)
 */
function serializeInlineExpression(expr) {
  if (expr.type === "VariableReference") {
    return `$${expr.id.name}`;
  } else if (expr.type === "NumberLiteral") {
    return expr.value;
  } else if (expr.type === "StringLiteral") {
    return `"${expr.value}"`;
  }

  return "";
}

/**
 * Main compilation function
 */
function compileTranslations() {
  console.log("🌍 Compiling translation files...");

  if (!fs.existsSync(LOCALES_SOURCE_DIR)) {
    console.error(`❌ Locales directory not found: ${LOCALES_SOURCE_DIR}`);
    process.exit(1);
  }

  // Create output directory if it doesn't exist
  if (!fs.existsSync(LOCALES_OUTPUT_DIR)) {
    fs.mkdirSync(LOCALES_OUTPUT_DIR, { recursive: true });
  }

  // Get supported locales from manifest
  const manifestLocales = manifest.supportedLocales;
  const locales = manifestLocales.filter((locale) => {
    const localeDir = path.join(LOCALES_SOURCE_DIR, locale);
    return fs.existsSync(localeDir) && fs.statSync(localeDir).isDirectory();
  });

  if (locales.length === 0) {
    console.error(`❌ No locale directories found in ${LOCALES_SOURCE_DIR}`);
    process.exit(1);
  }

  let totalFiles = 0;

  // Process each locale
  for (const locale of locales) {
    const localeSourceDir = path.join(LOCALES_SOURCE_DIR, locale);
    const localeOutputDir = path.join(LOCALES_OUTPUT_DIR, locale);

    console.log(`📦 Processing locale: ${locale}`);

    // Create locale output directory
    if (!fs.existsSync(localeOutputDir)) {
      fs.mkdirSync(localeOutputDir, { recursive: true });
    }

    // Find all .ftl files for this locale
    const ftlFiles = findFtlFiles(localeSourceDir);

    if (ftlFiles.length === 0) {
      console.warn(`⚠️  No .ftl files found in ${localeSourceDir}`);
      continue;
    }

    // Process each .ftl file as a separate namespace
    for (const { path: ftlPath, namespace } of ftlFiles) {
      try {
        const content = fs.readFileSync(ftlPath, "utf-8");
        const messages = extractMessagesFromFtl(content);

        if (Object.keys(messages).length === 0) {
          console.warn(`⚠️  No messages found in ${ftlPath}`);
          continue;
        }

        // Write namespace JSON file
        const outputPath = path.join(localeOutputDir, `${namespace}.json`);
        fs.writeFileSync(
          outputPath,
          JSON.stringify(messages, null, 2),
          "utf-8",
        );

        console.log(
          `  ✅ ${namespace}: ${Object.keys(messages).length} keys → ${path.relative(process.cwd(), outputPath)}`,
        );
        totalFiles++;
      } catch (error) {
        console.error(`❌ Error processing ${ftlPath}:`, error.message);
      }
    }
  }

  console.log(
    `🎉 Compilation complete! ${totalFiles} namespace files generated.`,
  );
}

/**
 * Copy compiled translations to app/public/locales
 */
function copyToApp() {
  const appPublicLocales = path.join(
    __dirname,
    "..",
    "..",
    "app",
    "public",
    "locales",
  );

  console.log("\n📋 Copying translations to app...");

  // Remove old locales directory in app
  if (fs.existsSync(appPublicLocales)) {
    fs.rmSync(appPublicLocales, { recursive: true, force: true });
  }

  // Copy compiled locales to app
  fs.cpSync(LOCALES_OUTPUT_DIR, appPublicLocales, { recursive: true });

  console.log(`✅ Copied to ${path.relative(process.cwd(), appPublicLocales)}`);
}

// Run the compilation
compileTranslations();

// Copy to app if it exists
const appPath = path.join(__dirname, "..", "..", "app");
if (fs.existsSync(appPath)) {
  copyToApp();
}
