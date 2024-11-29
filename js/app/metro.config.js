// Learn more https://docs.expo.io/guides/customizing-metro
/**
 * @type {import('expo/metro-config').MetroConfig}
 */
const { getDefaultConfig } = require("expo/metro-config");

let config = getDefaultConfig(__dirname, {
  // [Web-only]: Enables CSS support in Metro.
  isCSSEnabled: true,
});

// Enable Tamagui and add nice web support with optimizing compiler + CSS extraction
const { withTamagui } = require("@tamagui/metro-plugin");
config = withTamagui(config, {
  components: ["tamagui"],
  config: "./tamagui.config.ts",
  outputCSS: "./tamagui-web.css",
});

const nativeOverrides = {
  crypto: "react-native-quick-crypto",
  stream: "readable-stream",
};

config.resolver.resolveRequest = (context, moduleName, platform) => {
  if (platform !== "web") {
    for (const [key, value] of Object.entries(nativeOverrides)) {
      if (moduleName === key) {
        return context.resolveRequest(context, value, platform);
      }
    }
  }
  // otherwise chain to the standard Metro resolver.
  return context.resolveRequest(context, moduleName, platform);
};

config.resolver.sourceExts.push("mjs");
config.resolver.assetExts.push("md");

module.exports = config;

// REMOVE THIS (just for tamagui internal devs to work in monorepo):
// if (process.env.IS_TAMAGUI_DEV && __dirname.includes('tamagui')) {
//   const fs = require('fs')
//   const path = require('path')
//   const projectRoot = __dirname
//   const monorepoRoot = path.resolve(projectRoot, '../..')
//   config.watchFolders = [monorepoRoot]
//   config.resolver.nodeModulesPaths = [
//     path.resolve(projectRoot, 'node_modules'),
//     path.resolve(monorepoRoot, 'node_modules'),
//   ]
//   // have to manually de-deupe
//   try {
//     fs.rmSync(path.join(projectRoot, 'node_modules', '@tamagui'), {
//       recursive: true,
//       force: true,
//     })
//   } catch {}
//   try {
//     fs.rmSync(path.join(projectRoot, 'node_modules', 'tamagui'), {
//       recursive: true,
//       force: true,
//     })
//   } catch {}
//   try {
//     fs.rmSync(path.join(projectRoot, 'node_modules', 'react'), {
//       recursive: true,
//       force: true,
//     })
//   } catch {}
//   try {
//     fs.rmSync(path.join(projectRoot, 'node_modules', 'react-dom'), {
//       recursive: true,
//       force: true,
//     })
//   } catch {}
// }
