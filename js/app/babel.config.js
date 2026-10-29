const lucideTransformPlugin = require("./babel-plugin-lucide-transform");
const inlineImportsPlugin = require("./babel-plugin-inline-imports");

module.exports = (api) => {
  api.cache(true);
  return {
    presets: [["babel-preset-expo", { jsxRuntime: "automatic" }]],
    plugins: [
      lucideTransformPlugin,
      // NOTE: this is only necessary if you are using reanimated for animations
      "react-native-reanimated/plugin",
    ],
    overrides: [
      {
        // Sign in with Ethereum's libraries; see babel-plugin-inline-imports.js
        test: /node_modules[\\/](viem|ox)[\\/]/,
        plugins: [inlineImportsPlugin],
      },
    ],
  };
};
