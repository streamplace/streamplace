const { FileStore } = require("metro-cache");
const path = require("path");

const { getSentryExpoConfig } = require("@sentry/react-native/metro");

let config = getSentryExpoConfig(__dirname, {
  // [Web-only]: Enables CSS support in Metro.
  isCSSEnabled: true,
});

config.cacheStores = [
  new FileStore({
    root: path.join(__dirname, "node_modules", ".cache", "metro"),
  }),
];

const overrides = {};

const nativeOverrides = {
  stream: "readable-stream",
  // "node:buffer": "buffer",
  // "node:util": "util",
  // "node:http": path.resolve(__dirname, "./empty.mjs"),
  // "node:https": path.resolve(__dirname, "./empty.mjs"),
  // // "node:events": "events",
};

config.resolver.resolveRequest = (context, moduleName, platform) => {
  if (moduleName === "@react-navigation/elements/internal") {
    return context.resolveRequest(
      context,
      "@react-navigation/elements/lib/module/internal",
      platform,
    );
  }
  // if (moduleName.includes("zustand")) {
  //   const result = require.resolve(moduleName);
  //   return context.resolveRequest(context, result, platform);
  // }
  if (platform !== "web") {
    for (const [key, value] of Object.entries(nativeOverrides)) {
      if (moduleName === key) {
        return context.resolveRequest(context, value, platform);
      }
    }
  }
  for (const [key, value] of Object.entries(overrides)) {
    if (moduleName === key) {
      return context.resolveRequest(context, value, platform);
    }
  }
  // otherwise chain to the standard Metro resolver.
  try {
    return context.resolveRequest(context, moduleName, platform);
  } catch (error) {
    // The `streamplace` workspace package is ESM and (per Node's ESM rules)
    // uses explicit ".js" extensions on its relative imports even though the
    // sources are ".ts"/".tsx" (e.g. the generated lexicon files). Metro does
    // not perform that ".js" -> ".ts" remap, so retry extension-less.
    if (moduleName.startsWith(".") && moduleName.endsWith(".js")) {
      return context.resolveRequest(context, moduleName.slice(0, -3), platform);
    }
    throw error;
  }
};

config.resolver.sourceExts.push("mjs");
config.resolver.assetExts.push("md");

config.resolver.unstable_conditionNames.push("@streamplace/dev", "browser");

// Ensure workspace packages get transformed by babel
config.watchFolders = [path.resolve(__dirname, "../..")];
config.transformer = {
  ...config.transformer,
  getTransformOptions: async () => ({
    transform: {
      experimentalImportSupport: true,
      inlineRequires: true,
    },
  }),
};

// Transform @streamplace/components workspace package
const { getDefaultConfig } = require("expo/metro-config");
const defaultConfig = getDefaultConfig(__dirname);
config.resolver.nodeModulesPaths = [
  ...defaultConfig.resolver.nodeModulesPaths,
  path.resolve(__dirname, "../components"),
];

module.exports = config;
