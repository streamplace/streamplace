#!/usr/bin/env node

/**
 * i18n migration script
 * Migrates extracted JSON keys to .ftl files for translation
 *
 * This script expects that i18next-cli has already extracted keys to JSON files.
 * It reads those JSON files, compares them to existing .ftl files, and adds any
 * new keys to the .ftl files.
 *
 * For keys with i18next context/plural suffixes (e.g., key_male, key_female, key_one, key_other),
 * it will convert them into Fluent select expressions.
 *
 * Usage:
 *   node migrate-i18n.js                    # Report new keys
 *   node migrate-i18n.js --add-to=common    # Add new keys to common.ftl
 *   node migrate-i18n.js --add-to=settings  # Add new keys to settings.ftl
 */

const fs = require("fs");
const path = require("path");

// Parse command line arguments
const args = process.argv.slice(2);
const addToNamespace = args
  .find((arg) => arg.startsWith("--add-to="))
  ?.split("=")[1];

// Paths
const COMPONENTS_ROOT = path.join(__dirname, "..");
const MANIFEST_PATH = path.join(COMPONENTS_ROOT, "locales/manifest.json");
const LOCALES_FTL_DIR = path.join(COMPONENTS_ROOT, "locales");
const LOCALES_JSON_DIR = path.join(COMPONENTS_ROOT, "public/locales");

// Load manifest
const manifest = JSON.parse(fs.readFileSync(MANIFEST_PATH, "utf8"));

// Plural forms that i18next uses
const PLURAL_FORMS = ["zero", "one", "two", "few", "many", "other"];

// Separators used by i18next-cli (configured in i18next.config.js)
const CONTEXT_SEPARATOR = "|";
const PLURAL_SEPARATOR = "/";

/**
 * Group keys by base name, detecting context and plural variants
 * Returns { baseKey: { base: true, variants: { context: Set, plurals: Set } } }
 */
function groupKeysByBase(keys) {
  const groups = {};

  for (const key of keys) {
    if (!key.includes(CONTEXT_SEPARATOR) && !key.includes(PLURAL_SEPARATOR)) {
      // Simple key with no variants
      if (!groups[key]) {
        groups[key] = {
          base: true,
          variants: { contexts: new Set(), plurals: new Set() },
        };
      }
      groups[key].base = true;
    } else {
      // Key with variants
      // Format: base|context/plural or base/plural or base|context
      let baseKey = key;
      const detectedContexts = new Set();
      const detectedPlurals = new Set();

      // Split by context separator first
      if (key.includes(CONTEXT_SEPARATOR)) {
        const contextParts = key.split(CONTEXT_SEPARATOR);
        baseKey = contextParts[0];

        // The remaining part might have plurals
        const contextAndPlural = contextParts[1];

        if (contextAndPlural.includes(PLURAL_SEPARATOR)) {
          const pluralParts = contextAndPlural.split(PLURAL_SEPARATOR);
          detectedContexts.add(pluralParts[0]);
          pluralParts.slice(1).forEach((p) => {
            if (PLURAL_FORMS.includes(p)) {
              detectedPlurals.add(p);
            }
          });
        } else {
          detectedContexts.add(contextAndPlural);
        }
      } else if (key.includes(PLURAL_SEPARATOR)) {
        // No context, just plural
        const pluralParts = key.split(PLURAL_SEPARATOR);
        baseKey = pluralParts[0];
        pluralParts.slice(1).forEach((p) => {
          if (PLURAL_FORMS.includes(p)) {
            detectedPlurals.add(p);
          }
        });
      }

      if (!groups[baseKey]) {
        groups[baseKey] = {
          base: false,
          variants: { contexts: new Set(), plurals: new Set() },
        };
      }

      detectedContexts.forEach((c) => groups[baseKey].variants.contexts.add(c));
      detectedPlurals.forEach((p) => groups[baseKey].variants.plurals.add(p));
    }
  }

  return groups;
}

/**
 * Convert a group of keys into Fluent format
 */
function convertToFluentFormat(baseKey, group) {
  const hasContexts = group.variants.contexts.size > 0;
  const hasPlurals = group.variants.plurals.size > 0;

  if (!hasContexts && !hasPlurals) {
    // Simple key
    return `${baseKey} = ${baseKey}`;
  }

  // Build Fluent select expression
  let selector = "";
  let variants = [];

  if (hasContexts && hasPlurals) {
    // Both context and plural - outer selector is context, inner is plural
    selector = "$context";
    const contextsList = Array.from(group.variants.contexts).sort();
    const pluralsList = Array.from(group.variants.plurals).sort();

    contextsList.forEach((context, idx) => {
      const isDefault = idx === contextsList.length - 1;
      const prefix = isDefault ? "*" : " ";

      // Build inner plural select
      const pluralVariants = pluralsList
        .map((p) => {
          const pluralPrefix = p === "other" ? "*" : "";
          return `${pluralPrefix}[${p}] ${baseKey}`;
        })
        .join(" ");

      variants.push(
        `\n   ${prefix}[${context}] { $count -> ${pluralVariants} }`,
      );
    });
  } else if (hasContexts) {
    // Only context
    selector = "$context";
    const contextsList = Array.from(group.variants.contexts).sort();
    contextsList.forEach((context, idx) => {
      const isDefault = idx === contextsList.length - 1;
      const prefix = isDefault ? "*" : " ";
      variants.push(`\n   ${prefix}[${context}] ${baseKey}`);
    });
  } else if (hasPlurals) {
    // Only plural
    selector = "$count";
    const pluralsList = Array.from(group.variants.plurals).sort();
    pluralsList.forEach((plural) => {
      const isDefault = plural === "other";
      const prefix = isDefault ? "*" : " ";
      variants.push(`\n   ${prefix}[${plural}] ${baseKey}`);
    });
  }

  return `# TODO: Convert to proper Fluent select expression\n${baseKey} = { ${selector} ->${variants.join("")}\n}`;
}

/**
 * Read existing keys from .ftl files in a locale directory
 * Returns a map of namespace -> Set of keys
 */
function getExistingFtlKeys(localeDir) {
  const keysByNamespace = {};

  if (!fs.existsSync(localeDir)) {
    return keysByNamespace;
  }

  const ftlFiles = fs
    .readdirSync(localeDir)
    .filter((file) => file.endsWith(".ftl"));

  for (const file of ftlFiles) {
    const namespace = path.basename(file, ".ftl");
    const keys = new Set();

    const content = fs.readFileSync(path.join(localeDir, file), "utf8");
    const lines = content.split("\n");

    for (const line of lines) {
      const trimmed = line.trim();
      const keyMatch = trimmed.match(/^([a-zA-Z][a-zA-Z0-9_-]*)\s*=/);
      if (keyMatch) {
        keys.add(keyMatch[1]);
      }
    }

    keysByNamespace[namespace] = keys;
  }

  return keysByNamespace;
}

/**
 * Get all namespaces (json files) in the locale directory
 */
function getNamespaces(localeJsonDir) {
  if (!fs.existsSync(localeJsonDir)) {
    return [];
  }

  return fs
    .readdirSync(localeJsonDir)
    .filter((file) => file.endsWith(".json"))
    .map((file) => path.basename(file, ".json"));
}

/**
 * Add new keys to a .ftl file, converting context/plural keys to Fluent format
 */
function addKeysToFtlFile(localeDir, namespace, newKeys, locale) {
  const targetFile = path.join(localeDir, `${namespace}.ftl`);

  // Create file with header if it doesn't exist
  if (!fs.existsSync(localeDir)) {
    fs.mkdirSync(localeDir, { recursive: true });
  }

  if (!fs.existsSync(targetFile)) {
    const languageName = manifest.languages[locale]?.name || locale;
    const namespaceName =
      namespace.charAt(0).toUpperCase() + namespace.slice(1);
    const header = `# ${namespaceName} translations - ${languageName}\n\n`;
    fs.writeFileSync(targetFile, header);
  }

  // Group keys by base to detect context/plural variants
  const keyGroups = groupKeysByBase(newKeys);

  // Build content
  const fluentEntries = [];
  for (const [baseKey, group] of Object.entries(keyGroups)) {
    fluentEntries.push(convertToFluentFormat(baseKey, group));
  }

  // Append new keys
  let content = fs.readFileSync(targetFile, "utf8");

  if (!content.endsWith("\n")) {
    content += "\n";
  }

  content += "\n# Newly extracted keys\n";
  content += fluentEntries.join("\n\n") + "\n";

  fs.writeFileSync(targetFile, content);

  return targetFile;
}

/**
 * Migrate extracted JSON keys to .ftl files
 */
function migrateKeysToFtl() {
  console.log("🔄 Analyzing extracted keys...");

  const newKeysByLocaleAndNamespace = {}; // locale -> namespace -> [keys]

  // Process each locale
  for (const locale of manifest.supportedLocales) {
    const localeJsonDir = path.join(LOCALES_JSON_DIR, locale);
    const localeFtlDir = path.join(LOCALES_FTL_DIR, locale);

    if (!fs.existsSync(localeJsonDir)) {
      console.log(`⚠️  No JSON files found for ${locale}`);
      continue;
    }

    // Get all namespaces (json files)
    const namespaces = getNamespaces(localeJsonDir);

    if (namespaces.length === 0) {
      console.log(`⚠️  No namespace files found for ${locale}`);
      continue;
    }

    // Get existing keys from .ftl files
    const existingKeysByNamespace = getExistingFtlKeys(localeFtlDir);

    // Process each namespace
    for (const namespace of namespaces) {
      const jsonPath = path.join(localeJsonDir, `${namespace}.json`);
      const jsonContent = JSON.parse(fs.readFileSync(jsonPath, "utf8"));
      const extractedKeys = Object.keys(jsonContent);

      // Get existing keys for this namespace
      const existingKeys = existingKeysByNamespace[namespace] || new Set();

      // Find new keys
      const newKeys = extractedKeys.filter((key) => !existingKeys.has(key));

      if (newKeys.length > 0) {
        if (!newKeysByLocaleAndNamespace[locale]) {
          newKeysByLocaleAndNamespace[locale] = {};
        }
        newKeysByLocaleAndNamespace[locale][namespace] = newKeys;
      }
    }
  }

  // Check if there are any new keys
  const hasNewKeys = Object.keys(newKeysByLocaleAndNamespace).length > 0;

  if (!hasNewKeys) {
    console.log(
      "\n🎉 No new keys found. All extracted keys already exist in .ftl files.",
    );
    return;
  }

  // Display found keys
  console.log("\n📊 New keys found:");
  for (const locale of Object.keys(newKeysByLocaleAndNamespace)) {
    console.log(`\n${locale}:`);
    for (const namespace of Object.keys(newKeysByLocaleAndNamespace[locale])) {
      const keys = newKeysByLocaleAndNamespace[locale][namespace];
      console.log(`  📝 ${namespace} (${keys.length} new keys):`);
      keys.forEach((key) => console.log(`     - ${key}`));
    }
  }

  // If --add-to flag is provided, add keys to that namespace
  if (addToNamespace) {
    console.log(`\n✍️  Adding new keys to ${addToNamespace}.ftl files...`);

    let totalAdded = 0;
    const processedFiles = [];

    for (const locale of Object.keys(newKeysByLocaleAndNamespace)) {
      const localeFtlDir = path.join(LOCALES_FTL_DIR, locale);
      const namespacesForLocale = newKeysByLocaleAndNamespace[locale];

      // Collect all new keys across all namespaces for this locale
      const allNewKeys = [];
      for (const namespace of Object.keys(namespacesForLocale)) {
        allNewKeys.push(...namespacesForLocale[namespace]);
      }

      if (allNewKeys.length === 0) continue;

      // Add all keys to the specified namespace
      const targetFile = addKeysToFtlFile(
        localeFtlDir,
        addToNamespace,
        allNewKeys,
        locale,
      );
      processedFiles.push(path.relative(process.cwd(), targetFile));
      totalAdded += allNewKeys.length;

      console.log(
        `✅ ${locale}: Added ${allNewKeys.length} keys to ${addToNamespace}.ftl`,
      );
    }

    console.log(
      `\n🎉 Migration complete! Added ${totalAdded} new keys to ${addToNamespace}.ftl files.`,
    );
    console.log("\nModified files:");
    processedFiles.forEach((file) => console.log(`   📄 ${file}`));

    console.log("\n💡 Next steps:");
    console.log("   1. Review the new keys in your .ftl files");
    console.log(
      "   2. Convert TODO placeholders to proper Fluent translations",
    );
    console.log("   3. Run `pnpm i18n:compile` to update compiled JSON files");
  } else {
    // Just report
    let totalNewKeys = 0;
    const namespaceSet = new Set();

    for (const locale of Object.keys(newKeysByLocaleAndNamespace)) {
      for (const namespace of Object.keys(
        newKeysByLocaleAndNamespace[locale],
      )) {
        namespaceSet.add(namespace);
        totalNewKeys += newKeysByLocaleAndNamespace[locale][namespace].length;
      }
    }

    console.log(
      `\n💡 Found ${totalNewKeys} new keys across ${namespaceSet.size} namespace(s).`,
    );
    console.log("\nTo add these keys to a specific namespace file, run:");
    Array.from(namespaceSet).forEach((ns) => {
      console.log(`   node migrate-i18n.js --add-to=${ns}`);
    });
  }
}

function main() {
  migrateKeysToFtl();
}

main();
