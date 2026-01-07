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
  crypto: "react-native-quick-crypto",
  // "node:crypto": "react-native-quick-crypto",
  stream: "readable-stream",
  // "node:buffer": "buffer",
  // "node:util": "util",
  // "node:http": path.resolve(__dirname, "./empty.mjs"),
  // "node:https": path.resolve(__dirname, "./empty.mjs"),
  // // "node:events": "events",
};

config.resolver.resolveRequest = (context, moduleName, platform) => {
  if (moduleName.includes("zustand")) {
    const result = require.resolve(moduleName);
    return context.resolveRequest(context, result, platform);
  }
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
  return context.resolveRequest(context, moduleName, platform);
};

config.resolver.sourceExts.push("mjs");
config.resolver.assetExts.push("md");

config.resolver.unstable_conditionNames.push("@streamplace/dev", "browser");

module.exports = config;
